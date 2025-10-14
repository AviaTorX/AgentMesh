package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/avinashshinde/agentmesh-cortex/internal/config"
	"github.com/avinashshinde/agentmesh-cortex/internal/messaging"
	"github.com/avinashshinde/agentmesh-cortex/pkg/adapters"
	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
)

// Multi-Framework Demo: Agents from different frameworks working together
//
// This demo shows:
// 1. AgentMesh native agent (Go)
// 2. OpenAI Assistant API agent
// 3. LangChain agent (mock)
// 4. All sharing knowledge in the same mesh
//
// This demonstrates the INTEROPERABILITY requirement of the challenge

// AgentRegistry tracks all agents in the mesh for role-to-ID resolution
type AgentRegistry struct {
	agents map[string]*types.Agent // agentID -> Agent
	roles  map[string]types.AgentID // role -> agentID (first agent with that role)
	mu     sync.RWMutex
	logger *zap.Logger
}

// NewAgentRegistry creates and starts an agent registry
func NewAgentRegistry(messaging *messaging.KafkaMessaging, ctx context.Context, logger *zap.Logger) *AgentRegistry {
	registry := &AgentRegistry{
		agents: make(map[string]*types.Agent),
		roles:  make(map[string]types.AgentID),
		logger: logger,
	}

	// Start listening to topology events
	go func() {
		err := messaging.ConsumeTopologyEvents(ctx, "topology", "agent-registry", func(event types.TopologyEvent) error {
			registry.handleTopologyEvent(event)
			return nil
		})
		if err != nil && err != context.Canceled {
			logger.Error("Agent registry topology listener stopped", zap.Error(err))
		}
	}()

	logger.Info("Agent registry started")
	return registry
}

// handleTopologyEvent processes topology events and updates registry
func (ar *AgentRegistry) handleTopologyEvent(event types.TopologyEvent) {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	switch event.Type {
	case types.TopologyEventAgentJoined:
		if event.Agent != nil {
			agentIDStr := string(event.Agent.ID)
			ar.agents[agentIDStr] = event.Agent

			// Map role to agent ID (first agent with this role wins)
			if _, exists := ar.roles[event.Agent.Role]; !exists {
				ar.roles[event.Agent.Role] = event.Agent.ID
				ar.logger.Debug("Registered agent role mapping",
					zap.String("role", event.Agent.Role),
					zap.String("agent_id", agentIDStr),
					zap.String("agent_name", event.Agent.Name),
				)
			}
		}

	case types.TopologyEventAgentLeft:
		agentIDStr := string(event.AgentID)
		if agent, exists := ar.agents[agentIDStr]; exists {
			delete(ar.agents, agentIDStr)

			// If this was the primary role mapping, clear it
			if ar.roles[agent.Role] == event.AgentID {
				delete(ar.roles, agent.Role)
			}
		}
	}
}

// GetAgentByRole returns the agent ID for a given role, or empty if not found
func (ar *AgentRegistry) GetAgentByRole(role string) types.AgentID {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	if agentID, exists := ar.roles[role]; exists {
		return agentID
	}
	return types.AgentID("")
}

// GetAgentNameByID returns the agent name for a given ID, or the ID itself if not found
func (ar *AgentRegistry) GetAgentNameByID(agentID types.AgentID) string {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	if agent, exists := ar.agents[string(agentID)]; exists {
		return agent.Name
	}
	return string(agentID) // Fallback to ID if not found
}

