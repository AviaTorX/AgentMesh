package adapters

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/avinashshinde/agentmesh-cortex/internal/messaging"
	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
)

// LangChainAdapter wraps a LangChain agent to participate in AgentMesh
//
// This is a mock implementation showing how LangChain agents would integrate.
// In production, this would use LangChain's Python/Go SDK.
//
// Example Usage:
//   adapter := NewLangChainAdapter(agentConfig, meshConfig, logger)
//   adapter.Start(ctx)
//   // LangChain agent now shares insights with AgentMesh!
type LangChainAdapter struct {
	agent      *types.Agent
	messaging  *messaging.KafkaMessaging
	config     *MeshConfig
	logger     *zap.Logger
	filter     *InsightFilter

	// Mock LangChain specific fields
	chain      string // e.g., "ConversationalRetrievalChain"
	vectorStore string // e.g., "Pinecone", "Chroma"

	ctx    context.Context
	cancel context.CancelFunc
}

// NewLangChainAdapter creates an adapter for LangChain agents
func NewLangChainAdapter(
	agentConfig map[string]interface{}, // LangChain agent configuration
	meshConfig *MeshConfig,
	logger *zap.Logger,
) *LangChainAdapter {
	ctx, cancel := context.WithCancel(context.Background())

	agent := &types.Agent{
		ID:           meshConfig.AgentID,
		Name:         meshConfig.AgentName,
		Role:         meshConfig.Role,
		Status:       types.AgentStatusActive,
		Capabilities: meshConfig.Capabilities,
		Metadata: map[string]string{
			"framework": "langchain",
			"chain_type": getStringFromConfig(agentConfig, "chain", "ConversationalChain"),
			"llm": getStringFromConfig(agentConfig, "llm", "gpt-3.5-turbo"),
		},
		CreatedAt:  time.Now(),
		LastSeenAt: time.Now(),
	}

	return &LangChainAdapter{
		agent:       agent,
		config:      meshConfig,
		logger:      logger.With(zap.String("adapter", "langchain"), zap.String("agent_id", string(agent.ID))),
		filter:      DefaultInsightFilter(),
		chain:       getStringFromConfig(agentConfig, "chain", "ConversationalChain"),
		vectorStore: getStringFromConfig(agentConfig, "vector_store", "memory"),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start connects the LangChain agent to AgentMesh
func (lc *LangChainAdapter) Start(ctx context.Context) error {
	lc.logger.Info("Starting LangChain adapter",
		zap.String("chain", lc.chain),
		zap.String("vector_store", lc.vectorStore),
	)

	// Initialize Kafka messaging
	cfg := &types.Config{
		KafkaBrokers:     lc.config.KafkaBrokers,
		KafkaTopicPrefix: "agentmesh",
		RedisAddr:        lc.config.RedisAddr,
	}
	lc.messaging = messaging.NewKafkaMessaging(cfg, lc.logger)

	// Publish agent joined event
	joinEvent := types.TopologyEvent{
		Type:      types.TopologyEventAgentJoined,
		AgentID:   lc.agent.ID,
		Agent:     lc.agent,
		Timestamp: time.Now(),
	}
	if err := lc.messaging.PublishTopologyEvent(ctx, joinEvent); err != nil {
		return fmt.Errorf("failed to publish join event: %w", err)
	}

	// Start message consumer
	go lc.consumeMessages()

	// Simulate LangChain agent running
	go lc.simulateLangChainAgent()

	lc.logger.Info("LangChain adapter started")
	return nil
}

// Stop disconnects from AgentMesh
func (lc *LangChainAdapter) Stop() error {
	lc.logger.Info("Stopping LangChain adapter")

	// Publish agent left event
	leaveEvent := types.TopologyEvent{
		Type:      types.TopologyEventAgentLeft,
		AgentID:   lc.agent.ID,
		Timestamp: time.Now(),
	}
	lc.messaging.PublishTopologyEvent(lc.ctx, leaveEvent)

	lc.cancel()
	lc.messaging.Close()
	return nil
}

// GetAgent returns agent metadata
func (lc *LangChainAdapter) GetAgent() *types.Agent {
	return lc.agent
}

// GetCapabilities returns what this agent can do
func (lc *LangChainAdapter) GetCapabilities() []string {
	return lc.agent.Capabilities
}

// GetRole returns the agent's role
func (lc *LangChainAdapter) GetRole() string {
	return lc.agent.Role
}

// ShareInsight publishes knowledge to the mesh
func (lc *LangChainAdapter) ShareInsight(ctx context.Context, insight *types.Insight) error {
	insight.AgentID = lc.agent.ID
	insight.AgentRole = lc.agent.Role

	if err := lc.messaging.PublishInsight(ctx, insight); err != nil {
		return fmt.Errorf("failed to publish insight: %w", err)
	}

	lc.logger.Info("Shared insight",
		zap.String("insight_id", string(insight.ID)),
		zap.String("topic", insight.Topic),
	)

	return nil
}

// ReceiveInsight is called when another agent shares knowledge
func (lc *LangChainAdapter) ReceiveInsight(ctx context.Context, insight *types.Insight) error {
	if !lc.matchesFilter(insight) {
		return nil
	}

	lc.logger.Info("Received insight from mesh",
		zap.String("insight_id", string(insight.ID)),
		zap.String("from_agent", string(insight.AgentID)),
		zap.String("topic", insight.Topic),
	)

	// In production:
	// 1. Add insight to LangChain agent's vector store
	// 2. Update agent memory
	// 3. Incorporate into retrieval chain

	lc.logger.Debug("Added insight to LangChain vector store (mock)",
		zap.String("vector_store", lc.vectorStore),
	)

	return nil
}

// SendMessage sends a message to another agent
func (lc *LangChainAdapter) SendMessage(ctx context.Context, toAgentID types.AgentID, msgType types.MessageType, payload map[string]any) error {
	message := &types.Message{
		ID:          fmt.Sprintf("%s-%d", lc.agent.ID, time.Now().UnixNano()),
		FromAgentID: lc.agent.ID,
		ToAgentID:   toAgentID,
		Type:        msgType,
		Payload:     payload,
		Metadata:    map[string]string{"framework": "langchain", "chain": lc.chain},
		Timestamp:   time.Now(),
		EdgeID:      types.NewEdgeID(lc.agent.ID, toAgentID),
	}

	return lc.messaging.PublishMessage(ctx, "messages", message)
}

// ReceiveMessage processes an incoming message
func (lc *LangChainAdapter) ReceiveMessage(ctx context.Context, msg *types.Message) error {
	lc.logger.Info("Received message",
		zap.String("from", string(msg.FromAgentID)),
		zap.String("type", string(msg.Type)),
	)

	// In production:
	// 1. Convert message to LangChain prompt
	// 2. Execute chain with prompt
	// 3. Process LLM response
	// 4. Extract insights from response
	// 5. Update agent memory
	// 6. Share insights to mesh

	// Mock: Generate insight from message processing
	insight := types.NewInsight(
		lc.agent.ID,
		lc.agent.Role,
		types.InsightTypeBehaviorPattern,
		"langchain_processing",
		fmt.Sprintf("LangChain agent analyzed message and detected pattern: %s", msg.Type),
		0.75,
	)
	insight.Data = map[string]any{
		"chain_type":  lc.chain,
		"message_type": msg.Type,
		"from_agent":   msg.FromAgentID,
	}

	return lc.ShareInsight(ctx, insight)
}

// consumeMessages listens for messages from the mesh
func (lc *LangChainAdapter) consumeMessages() {
	groupID := fmt.Sprintf("langchain-%s", lc.agent.ID)
	err := lc.messaging.ConsumeMessages(lc.ctx, "messages", groupID, func(msg *types.Message) error {
		if msg.ToAgentID != lc.agent.ID {
			return nil
		}
		return lc.ReceiveMessage(lc.ctx, msg)
	})

	if err != nil && err != context.Canceled {
		lc.logger.Error("Message consumption stopped", zap.Error(err))
	}
}

// simulateLangChainAgent simulates the agent doing work and learning
func (lc *LangChainAdapter) simulateLangChainAgent() {
	ticker := time.NewTicker(45 * time.Second)
	defer ticker.Stop()

	scenarios := []struct {
		topic   string
		content string
		insightType types.InsightType
	}{
		{"customer_behavior", "Customers asking more questions about pricing transparency", types.InsightTypeBehaviorPattern},
		{"product_feedback", "Users want mobile app dark mode feature", types.InsightTypeProductIssue},
		{"process_improvement", "Onboarding flow could be streamlined with guided tutorial", types.InsightTypeProcessImprovement},
	}

	count := 0
	for {
		select {
		case <-lc.ctx.Done():
			return
		case <-ticker.C:
			// Simulate LangChain agent learning from interactions
			scenario := scenarios[count%len(scenarios)]

			insight := types.NewInsight(
				lc.agent.ID,
				lc.agent.Role,
				scenario.insightType,
				scenario.topic,
				scenario.content,
				0.80,
			)
			insight.Tags = []string{"langchain", "auto-generated"}
			insight.Metadata = map[string]string{
				"source": "langchain_chain_execution",
				"chain":  lc.chain,
			}

			if err := lc.ShareInsight(lc.ctx, insight); err != nil {
				lc.logger.Error("Failed to share insight", zap.Error(err))
			}

			count++
		}
	}
}

// matchesFilter checks if an insight matches the agent's filter
func (lc *LangChainAdapter) matchesFilter(insight *types.Insight) bool {
	if insight.Confidence < lc.filter.MinConfidence {
		return false
	}

	if len(lc.filter.Topics) > 0 {
		found := false
		for _, topic := range lc.filter.Topics {
			if insight.Topic == topic {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// SetInsightFilter configures what insights this agent wants to receive
func (lc *LangChainAdapter) SetInsightFilter(filter *InsightFilter) {
	lc.filter = filter
	lc.logger.Info("Updated insight filter",
		zap.Int("topics", len(filter.Topics)),
		zap.Float64("min_confidence", filter.MinConfidence),
	)
}

// Helper function to extract string from config map
func getStringFromConfig(config map[string]interface{}, key, defaultValue string) string {
	if val, ok := config[key].(string); ok {
		return val
	}
	return defaultValue
}
