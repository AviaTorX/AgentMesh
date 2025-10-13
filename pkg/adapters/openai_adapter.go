package adapters

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/avinashshinde/agentmesh-cortex/internal/messaging"
	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
)

// OpenAIAdapter wraps an OpenAI Assistant to participate in AgentMesh
//
// Example Usage:
//   adapter := NewOpenAIAdapter(apiKey, assistantID, meshConfig, logger)
//   adapter.Start(ctx)
//   // OpenAI assistant now shares insights with AgentMesh!
type OpenAIAdapter struct {
	apiKey      string
	assistantID string
	threadID    string // OpenAI thread for conversations

	agent      *types.Agent
	messaging  *messaging.KafkaMessaging
	config     *MeshConfig
	logger     *zap.Logger
	filter     *InsightFilter

	httpClient *http.Client
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewOpenAIAdapter creates an adapter for OpenAI Assistant API
func NewOpenAIAdapter(
	apiKey string,
	assistantID string,
	meshConfig *MeshConfig,
	logger *zap.Logger,
) *OpenAIAdapter {
	ctx, cancel := context.WithCancel(context.Background())

	agent := &types.Agent{
		ID:           meshConfig.AgentID,
		Name:         meshConfig.AgentName,
		Role:         meshConfig.Role,
		Status:       types.AgentStatusActive,
		Capabilities: meshConfig.Capabilities,
		Metadata: map[string]string{
			"framework":    "openai",
			"assistant_id": assistantID,
		},
		CreatedAt:  time.Now(),
		LastSeenAt: time.Now(),
	}

	return &OpenAIAdapter{
		apiKey:      apiKey,
		assistantID: assistantID,
		agent:       agent,
		config:      meshConfig,
		logger:      logger.With(zap.String("adapter", "openai"), zap.String("agent_id", string(agent.ID))),
		filter:      DefaultInsightFilter(),
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start connects the OpenAI assistant to AgentMesh
func (oa *OpenAIAdapter) Start(ctx context.Context) error {
	oa.logger.Info("Starting OpenAI adapter")

	// Initialize Kafka messaging
	cfg := &types.Config{
		KafkaBrokers: oa.config.KafkaBrokers,
		RedisAddr:    oa.config.RedisAddr,
	}
	oa.messaging = messaging.NewKafkaMessaging(cfg, oa.logger)

	// Create OpenAI thread
	if err := oa.createThread(); err != nil {
		return fmt.Errorf("failed to create OpenAI thread: %w", err)
	}

	// Publish agent joined event
	joinEvent := types.TopologyEvent{
		Type:      types.TopologyEventAgentJoined,
		AgentID:   oa.agent.ID,
		Timestamp: time.Now(),
	}
	if err := oa.messaging.PublishTopologyEvent(ctx, joinEvent); err != nil {
		return fmt.Errorf("failed to publish join event: %w", err)
	}

	// Start message consumer
	go oa.consumeMessages()

	oa.logger.Info("OpenAI adapter started", zap.String("assistant_id", oa.assistantID))
	return nil
}

// Stop disconnects from AgentMesh
func (oa *OpenAIAdapter) Stop() error {
	oa.logger.Info("Stopping OpenAI adapter")

	// Publish agent left event
	leaveEvent := types.TopologyEvent{
		Type:      types.TopologyEventAgentLeft,
		AgentID:   oa.agent.ID,
		Timestamp: time.Now(),
	}
	oa.messaging.PublishTopologyEvent(oa.ctx, leaveEvent)

	oa.cancel()
	oa.messaging.Close()
	return nil
}

// GetAgent returns agent metadata
func (oa *OpenAIAdapter) GetAgent() *types.Agent {
	return oa.agent
}

// GetCapabilities returns what this agent can do
func (oa *OpenAIAdapter) GetCapabilities() []string {
	return oa.agent.Capabilities
}

// GetRole returns the agent's role
func (oa *OpenAIAdapter) GetRole() string {
	return oa.agent.Role
}

// ShareInsight publishes knowledge to the mesh
func (oa *OpenAIAdapter) ShareInsight(ctx context.Context, insight *types.Insight) error {
	insight.AgentID = oa.agent.ID
	insight.AgentRole = oa.agent.Role

	if err := oa.messaging.PublishInsight(ctx, insight); err != nil {
		return fmt.Errorf("failed to publish insight: %w", err)
	}

	oa.logger.Info("Shared insight",
		zap.String("insight_id", string(insight.ID)),
		zap.String("topic", insight.Topic),
	)

	return nil
}

// ReceiveInsight is called when another agent shares knowledge
func (oa *OpenAIAdapter) ReceiveInsight(ctx context.Context, insight *types.Insight) error {
	// Filter based on agent's interests
	if !oa.matchesFilter(insight) {
		return nil
	}

	oa.logger.Info("Received insight from mesh",
		zap.String("insight_id", string(insight.ID)),
		zap.String("from_agent", string(insight.AgentID)),
		zap.String("topic", insight.Topic),
	)

	// In a full implementation, you could:
	// 1. Add insight to OpenAI assistant's knowledge
	// 2. Update assistant instructions
	// 3. Store in vector database for retrieval

	return nil
}

// SendMessage sends a message to another agent
func (oa *OpenAIAdapter) SendMessage(ctx context.Context, toAgentID types.AgentID, msgType types.MessageType, payload map[string]any) error {
	message := &types.Message{
		ID:          fmt.Sprintf("%s-%d", oa.agent.ID, time.Now().UnixNano()),
		FromAgentID: oa.agent.ID,
		ToAgentID:   toAgentID,
		Type:        msgType,
		Payload:     payload,
		Metadata:    map[string]string{"framework": "openai"},
		Timestamp:   time.Now(),
		EdgeID:      types.NewEdgeID(oa.agent.ID, toAgentID),
	}

	return oa.messaging.PublishMessage(ctx, "messages", message)
}

// ReceiveMessage processes an incoming message
func (oa *OpenAIAdapter) ReceiveMessage(ctx context.Context, msg *types.Message) error {
	oa.logger.Info("Received message",
		zap.String("from", string(msg.FromAgentID)),
		zap.String("type", string(msg.Type)),
	)

	// In a full implementation:
	// 1. Convert message to OpenAI format
	// 2. Send to assistant via API
	// 3. Process assistant response
	// 4. Extract insights
	// 5. Share insights back to mesh

	// For demo: Extract a simple insight
	insight := types.NewInsight(
		oa.agent.ID,
		oa.agent.Role,
		types.InsightTypeCustomerFeedback,
		"message_processing",
		fmt.Sprintf("OpenAI assistant processed message of type %s", msg.Type),
		0.6,
	)

	return oa.ShareInsight(ctx, insight)
}

// consumeMessages listens for messages from the mesh
func (oa *OpenAIAdapter) consumeMessages() {
	groupID := fmt.Sprintf("openai-%s", oa.agent.ID)
	err := oa.messaging.ConsumeMessages(oa.ctx, "messages", groupID, func(msg *types.Message) error {
		if msg.ToAgentID != oa.agent.ID {
			return nil
		}
		return oa.ReceiveMessage(oa.ctx, msg)
	})

	if err != nil && err != context.Canceled {
		oa.logger.Error("Message consumption stopped", zap.Error(err))
	}
}

// matchesFilter checks if an insight matches the agent's filter
func (oa *OpenAIAdapter) matchesFilter(insight *types.Insight) bool {
	// Check confidence
	if insight.Confidence < oa.filter.MinConfidence {
		return false
	}

	// Check topics
	if len(oa.filter.Topics) > 0 {
		found := false
		for _, topic := range oa.filter.Topics {
			if insight.Topic == topic {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check privacy
	if len(oa.filter.PrivacyLevels) > 0 {
		found := false
		for _, privacy := range oa.filter.PrivacyLevels {
			if insight.Privacy == privacy {
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

// createThread creates an OpenAI conversation thread
func (oa *OpenAIAdapter) createThread() error {
	// Simplified: In production, call OpenAI API to create thread
	oa.threadID = fmt.Sprintf("thread_%d", time.Now().Unix())
	oa.logger.Info("Created OpenAI thread", zap.String("thread_id", oa.threadID))
	return nil
}

// callOpenAI makes an API call to OpenAI (stub for demo)
func (oa *OpenAIAdapter) callOpenAI(endpoint string, payload interface{}) (map[string]interface{}, error) {
	// In production implementation:
	// 1. Marshal payload to JSON
	// 2. Create HTTP request with Authorization header
	// 3. Send to https://api.openai.com/v1/{endpoint}
	// 4. Parse response

	oa.logger.Debug("OpenAI API call (stub)", zap.String("endpoint", endpoint))
	return map[string]interface{}{"status": "ok"}, nil
}

// SetInsightFilter configures what insights this agent wants to receive
func (oa *OpenAIAdapter) SetInsightFilter(filter *InsightFilter) {
	oa.filter = filter
	oa.logger.Info("Updated insight filter",
		zap.Int("topics", len(filter.Topics)),
		zap.Float64("min_confidence", filter.MinConfidence),
	)
}