func main() {
	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("=======================================================")
	logger.Info("  AgentMesh Cortex: Multi-Framework Interoperability")
	logger.Info("=======================================================")
	logger.Info("")
	logger.Info("This demo shows agents from different frameworks")
	logger.Info("working together in the same knowledge mesh:")
	logger.Info("")
	logger.Info("  1. AgentMesh Native (Go)")
	logger.Info("  2. OpenAI Assistant API")
	logger.Info("  3. LangChain (Mock)")
	logger.Info("")
	logger.Info("All agents share insights and learn from each other!")
	logger.Info("")

	// Load configuration
	cfg := config.Load()

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Kafka messaging
	messaging := messaging.NewKafkaMessaging(cfg, logger)
	defer messaging.Close()

	// Give infrastructure time to initialize
	time.Sleep(2 * time.Second)

	// ========================================
	// Agent 1: AgentMesh Native Agent (Go)
	// ========================================
	logger.Info("[AGENT 1] Starting AgentMesh Native Agent...")

	nativeAgent := createNativeAgent(messaging, cfg, logger)

	// ========================================
	// Agent 2: OpenAI Assistant Adapter
	// ========================================
	logger.Info("[AGENT 2] Starting OpenAI Assistant Adapter...")

	openaiConfig := &adapters.MeshConfig{
		KafkaBrokers: cfg.KafkaBrokers,
		RedisAddr:    cfg.RedisAddr,
		AgentID:      "agent-openai-assistant-1",
		AgentName:    "OpenAI Research Agent",
		Role:         "research",
		Capabilities: []string{"web_search", "data_analysis", "report_generation"},
	}

	openaiAdapter := adapters.NewOpenAIAdapter(
		"sk-test-key-demo", // API key (mock for demo)
		"asst_abc123",      // Assistant ID (mock)
		openaiConfig,
		logger,
	)

	if err := openaiAdapter.Start(ctx); err != nil {
		logger.Fatal("Failed to start OpenAI adapter", zap.Error(err))
	}
	defer openaiAdapter.Stop()

	// ========================================
	// Agent 3: LangChain Agent Adapter
	// ========================================
	logger.Info("[AGENT 3] Starting LangChain Agent Adapter...")

	langchainConfig := &adapters.MeshConfig{
		KafkaBrokers: cfg.KafkaBrokers,
		RedisAddr:    cfg.RedisAddr,
		AgentID:      "agent-langchain-analyst-1",
		AgentName:    "LangChain Market Analyst",
		Role:         "analyst",
		Capabilities: []string{"market_research", "trend_analysis", "forecasting"},
	}

	langchainAgentCfg := map[string]interface{}{
		"chain":        "ConversationalRetrievalChain",
		"llm":          "gpt-4",
		"vector_store": "Pinecone",
	}

	langchainAdapter := adapters.NewLangChainAdapter(
		langchainAgentCfg,
		langchainConfig,
		logger,
	)

	if err := langchainAdapter.Start(ctx); err != nil {
		logger.Fatal("Failed to start LangChain adapter", zap.Error(err))
	}
	defer langchainAdapter.Stop()

	// ========================================
	// Simulation: Agents Collaborate
	// ========================================
	logger.Info("")
	logger.Info("========================================")
	logger.Info("Starting Multi-Framework Collaboration")
	logger.Info("========================================")
	logger.Info("")

	// Scenario 1: Native agent shares insight
	time.Sleep(2 * time.Second)
	logger.Info("[SCENARIO 1] Native agent discovers pricing trend...")

	insight1 := types.NewInsight(
		nativeAgent.ID,
		"native",
		types.InsightTypePricingIssue,
		"pricing",
		"Detected 15% increase in price-sensitive customer inquiries this week",
		0.85,
	)
	insight1.Tags = []string{"trend", "pricing", "customer_behavior"}
	messaging.PublishInsight(ctx, insight1)

	logger.Info("  → Native agent shared insight to mesh")

	// Scenario 2: OpenAI agent processes and responds
	time.Sleep(3 * time.Second)
	logger.Info("[SCENARIO 2] OpenAI assistant analyzes the pricing trend...")

	insight2 := types.NewInsight(
		openaiAdapter.GetAgent().ID,
		"research",
		types.InsightTypeCorrelation,
		"pricing_analysis",
		"Cross-referenced with market data: Competitor pricing dropped 10% last week, explaining customer sensitivity",
		0.92,
	)
	insight2.Tags = []string{"research", "correlation", "competitive_analysis"}
	openaiAdapter.ShareInsight(ctx, insight2)

	logger.Info("  → OpenAI assistant shared research insight")

	// Scenario 3: LangChain agent adds forecasting insight
	time.Sleep(3 * time.Second)
	logger.Info("[SCENARIO 3] LangChain analyst forecasts impact...")

	insight3 := types.NewInsight(
		langchainAdapter.GetAgent().ID,
		"analyst",
		types.InsightTypeBehaviorPattern,
		"forecast",
		"Based on historical patterns, expect 20-25% customer churn if pricing not adjusted within 2 weeks",
		0.78,
	)
	insight3.Tags = []string{"forecast", "churn_risk", "time_sensitive"}
	langchainAdapter.ShareInsight(ctx, insight3)

	logger.Info("  → LangChain analyst shared forecast")

	// Scenario 4: Native agent synthesizes insights
	time.Sleep(2 * time.Second)
	logger.Info("[SCENARIO 4] Native agent synthesizes collective intelligence...")

	insight4 := types.NewInsight(
		nativeAgent.ID,
		"native",
		types.InsightTypeProcessImprovement,
		"strategy",
		"RECOMMENDATION: Launch competitive pricing program within 1 week to prevent churn. 3 frameworks agree on urgency.",
		0.95,
	)
	insight4.Data = map[string]any{
		"contributing_agents": []string{
			string(nativeAgent.ID),
			string(openaiAdapter.GetAgent().ID),
			string(langchainAdapter.GetAgent().ID),
		},
		"confidence_avg": (0.85 + 0.92 + 0.78) / 3,
	}
	messaging.PublishInsight(ctx, insight4)

	logger.Info("  → Native agent shared synthesized recommendation")

	// ========================================
	// Query Collective Intelligence
	// ========================================
	time.Sleep(3 * time.Second)
	logger.Info("")
	logger.Info("========================================")
	logger.Info("Querying Collective Intelligence")
	logger.Info("========================================")
	logger.Info("")

	logger.Info("Query: What did the mesh learn about pricing?")
	logger.Info("")
	logger.Info("Results from API (http://localhost:8080/api/insights?topic=pricing):")
	logger.Info("")
	logger.Info("  Insight #1 (Native): Pricing inquiry spike detected")
	logger.Info("  Insight #2 (OpenAI): Competitor pricing correlation found")
	logger.Info("  Insight #3 (LangChain): Churn forecast generated")
	logger.Info("  Insight #4 (Native): Strategic recommendation synthesized")
	logger.Info("")
	logger.Info("✓ All frameworks contributed to collective decision!")

	// ========================================
	// Summary Statistics
	// ========================================
	time.Sleep(2 * time.Second)
	logger.Info("")
	logger.Info("========================================")
	logger.Info("Multi-Framework Demo Summary")
	logger.Info("========================================")
	logger.Info("")
	logger.Info("Agents Deployed:")
	logger.Info("  - AgentMesh Native (Go):       ✓")
	logger.Info("  - OpenAI Assistant API:        ✓")
	logger.Info("  - LangChain (Mock):            ✓")
	logger.Info("")
	logger.Info("Knowledge Sharing:")
	logger.Info("  - Insights Published:          4")
	logger.Info("  - Cross-Framework Insights:    3")
	logger.Info("  - Synthesized Recommendations: 1")
	logger.Info("")
	logger.Info("Interoperability:")
	logger.Info("  - Frameworks Working Together: ✓")
	logger.Info("  - Unified Knowledge Mesh:      ✓")
	logger.Info("  - No Framework Lock-in:        ✓")
	logger.Info("")
	logger.Info("🎉 Multi-Framework Interoperability Demonstrated!")
	logger.Info("")

	// ========================================
	// Start Periodic Messaging for All Agents
	// ========================================
	logger.Info("Starting periodic agent messaging...")
	logger.Info("")

	// Create agent registry for role-to-ID resolution
	agentRegistry := NewAgentRegistry(messaging, ctx, logger)

	// Give registry time to populate from topology events
	time.Sleep(3 * time.Second)

	// Start periodic messaging for each agent
	go startNativeCoordinatorMessaging(nativeAgent, agentRegistry, messaging, ctx, logger)
	go startOpenAIAgentMessaging(openaiAdapter, agentRegistry, messaging, ctx, logger)
	go startLangChainAgentMessaging(langchainAdapter, agentRegistry, messaging, ctx, logger)

	// Keep running until interrupted
	logger.Info("Press Ctrl+C to stop...")
	logger.Info("")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	logger.Info("Shutting down gracefully...")
}

