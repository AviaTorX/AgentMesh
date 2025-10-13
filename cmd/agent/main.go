package main

import (
	"context"
	"flag"
	"fmt"
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
	flag.Parse()

	if *agentName == "" || *agentRole == "" {
		fmt.Println("Usage: agent -name=<name> -role=<role> -capabilities=<cap1,cap2>")
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
		Timestamp: time.Now(),
	}
	if err := da.messaging.PublishTopologyEvent(ctx, joinEvent); err != nil {
		return fmt.Errorf("failed to publish join event: %w", err)
	}

	// Start message consumer
	go da.consumeMessages()

	// Start heartbeat sender
	go da.sendHeartbeats()

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
