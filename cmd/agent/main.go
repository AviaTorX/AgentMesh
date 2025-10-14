package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/avinashshinde/agentmesh-cortex/internal/config"
	"github.com/avinashshinde/agentmesh-cortex/internal/messaging"
	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
)

// Standalone agent that runs as a separate process
// Communicates only via Kafka and Redis (no shared memory)

func main() {
	// Parse command-line flags
	agentName := flag.String("name", "", "Agent name (required)")
	agentRole := flag.String("role", "", "Agent role (required)")
	capabilities := flag.String("capabilities", "", "Comma-separated capabilities")
	metadata := flag.String("metadata", "", "Comma-separated key:value pairs (e.g., framework:openai,model:gpt-4)")
	flag.Parse()

	if *agentName == "" || *agentRole == "" {
		fmt.Println("Usage: agent -name=<name> -role=<role> -capabilities=<cap1,cap2> -metadata=<key:value,key:value>")
		os.Exit(1)
	}

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting AgentMesh Cortex Agent",
		zap.String("name", *agentName),
		zap.String("role", *agentRole),
	)

	// Load configuration
	cfg := config.Load()

	// Create agent instance
	agent := &types.Agent{
		ID:           types.NewAgentID(),
		Name:         *agentName,
		Role:         *agentRole,
		Status:       types.AgentStatusActive,
		Capabilities: parseCapabilities(*capabilities),
		Metadata:     parseMetadata(*metadata),
		CreatedAt:    time.Now(),
		LastSeenAt:   time.Now(),
	}

	// Initialize Kafka messaging
	messaging := messaging.NewKafkaMessaging(cfg, logger)
	defer messaging.Close()

	// Create distributed agent runtime
	runtime := NewDistributedAgent(agent, messaging, cfg, logger)

	// Start agent
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := runtime.Start(ctx); err != nil {
		logger.Fatal("Failed to start agent", zap.Error(err))
	}
	defer runtime.Stop()

	logger.Info("Agent running",
		zap.String("agent_id", string(agent.ID)),
		zap.String("name", agent.Name),
		zap.String("role", agent.Role),
	)

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	logger.Info("Agent shutting down gracefully...")
}

func parseCapabilities(capStr string) []string {
	if capStr == "" {
		return []string{}
	}
	return strings.Split(capStr, ",")
}