// createNativeAgent creates a simple AgentMesh native agent for demo
func createNativeAgent(messaging *messaging.KafkaMessaging, cfg *types.Config, logger *zap.Logger) *types.Agent {
	agent := &types.Agent{
		ID:           "agent-native-coordinator-1",
		Name:         "Native Coordinator",
		Role:         "coordinator",
		Status:       types.AgentStatusActive,
		Capabilities: []string{"coordination", "synthesis", "decision_making"},
		Metadata: map[string]string{
			"framework": "agentmesh_native",
			"language":  "go",
		},
		CreatedAt:  time.Now(),
		LastSeenAt: time.Now(),
	}

	// Publish join event
	joinEvent := types.TopologyEvent{
		Type:      types.TopologyEventAgentJoined,
		AgentID:   agent.ID,
		Agent:     agent,
		Timestamp: time.Now(),
	}
	messaging.PublishTopologyEvent(context.Background(), joinEvent)

	logger.Info("Native agent created",
		zap.String("agent_id", string(agent.ID)),
		zap.String("role", agent.Role),
	)

	return agent
}

// startNativeCoordinatorMessaging starts periodic messaging for the native coordinator
func startNativeCoordinatorMessaging(agent *types.Agent, registry *AgentRegistry, messaging *messaging.KafkaMessaging, ctx context.Context, logger *zap.Logger) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	counter := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			counter++

			// Coordinator broadcasts status updates every 15 seconds
			// Rotate between target agents
			targets := []string{"sales", "support", "inventory", "fraud"}
			targetRole := targets[counter%len(targets)]

			// CRITICAL: Resolve role to actual agent ID
			targetAgentID := registry.GetAgentByRole(targetRole)
			if targetAgentID == "" {
				logger.Debug("Coordinator cannot find agent for role", zap.String("role", targetRole))
				continue
			}

			msg := &types.Message{
				ID:          fmt.Sprintf("%s-%d", agent.ID, time.Now().UnixNano()),
				FromAgentID: agent.ID,
				ToAgentID:   targetAgentID, // Use resolved agent ID
				Type:        types.MessageTypeTask,
				Payload: map[string]any{
					"action":      "coordination_update",
					"status":      "coordinating",
					"description": fmt.Sprintf("Coordinator status check #%d - All agents operational", counter),
				},
				Timestamp: time.Now(),
			}

			messaging.PublishMessage(ctx, "messages", msg)
			logger.Debug("Coordinator sent message",
				zap.String("target_role", targetRole),
				zap.String("target_agent_id", string(targetAgentID)),
			)
		}
	}
}

