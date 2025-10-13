package types

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AgentID is a unique identifier for an agent
type AgentID string

// EdgeID is a unique identifier for an edge between agents
type EdgeID string

// ProposalID is a unique identifier for a consensus proposal
type ProposalID string

// Agent represents an autonomous agent in the mesh
type Agent struct {
	ID           AgentID           `json:"id"`
	Name         string            `json:"name"`
	Role         string            `json:"role"` // e.g., "sales", "support", "inventory"
	Status       AgentStatus       `json:"status"`
	Metadata     map[string]string `json:"metadata"`
	Capabilities []string          `json:"capabilities"`
	CreatedAt    time.Time         `json:"created_at"`
	LastSeenAt   time.Time         `json:"last_seen_at"`
}

// AgentStatus represents the operational state of an agent
type AgentStatus string

const (
	AgentStatusActive  AgentStatus = "active"
	AgentStatusIdle    AgentStatus = "idle"
	AgentStatusBusy    AgentStatus = "busy"
	AgentStatusOffline AgentStatus = "offline"
)

// Edge represents a communication path between two agents (SlimeMold topology)
type Edge struct {
	ID        EdgeID    `json:"id"`
	SourceID  AgentID   `json:"source_id"`
	TargetID  AgentID   `json:"target_id"`
	Weight    float64   `json:"weight"` // Pheromone strength (0.0 - 1.0)
	Usage     int64     `json:"usage"`  // Message count through this edge
	LastUsed  time.Time `json:"last_used"`
	CreatedAt time.Time `json:"created_at"`

	mu sync.RWMutex `json:"-"`
}

// Reinforce increases the edge weight (SlimeMold reinforcement)
func (e *Edge) Reinforce(amount float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Weight = min(1.0, e.Weight+amount)
	e.Usage++
	e.LastUsed = time.Now()
}

// Decay decreases the edge weight over time (SlimeMold evaporation)
func (e *Edge) Decay(rate float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Weight = max(0.0, e.Weight-rate)
}

// GetWeight safely retrieves the edge weight
func (e *Edge) GetWeight() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.Weight
}

// Message represents a communication between agents
type Message struct {
	ID          string            `json:"id"`
	FromAgentID AgentID           `json:"from_agent_id"`
	ToAgentID   AgentID           `json:"to_agent_id"`
	Type        MessageType       `json:"type"`
	Payload     map[string]any    `json:"payload"`
	Metadata    map[string]string `json:"metadata"`
	Timestamp   time.Time         `json:"timestamp"`
	EdgeID      EdgeID            `json:"edge_id,omitempty"`
}

// MessageType defines the kind of message
type MessageType string

const (
	MessageTypeTask      MessageType = "task"
	MessageTypeResponse  MessageType = "response"
	MessageTypeWaggle    MessageType = "waggle" // Bee consensus broadcast
	MessageTypeVote      MessageType = "vote"   // Bee consensus vote
	MessageTypeHeartbeat MessageType = "heartbeat"
	MessageTypeTopology  MessageType = "topology" // Topology update
)

// Proposal represents a consensus proposal in the Bee algorithm
type Proposal struct {
	ID         ProposalID       `json:"id"`
	ProposerID AgentID          `json:"proposer_id"`
	Type       ProposalType     `json:"type"`
	Content    map[string]any   `json:"content"`
	Waggle     WaggleDance      `json:"waggle"` // Bee waggle dance
	Votes      map[AgentID]Vote `json:"votes"`
	Status     ProposalStatus   `json:"status"`
	CreatedAt  time.Time        `json:"created_at"`
	ExpiresAt  time.Time        `json:"expires_at"`

	mu sync.RWMutex `json:"-"`
}

// ProposalType defines the kind of proposal
type ProposalType string

const (
	ProposalTypeDecision ProposalType = "decision" // Binary decision
	ProposalTypeAction   ProposalType = "action"   // Execute an action
	ProposalTypeTopology ProposalType = "topology" // Network change
)

// ProposalStatus represents the state of a proposal
type ProposalStatus string

const (
	ProposalStatusPending  ProposalStatus = "pending"
	ProposalStatusAccepted ProposalStatus = "accepted"
	ProposalStatusRejected ProposalStatus = "rejected"
	ProposalStatusExpired  ProposalStatus = "expired"
)

