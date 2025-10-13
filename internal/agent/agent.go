package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/avinashshinde/agentmesh-cortex/internal/consensus"
	"github.com/avinashshinde/agentmesh-cortex/internal/messaging"
	"github.com/avinashshinde/agentmesh-cortex/internal/topology"
	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
	"go.uber.org/zap"
)

// Agent represents an autonomous agent in the mesh
type AgentRuntime struct {
	agent     *types.Agent
	topology  *topology.SlimeMoldTopology
	consensus *consensus.BeeConsensus
	messaging *messaging.KafkaMessaging
	logger    *zap.Logger
	config    *types.Config

	handlers map[types.MessageType]MessageHandler
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// MessageHandler is a function that handles incoming messages
type MessageHandler func(msg *types.Message) error

// NewAgentRuntime creates a new agent runtime
func NewAgentRuntime(
	agent *types.Agent,
	topology *topology.SlimeMoldTopology,
	consensus *consensus.BeeConsensus,
	messaging *messaging.KafkaMessaging,
	config *types.Config,
	logger *zap.Logger,
) *AgentRuntime {
	ctx, cancel := context.WithCancel(context.Background())

	return &AgentRuntime{
		agent:     agent,
		topology:  topology,
		consensus: consensus,
		messaging: messaging,
		config:    config,
		logger:    logger.With(zap.String("agent_id", string(agent.ID)), zap.String("agent_name", agent.Name)),
		handlers:  make(map[types.MessageType]MessageHandler),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// RegisterHandler registers a message handler for a message type
func (ar *AgentRuntime) RegisterHandler(msgType types.MessageType, handler MessageHandler) {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	ar.handlers[msgType] = handler
}

// Start starts the agent runtime
func (ar *AgentRuntime) Start() error {
	ar.logger.Info("Starting agent runtime",
		zap.String("role", ar.agent.Role),
		zap.Strings("capabilities", ar.agent.Capabilities),
	)

	// Register agent in consensus
	ar.consensus.RegisterAgent(ar.agent.ID)

	// Add agent to topology
	if err := ar.topology.AddAgent(ar.agent); err != nil {
		return fmt.Errorf("failed to add agent to topology: %w", err)
	}

	// Start message consumers
	ar.wg.Add(3)
	go ar.consumeMessages()
	go ar.consumeProposals()
	go ar.sendHeartbeats()

	return nil
}

// Stop stops the agent runtime
func (ar *AgentRuntime) Stop() error {
	ar.logger.Info("Stopping agent runtime")
	ar.cancel()
	ar.wg.Wait()

	// Unregister from consensus
	ar.consensus.UnregisterAgent(ar.agent.ID)

	// Remove from topology
	if err := ar.topology.RemoveAgent(ar.agent.ID); err != nil {
		ar.logger.Warn("Failed to remove agent from topology", zap.Error(err))
	}

	return nil
}

// SendMessage sends a message to another agent
func (ar *AgentRuntime) SendMessage(toAgentID types.AgentID, msgType types.MessageType, payload map[string]any) error {
	message := &types.Message{
		ID:          fmt.Sprintf("%s-%d", ar.agent.ID, time.Now().UnixNano()),
		FromAgentID: ar.agent.ID,
		ToAgentID:   toAgentID,
		Type:        msgType,
		Payload:     payload,
		Metadata:    map[string]string{"agent_role": ar.agent.Role},
		Timestamp:   time.Now(),
		EdgeID:      types.NewEdgeID(ar.agent.ID, toAgentID),
	}

	// Publish message to Kafka
	if err := ar.messaging.PublishMessage(ar.ctx, "messages", message); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	// Reinforce edge in topology
	if err := ar.topology.ReinforceEdge(ar.agent.ID, toAgentID); err != nil {
		ar.logger.Warn("Failed to reinforce edge", zap.Error(err))
	}

	ar.logger.Debug("Sent message",
		zap.String("to", string(toAgentID)),
		zap.String("type", string(msgType)),
	)

	return nil
}

// ProposeAction creates a new proposal for consensus
func (ar *AgentRuntime) ProposeAction(proposalType types.ProposalType, content map[string]any) (*types.Proposal, error) {
	proposal, err := ar.consensus.CreateProposal(ar.agent.ID, proposalType, content)
	if err != nil {
		return nil, fmt.Errorf("failed to create proposal: %w", err)
	}

	// Publish proposal to Kafka
	if err := ar.messaging.PublishProposal(ar.ctx, proposal); err != nil {
		ar.logger.Error("Failed to publish proposal", zap.Error(err))
	}

	ar.logger.Info("Proposed action",
		zap.String("proposal_id", string(proposal.ID)),
		zap.String("type", string(proposalType)),
	)

	return proposal, nil
}

// VoteOnProposal votes on a proposal
func (ar *AgentRuntime) VoteOnProposal(proposalID types.ProposalID, support bool, intensity float64) error {
	if err := ar.consensus.Vote(proposalID, ar.agent.ID, support, intensity); err != nil {
		return fmt.Errorf("failed to vote: %w", err)
	}

	ar.logger.Debug("Voted on proposal",
		zap.String("proposal_id", string(proposalID)),
		zap.Bool("support", support),
		zap.Float64("intensity", intensity),
	)

	return nil
}

// consumeMessages consumes messages from Kafka
func (ar *AgentRuntime) consumeMessages() {
	defer ar.wg.Done()

	groupID := fmt.Sprintf("agent-%s", ar.agent.ID)
	err := ar.messaging.ConsumeMessages(ar.ctx, "messages", groupID, func(msg *types.Message) error {
		// Only process messages addressed to this agent
		if msg.ToAgentID != ar.agent.ID {
			return nil
		}

		ar.mu.RLock()
		handler, exists := ar.handlers[msg.Type]
		ar.mu.RUnlock()

		if exists {
			return handler(msg)
		}

		ar.logger.Debug("No handler for message type", zap.String("type", string(msg.Type)))
		return nil
	})

	if err != nil && err != context.Canceled {
		ar.logger.Error("Message consumption stopped", zap.Error(err))
	}
}

// consumeProposals consumes proposals from Kafka
func (ar *AgentRuntime) consumeProposals() {
	defer ar.wg.Done()

	groupID := "proposals-all"
	err := ar.messaging.ConsumeMessages(ar.ctx, "proposals", groupID, func(msg *types.Message) error {
		// Proposals are broadcast to all agents - evaluate and vote
		return ar.evaluateProposal(msg)
	})

	if err != nil && err != context.Canceled {
		ar.logger.Error("Proposal consumption stopped", zap.Error(err))
	}
}

// evaluateProposal evaluates a proposal and decides whether to vote
func (ar *AgentRuntime) evaluateProposal(msg *types.Message) error {
	// Simple voting logic: vote based on waggle dance intensity
	// In a real system, agents would use their own decision-making logic

	proposalData, ok := msg.Payload["proposal"]
	if !ok {
		return nil
	}

	// Extract waggle dance
	waggleData, ok := proposalData.(map[string]any)["waggle"]
	if !ok {
		return nil
	}

	waggle, ok := waggleData.(types.WaggleDance)
	if !ok {
		return nil
	}

	// Decision logic: support if waggle intensity is high enough
	support := waggle.Intensity >= ar.config.WaggleIntensityMin
	voteIntensity := waggle.Intensity

	// Cast vote
	proposalID := types.ProposalID(msg.Payload["proposal_id"].(string))
	return ar.VoteOnProposal(proposalID, support, voteIntensity)
}

// sendHeartbeats sends periodic heartbeats
func (ar *AgentRuntime) sendHeartbeats() {
	defer ar.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ar.ctx.Done():
			return
		case <-ticker.C:
			ar.agent.LastSeenAt = time.Now()
			ar.agent.Status = types.AgentStatusActive

			ar.logger.Debug("Heartbeat sent")
		}
	}
}

// GetAgent returns the agent instance
func (ar *AgentRuntime) GetAgent() *types.Agent {
	return ar.agent
}

// SetStatus sets the agent status
func (ar *AgentRuntime) SetStatus(status types.AgentStatus) {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	ar.agent.Status = status
}