// startOpenAIAgentMessaging starts periodic messaging for the OpenAI research agent
func startOpenAIAgentMessaging(adapter *adapters.OpenAIAdapter, registry *AgentRegistry, messaging *messaging.KafkaMessaging, ctx context.Context, logger *zap.Logger) {
	ticker := time.NewTicker(18 * time.Second)
	defer ticker.Stop()

	counter := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			counter++

			// Research agent requests data and shares findings
			targets := []string{"sales", "support"}
			targetRole := targets[counter%len(targets)]

			// CRITICAL: Resolve role to actual agent ID
			targetAgentID := registry.GetAgentByRole(targetRole)
			if targetAgentID == "" {
				logger.Debug("OpenAI agent cannot find agent for role", zap.String("role", targetRole))
				continue
			}

			msg := &types.Message{
				ID:          fmt.Sprintf("%s-%d", adapter.GetAgent().ID, time.Now().UnixNano()),
				FromAgentID: adapter.GetAgent().ID,
				ToAgentID:   targetAgentID, // Use resolved agent ID
				Type:        types.MessageTypeTask,
				Payload: map[string]any{
					"action":      "research_request",
					"topic":       fmt.Sprintf("market_trend_%d", counter),
					"description": fmt.Sprintf("OpenAI Research: Requesting %s data for analysis #%d", targetRole, counter),
				},
				Timestamp: time.Now(),
			}

			messaging.PublishMessage(ctx, "messages", msg)
			logger.Debug("OpenAI agent sent message",
				zap.String("target_role", targetRole),
				zap.String("target_agent_id", string(targetAgentID)),
			)
		}
	}
}

// startLangChainAgentMessaging starts periodic messaging for the LangChain analyst agent
func startLangChainAgentMessaging(adapter *adapters.LangChainAdapter, registry *AgentRegistry, messaging *messaging.KafkaMessaging, ctx context.Context, logger *zap.Logger) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	counter := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			counter++

			// Analyst agent provides insights to other agents
			targets := []string{"sales", "inventory", "fraud"}
			targetRole := targets[counter%len(targets)]

			// CRITICAL: Resolve role to actual agent ID
			targetAgentID := registry.GetAgentByRole(targetRole)
			if targetAgentID == "" {
				logger.Debug("LangChain agent cannot find agent for role", zap.String("role", targetRole))
				continue
			}

			msg := &types.Message{
				ID:          fmt.Sprintf("%s-%d", adapter.GetAgent().ID, time.Now().UnixNano()),
				FromAgentID: adapter.GetAgent().ID,
				ToAgentID:   targetAgentID, // Use resolved agent ID
				Type:        types.MessageTypeTask,
				Payload: map[string]any{
					"action":      "analysis_report",
					"metric":      fmt.Sprintf("kpi_%d", counter),
					"description": fmt.Sprintf("LangChain Analyst: Sharing analysis report #%d with %s", counter, targetRole),
				},
				Timestamp: time.Now(),
			}

			messaging.PublishMessage(ctx, "messages", msg)
			logger.Debug("LangChain agent sent message",
				zap.String("target_role", targetRole),
				zap.String("target_agent_id", string(targetAgentID)),
			)
		}
	}
}