func parseMetadata(metaStr string) map[string]string {
	metadata := make(map[string]string)
	if metaStr == "" {
		return metadata
	}

	// Parse comma-separated key:value pairs
	// Example: "framework:openai,model:gpt-4"
	pairs := strings.Split(metaStr, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) == 2 {
			metadata[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return metadata
}

// DistributedAgent is an agent that communicates only via Kafka/Redis (no shared memory)
type DistributedAgent struct {
	agent     *types.Agent
	messaging *messaging.KafkaMessaging
	config    *types.Config
	logger    *zap.Logger
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewDistributedAgent(
	agent *types.Agent,
	msg *messaging.KafkaMessaging,
	cfg *types.Config,
	logger *zap.Logger,
) *DistributedAgent {
	ctx, cancel := context.WithCancel(context.Background())
	return &DistributedAgent{
		agent:     agent,
		messaging: msg,
		config:    cfg,
		logger:    logger.With(zap.String("agent_id", string(agent.ID))),
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (da *DistributedAgent) Start(ctx context.Context) error {
	da.logger.Info("Agent joining mesh")

	// Publish agent joined event to Kafka
	joinEvent := types.TopologyEvent{
		Type:      types.TopologyEventAgentJoined,
		AgentID:   da.agent.ID,
		Agent:     da.agent,
		Timestamp: time.Now(),
	}
	if err := da.messaging.PublishTopologyEvent(ctx, joinEvent); err != nil {
		return fmt.Errorf("failed to publish join event: %w", err)
	}

	// Start message consumer
	go da.consumeMessages()

	// Start heartbeat sender
	go da.sendHeartbeats()

	// Start business logic simulator
	go da.simulateBusinessLogic()

	return nil
}

func (da *DistributedAgent) Stop() error {
	da.logger.Info("Agent leaving mesh")

	// Publish agent left event
	leaveEvent := types.TopologyEvent{
		Type:      types.TopologyEventAgentLeft,
		AgentID:   da.agent.ID,
		Timestamp: time.Now(),
	}
	da.messaging.PublishTopologyEvent(da.ctx, leaveEvent)

	da.cancel()
	return nil
}

func (da *DistributedAgent) SendMessage(toAgentID types.AgentID, msgType types.MessageType, payload map[string]any) error {
	message := &types.Message{
		ID:          fmt.Sprintf("%s-%d", da.agent.ID, time.Now().UnixNano()),
		FromAgentID: da.agent.ID,
		ToAgentID:   toAgentID,
		Type:        msgType,
		Payload:     payload,
		Metadata:    map[string]string{"agent_role": da.agent.Role},
		Timestamp:   time.Now(),
		EdgeID:      types.NewEdgeID(da.agent.ID, toAgentID),
	}

	// Publish to Kafka - topology manager will handle reinforcement
	if err := da.messaging.PublishMessage(da.ctx, "messages", message); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	da.logger.Debug("Sent message",
		zap.String("to", string(toAgentID)),
		zap.String("type", string(msgType)),
	)

	return nil
}

func (da *DistributedAgent) consumeMessages() {
	groupID := fmt.Sprintf("agent-%s", da.agent.ID)
	err := da.messaging.ConsumeMessages(da.ctx, "messages", groupID, func(msg *types.Message) error {
		// Only process messages addressed to this agent
		if msg.ToAgentID != da.agent.ID {
			return nil
		}

		da.logger.Info("Received message",
			zap.String("from", string(msg.FromAgentID)),
			zap.String("type", string(msg.Type)),
		)

		// Process message and learn insights
		da.processMessageAndLearn(msg)

		return nil
	})

	if err != nil && err != context.Canceled {
		da.logger.Error("Message consumption stopped", zap.Error(err))
	}
}

// processMessageAndLearn handles a message and extracts insights
func (da *DistributedAgent) processMessageAndLearn(msg *types.Message) {
	// Simple rule-based insight generation
	// In production, this would use LLM to analyze and learn

	var insight *types.Insight

	// Example: Sales agent learns from pricing-related messages
	if da.agent.Role == "sales" {
		if action, ok := msg.Payload["action"].(string); ok {
			if action == "check_price" || action == "negotiate_price" {
				insight = types.NewInsight(
					da.agent.ID,
					da.agent.Role,
					types.InsightTypePricingIssue,
					"pricing",
					fmt.Sprintf("Customer interested in pricing for %v", msg.Payload["product"]),
					0.7,
				)
			}
		}
	}

	// Example: Support agent learns from customer complaints
	if da.agent.Role == "support" {
		if msgType := string(msg.Type); msgType == "task" {
			if action, ok := msg.Payload["action"].(string); ok {
				if action == "report_issue" {
					insight = types.NewInsight(
						da.agent.ID,
						da.agent.Role,
						types.InsightTypeProductIssue,
						"product_quality",
						fmt.Sprintf("Customer reported issue: %v", msg.Payload["issue"]),
						0.85,
					)
				}
			}
		}
	}

	// Example: Fraud agent learns from verification requests
	if da.agent.Role == "fraud" {
		if action, ok := msg.Payload["action"].(string); ok {
			if action == "verify_user" || action == "check_transaction" {
				insight = types.NewInsight(
					da.agent.ID,
					da.agent.Role,
					types.InsightTypeFraudPattern,
					"fraud_detection",
					fmt.Sprintf("Verification requested for %v", msg.Payload["user_id"]),
					0.6,
				)
			}
		}
	}

	// Example: Inventory agent learns from stock patterns
	if da.agent.Role == "inventory" {
		if action, ok := msg.Payload["action"].(string); ok {
			if action == "check_stock" {
				// Track stock check frequency as inventory trend
				insight = types.NewInsight(
					da.agent.ID,
					da.agent.Role,
					types.InsightTypeInventoryTrend,
					"inventory",
					fmt.Sprintf("Stock check for SKU: %v", msg.Payload["sku"]),
					0.5,
				)
			}
		}
	}

	// Publish insight to knowledge mesh
	if insight != nil {
		if err := da.messaging.PublishInsight(da.ctx, insight); err != nil {
			da.logger.Error("Failed to publish insight", zap.Error(err))
		} else {
			da.logger.Info("Published insight",
				zap.String("insight_id", string(insight.ID)),
				zap.String("type", string(insight.Type)),
				zap.String("topic", insight.Topic),
			)
		}
	}
}

func (da *DistributedAgent) sendHeartbeats() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-da.ctx.Done():
			return
		case <-ticker.C:
			da.agent.LastSeenAt = time.Now()
			da.logger.Debug("Heartbeat")
		}
	}
}

// simulateBusinessLogic simulates agent behavior by sending messages to other agents
func (da *DistributedAgent) simulateBusinessLogic() {
	// Send initial message immediately to create edge
	da.sendInitialMessage()

	// Then wait for periodic messaging
	time.Sleep(5 * time.Second)

	// Define agent interactions based on role
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	counter := 0
	for {
		select {
		case <-da.ctx.Done():
			return
		case <-ticker.C:
			counter++

			// Sales agent creates orders and checks inventory
			if da.agent.Role == "sales" {
				if counter%2 == 0 {
					// Send to Inventory agent (role-based, will find any inventory agent)
					productName := fmt.Sprintf("Product-%d", counter)
					da.sendToRole("inventory", types.MessageTypeTask, map[string]any{
						"action":      "check_stock",
						"product":     productName,
						"qty":         counter % 10,
						"description": fmt.Sprintf("Check stock availability for %s (qty: %d)", productName, counter%10),
					})
				}
				if counter%3 == 0 {
					// Send to Fraud agent
					orderID := fmt.Sprintf("ORD-%d", counter)
					amount := float64(counter * 100)
					da.sendToRole("fraud", types.MessageTypeTask, map[string]any{
						"action":      "verify_transaction",
						"order_id":    orderID,
						"amount":      amount,
						"description": fmt.Sprintf("Verify transaction %s ($%.2f)", orderID, amount),
					})
				}
			}

			// Support agent creates tickets and escalates to multiple teams
			if da.agent.Role == "support" {
				targets := []string{"sales", "inventory", "fraud"}
				targetRole := targets[counter%len(targets)]

				if counter%2 == 0 {
					ticketID := fmt.Sprintf("TKT-%d", counter)
					action := "escalate"
					issueType := "pricing_complaint"

					if targetRole == "inventory" {
						action = "check_delivery"
						issueType = "shipping_delay"
					} else if targetRole == "fraud" {
						action = "verify_account"
						issueType = "suspicious_activity"
					}

					da.sendToRole(targetRole, types.MessageTypeTask, map[string]any{
						"action":      action,
						"ticket_id":   ticketID,
						"issue_type":  issueType,
						"description": fmt.Sprintf("Support %s for ticket %s - %s", action, ticketID, issueType),
					})
				}
			}

			// Inventory agent notifies Sales and Support
			if da.agent.Role == "inventory" {
				targets := []string{"sales", "support"}
				targetRole := targets[counter%len(targets)]

				if counter%2 == 0 {
					productName := fmt.Sprintf("Product-%d", counter)
					action := "stock_alert"
					level := "low"

					if targetRole == "support" {
						action = "delivery_update"
						level = "delayed"
					}

					da.sendToRole(targetRole, types.MessageTypeTask, map[string]any{
						"action":      action,
						"product":     productName,
						"level":       level,
						"description": fmt.Sprintf("%s for %s - status: %s", action, productName, level),
					})
				}
			}

			// Fraud agent reports to Sales and Support
			if da.agent.Role == "fraud" {
				targets := []string{"sales", "support"}
				targetRole := targets[counter%len(targets)]

				if counter%3 == 0 {
					txnID := fmt.Sprintf("TXN-%d", counter)
					action := "fraud_alert"
					riskLevel := "medium"

					if targetRole == "support" {
						action = "account_suspension"
						riskLevel = "high"
					}

					da.sendToRole(targetRole, types.MessageTypeTask, map[string]any{
						"action":      action,
						"transaction": txnID,
						"risk_level":  riskLevel,
						"description": fmt.Sprintf("%s for transaction %s - risk: %s", action, txnID, riskLevel),
					})
				}
			}

			// Research agent (OpenAI) sends research requests and findings
			if da.agent.Role == "research" {
				targets := []string{"sales", "support", "inventory"}
				targetRole := targets[counter%len(targets)]

				if counter%2 == 0 {
					da.sendToRole(targetRole, types.MessageTypeTask, map[string]any{
						"action":      "research_request",
						"topic":       fmt.Sprintf("market_trend_%d", counter),
						"priority":    "high",
						"description": fmt.Sprintf("OpenAI Research: Requesting %s data for market analysis #%d", targetRole, counter),
					})
				}
			}

			// Market Analyst (LangChain) sends analysis reports and forecasts
			if da.agent.Role == "analyst" {
				targets := []string{"sales", "inventory", "fraud"}
				targetRole := targets[counter%len(targets)]

				if counter%2 == 0 {
					da.sendToRole(targetRole, types.MessageTypeTask, map[string]any{
						"action":      "analysis_report",
						"metric":      fmt.Sprintf("kpi_%d", counter),
						"trend":       "increasing",
						"description": fmt.Sprintf("LangChain Analyst: Market analysis report #%d for %s", counter, targetRole),
					})
				}
			}

			// Coordinator (Anthropic) sends coordination updates and health checks
			if da.agent.Role == "coordinator" {
				targets := []string{"sales", "support", "inventory", "fraud", "research", "analyst"}
				targetRole := targets[counter%len(targets)]

				if counter%2 == 0 {
					da.sendToRole(targetRole, types.MessageTypeTask, map[string]any{
						"action":      "coordination_update",
						"status":      "all_systems_operational",
						"check_id":    fmt.Sprintf("health_check_%d", counter),
						"description": fmt.Sprintf("Anthropic Coordinator: System health check #%d - %s status OK", counter, targetRole),
					})
				}
			}
		}
	}
}

// sendInitialMessage sends an initial self-message to create the edge immediately
func (da *DistributedAgent) sendInitialMessage() {
	message := &types.Message{
		ID:          fmt.Sprintf("%s-%d", da.agent.ID, time.Now().UnixNano()),
		FromAgentID: da.agent.ID,
		ToAgentID:   da.agent.ID,
		Type:        types.MessageTypeTask,
		Payload: map[string]any{
			"action":      "init",
			"message":     "Initial edge creation",
			"description": fmt.Sprintf("%s agent initializing and joining mesh", da.agent.Name),
		},
		Timestamp: time.Now(),
	}

	if err := da.messaging.PublishMessage(da.ctx, "messages", message); err != nil {
		da.logger.Error("Failed to send initial message", zap.Error(err))
	} else {
		da.logger.Debug("Sent initial self-message to create edge",
			zap.String("agent_id", string(da.agent.ID)),
		)
	}
}

// sendToRole sends a message to any agent with the given role (helper method)
func (da *DistributedAgent) sendToRole(role string, msgType types.MessageType, payload map[string]any) {
	// Query topology API to find an agent with the target role
	targetID := da.findAgentByRole(role)
	if targetID == "" {
		// If no agent found with role, pick a random other agent
		targetID = da.findRandomAgent()
		if targetID == "" {
			da.logger.Debug("No other agents available", zap.String("role", role))
			return
		}
	}

	message := &types.Message{
		ID:          fmt.Sprintf("%s-%d", da.agent.ID, time.Now().UnixNano()),
		FromAgentID: da.agent.ID,
		ToAgentID:   targetID,
		Type:        msgType,
		Payload:     payload,
		Timestamp:   time.Now(),
	}

	if err := da.messaging.PublishMessage(da.ctx, "messages", message); err != nil {
		da.logger.Error("Failed to send message", zap.Error(err))
	} else {
		da.logger.Debug("Sent message",
			zap.String("to_role", role),
			zap.String("target", string(targetID)),
			zap.String("type", string(msgType)),
		)
	}
}

// findAgentByRole queries the topology API to find an agent with the given role
func (da *DistributedAgent) findAgentByRole(role string) types.AgentID {
	resp, err := http.Get("http://localhost:8080/api/topology")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var topologyData struct {
		Agents map[types.AgentID]*types.Agent `json:"agents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&topologyData); err != nil {
		return ""
	}

	// Find first agent with matching role (excluding self)
	for id, agent := range topologyData.Agents {
		if agent.Role == role && id != da.agent.ID {
			return id
		}
	}
	return ""
}

// findRandomAgent returns a random agent ID from the topology (excluding self)
func (da *DistributedAgent) findRandomAgent() types.AgentID {
	resp, err := http.Get("http://localhost:8080/api/topology")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var topologyData struct {
		Agents map[types.AgentID]*types.Agent `json:"agents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&topologyData); err != nil {
		return ""
	}

	// Collect all agents except self
	var otherAgents []types.AgentID
	for id := range topologyData.Agents {
		if id != da.agent.ID {
			otherAgents = append(otherAgents, id)
		}
	}

	if len(otherAgents) == 0 {
		return ""
	}

	// Return random agent
	return otherAgents[time.Now().UnixNano()%int64(len(otherAgents))]
}
