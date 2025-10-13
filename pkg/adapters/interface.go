package adapters

import (
	"context"

	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
)

// AgentAdapter is the interface that any agent framework must implement
// to participate in the AgentMesh knowledge network.
//
// This enables interoperability between:
// - Lyzr SDK agents
// - LangChain agents
// - CrewAI agents
// - OpenAI assistants
// - Custom agent implementations
type AgentAdapter interface {
	// GetAgent returns the agent metadata
	GetAgent() *types.Agent

	// ShareInsight publishes knowledge learned by this agent to the mesh
	ShareInsight(ctx context.Context, insight *types.Insight) error

	// ReceiveInsight is called when another agent shares knowledge
	// The agent can choose to incorporate this into its own knowledge base
	ReceiveInsight(ctx context.Context, insight *types.Insight) error

	// SendMessage sends a message to another agent in the mesh
	SendMessage(ctx context.Context, toAgentID types.AgentID, msgType types.MessageType, payload map[string]any) error

	// ReceiveMessage is called when this agent receives a message
	// Returns an error if the message cannot be processed
	ReceiveMessage(ctx context.Context, msg *types.Message) error

	// Start initializes the agent and connects it to the mesh
	Start(ctx context.Context) error

	// Stop gracefully shuts down the agent
	Stop() error

	// GetCapabilities returns what this agent can do
	GetCapabilities() []string

	// GetRole returns the agent's role (e.g., "sales", "support")
	GetRole() string
}

// MeshConfig provides configuration for connecting to AgentMesh
type MeshConfig struct {
	// Kafka brokers for message passing
	KafkaBrokers []string

	// Redis address for state storage
	RedisAddr string

	// Agent metadata
	AgentID   types.AgentID
	AgentName string
	Role      string
	Capabilities []string
}

// InsightFilter allows agents to control what knowledge they receive
type InsightFilter struct {
	// Topics of interest (empty = all topics)
	Topics []string

	// Agent roles to subscribe to (empty = all roles)
	AgentRoles []string

	// Minimum confidence threshold (0.0 - 1.0)
	MinConfidence float64

	// Privacy levels to accept
	PrivacyLevels []types.InsightPrivacy
}

// DefaultInsightFilter returns a filter that accepts all public insights
func DefaultInsightFilter() *InsightFilter {
	return &InsightFilter{
		Topics:        []string{}, // All topics
		AgentRoles:    []string{}, // All roles
		MinConfidence: 0.0,        // All confidences
		PrivacyLevels: []types.InsightPrivacy{types.InsightPrivacyPublic},
	}
}

// AgentMetrics provides observability for adapters
type AgentMetrics struct {
	InsightsShared   int64
	InsightsReceived int64
	MessagesSent     int64
	MessagesReceived int64
	ErrorCount       int64
}

// HealthStatus indicates the agent's health
type HealthStatus struct {
	Healthy   bool
	Status    string // "running", "degraded", "stopped"
	LastError error
	Uptime    int64 // seconds
}