// WaggleDance represents the Bee algorithm's communication dance
type WaggleDance struct {
	Intensity   float64 `json:"intensity"`   // How strongly the proposer believes (0.0-1.0)
	Duration    int     `json:"duration"`    // Waggle duration in milliseconds
	Angle       float64 `json:"angle"`       // Direction/quality indicator
	Repetitions int     `json:"repetitions"` // Number of waggles
}

// Vote represents an agent's vote on a proposal
type Vote struct {
	VoterID   AgentID   `json:"voter_id"`
	Support   bool      `json:"support"`   // true = accept, false = reject
	Intensity float64   `json:"intensity"` // How strongly they support (0.0-1.0)
	Timestamp time.Time `json:"timestamp"`
}

// AddVote adds a vote to the proposal (thread-safe)
func (p *Proposal) AddVote(vote Vote) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Votes[vote.VoterID] = vote
}

// GetQuorum calculates the current quorum percentage
func (p *Proposal) GetQuorum(totalAgents int) float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if totalAgents == 0 {
		return 0.0
	}

	supportCount := 0
	for _, vote := range p.Votes {
		if vote.Support {
			supportCount++
		}
	}

	return float64(supportCount) / float64(totalAgents)
}

// TopologyEvent represents a change in the network topology
type TopologyEvent struct {
	Type      TopologyEventType `json:"type"`
	EdgeID    EdgeID            `json:"edge_id,omitempty"`
	AgentID   AgentID           `json:"agent_id,omitempty"`
	Edge      *Edge             `json:"edge,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// TopologyEventType defines topology change types
type TopologyEventType string

const (
	TopologyEventEdgeCreated  TopologyEventType = "edge_created"
	TopologyEventEdgeRemoved  TopologyEventType = "edge_removed"
	TopologyEventEdgeStrength TopologyEventType = "edge_strength_changed"
	TopologyEventAgentJoined  TopologyEventType = "agent_joined"
	TopologyEventAgentLeft    TopologyEventType = "agent_left"
)

// GraphSnapshot represents the state of the network at a point in time
type GraphSnapshot struct {
	Agents    map[AgentID]*Agent `json:"agents"`
	Edges     map[EdgeID]*Edge   `json:"edges"`
	Timestamp time.Time          `json:"timestamp"`
	Stats     GraphStats         `json:"stats"`
}

// GraphStats contains metrics about the network topology
type GraphStats struct {
	TotalAgents      int     `json:"total_agents"`
	TotalEdges       int     `json:"total_edges"`
	ActiveEdges      int     `json:"active_edges"` // Weight > 0.1
	AverageWeight    float64 `json:"average_weight"`
	MaxWeight        float64 `json:"max_weight"`
	MinWeight        float64 `json:"min_weight"`
	Density          float64 `json:"density"`           // Actual edges / possible edges
	ReductionPercent float64 `json:"reduction_percent"` // % reduction from full mesh
}

// ============================================================================
// Knowledge Layer Types - Collective Intelligence
// ============================================================================

// InsightID is a unique identifier for an insight
type InsightID string

// Insight represents knowledge learned by an agent and shared to the mesh
type Insight struct {
	ID         InsightID         `json:"id"`
	AgentID    AgentID           `json:"agent_id"`
	AgentRole  string            `json:"agent_role"`
	Type       InsightType       `json:"type"`
	Topic      string            `json:"topic"`      // e.g., "pricing", "customer_complaint", "product_quality"
	Content    string            `json:"content"`    // Natural language description
	Data       map[string]any    `json:"data"`       // Structured data
	Confidence float64           `json:"confidence"` // 0.0 - 1.0
	Tags       []string          `json:"tags"`
	Metadata   map[string]string `json:"metadata"`
	CreatedAt  time.Time         `json:"created_at"`

	// Privacy controls
	Privacy    InsightPrivacy    `json:"privacy"`
	SharedWith []AgentID         `json:"shared_with,omitempty"` // If privacy is "restricted"
}

// InsightType categorizes the kind of insight
type InsightType string

const (
	InsightTypeCustomerFeedback InsightType = "customer_feedback"
	InsightTypePricingIssue     InsightType = "pricing_issue"
	InsightTypeProductIssue     InsightType = "product_issue"
	InsightTypeProcessImprovement InsightType = "process_improvement"
	InsightTypeFraudPattern     InsightType = "fraud_pattern"
	InsightTypeInventoryTrend   InsightType = "inventory_trend"
	InsightTypeBehaviorPattern  InsightType = "behavior_pattern"
	InsightTypeCorrelation      InsightType = "correlation"
	InsightTypeAnomaly          InsightType = "anomaly"
)

// InsightPrivacy controls who can access the insight
type InsightPrivacy string

const (
	InsightPrivacyPublic     InsightPrivacy = "public"     // All agents can see
	InsightPrivacyRestricted InsightPrivacy = "restricted" // Only specific agents
	InsightPrivacyPrivate    InsightPrivacy = "private"    // Only the creating agent
)

// KnowledgeQuery represents a request to query the collective knowledge
type KnowledgeQuery struct {
	Question      string         `json:"question"`       // Natural language question
	Topics        []string       `json:"topics"`         // Filter by topics
	AgentTypes    []string       `json:"agent_types"`    // Filter by agent roles
	InsightTypes  []InsightType  `json:"insight_types"`  // Filter by insight type
	MinConfidence float64        `json:"min_confidence"` // Minimum confidence threshold
	TimeFrom      *time.Time     `json:"time_from"`      // Start time filter
	TimeTo        *time.Time     `json:"time_to"`        // End time filter
	Limit         int            `json:"limit"`          // Max results
}

// KnowledgeQueryResult represents the response to a knowledge query
type KnowledgeQueryResult struct {
	Query     KnowledgeQuery `json:"query"`
	Insights  []Insight      `json:"insights"`
	Count     int            `json:"count"`
	Patterns  []Pattern      `json:"patterns,omitempty"` // Detected patterns across insights
	Timestamp time.Time      `json:"timestamp"`
}

// Pattern represents an emergent pattern detected across multiple insights
type Pattern struct {
	ID          string      `json:"id"`
	Type        string      `json:"type"`        // e.g., "repeated_complaint", "correlation"
	Description string      `json:"description"` // Natural language summary
	Insights    []InsightID `json:"insights"`    // Supporting insights
	Frequency   int         `json:"frequency"`   // How often this pattern appears
	Confidence  float64     `json:"confidence"`
	DetectedAt  time.Time   `json:"detected_at"`
}

// NewInsightID generates a new unique insight ID
func NewInsightID() InsightID {
	return InsightID(fmt.Sprintf("insight-%d", time.Now().UnixNano()))
}

// NewInsight creates a new insight with defaults
func NewInsight(agentID AgentID, agentRole string, insightType InsightType, topic string, content string, confidence float64) *Insight {
	return &Insight{
		ID:         NewInsightID(),
		AgentID:    agentID,
		AgentRole:  agentRole,
		Type:       insightType,
		Topic:      topic,
		Content:    content,
		Data:       make(map[string]any),
		Confidence: confidence,
		Tags:       []string{},
		Metadata:   make(map[string]string),
		CreatedAt:  time.Now(),
		Privacy:    InsightPrivacyPublic, // Default to public
	}
}

// Config holds runtime configuration
type Config struct {
	// Topology settings
	InitialEdgeWeight   float64       `json:"initial_edge_weight"`
	ReinforcementAmount float64       `json:"reinforcement_amount"`
	DecayRate           float64       `json:"decay_rate"`
	DecayInterval       time.Duration `json:"decay_interval"`
	PruneThreshold      float64       `json:"prune_threshold"`

	// Consensus settings
	QuorumThreshold    float64       `json:"quorum_threshold"` // 0.6 = 60%
	ProposalTimeout    time.Duration `json:"proposal_timeout"`
	WaggleIntensityMin float64       `json:"waggle_intensity_min"`

	// Infrastructure
	KafkaBrokers     []string `json:"kafka_brokers"`
	KafkaTopicPrefix string   `json:"kafka_topic_prefix"`
	RedisAddr        string   `json:"redis_addr"`
	RedisDB          int      `json:"redis_db"`

	// Server
	HTTPPort      int `json:"http_port"`
	WebSocketPort int `json:"websocket_port"`
}

// Helper functions
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// NewAgentID generates a new unique agent ID
func NewAgentID() AgentID {
	return AgentID(uuid.New().String())
}

// NewEdgeID generates a new unique edge ID
func NewEdgeID(sourceID, targetID AgentID) EdgeID {
	return EdgeID(string(sourceID) + "->" + string(targetID))
}

// NewProposalID generates a new unique proposal ID
func NewProposalID() ProposalID {
	return ProposalID(uuid.New().String())
}
