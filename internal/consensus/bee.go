package consensus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
	"go.uber.org/zap"
)

// BeeConsensus implements the bee-inspired consensus mechanism
type BeeConsensus struct {
	proposals map[types.ProposalID]*types.Proposal
	agents    map[types.AgentID]bool // Track active agents
	config    *types.Config
	logger    *zap.Logger
	eventChan chan ConsensusEvent

	mu     sync.RWMutex
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// ConsensusEvent represents a consensus-related event
type ConsensusEvent struct {
	Type       ConsensusEventType
	ProposalID types.ProposalID
	Proposal   *types.Proposal
	Timestamp  time.Time
}

// ConsensusEventType defines consensus event types
type ConsensusEventType string

const (
	ConsensusEventProposalCreated  ConsensusEventType = "proposal_created"
	ConsensusEventProposalAccepted ConsensusEventType = "proposal_accepted"
	ConsensusEventProposalRejected ConsensusEventType = "proposal_rejected"
	ConsensusEventProposalExpired  ConsensusEventType = "proposal_expired"
	ConsensusEventVoteReceived     ConsensusEventType = "vote_received"
	ConsensusEventQuorumReached    ConsensusEventType = "quorum_reached"
)

// NewBeeConsensus creates a new bee consensus manager
func NewBeeConsensus(config *types.Config, logger *zap.Logger) *BeeConsensus {
	return &BeeConsensus{
		proposals: make(map[types.ProposalID]*types.Proposal),
		agents:    make(map[types.AgentID]bool),
		config:    config,
		logger:    logger,
		eventChan: make(chan ConsensusEvent, 100),
		stopCh:    make(chan struct{}),
	}
}

// Start begins the consensus engine
func (bc *BeeConsensus) Start(ctx context.Context) error {
	bc.logger.Info("Starting Bee consensus engine",
		zap.Float64("quorum_threshold", bc.config.QuorumThreshold),
		zap.Duration("proposal_timeout", bc.config.ProposalTimeout),
	)

	// Start proposal expiration checker
	bc.wg.Add(1)
	go bc.runExpirationLoop(ctx)

	return nil
}

// Stop stops the consensus engine
func (bc *BeeConsensus) Stop() error {
	close(bc.stopCh)
	bc.wg.Wait()
	close(bc.eventChan)
	bc.logger.Info("Bee consensus engine stopped")
	return nil
}

// RegisterAgent registers an agent for consensus participation
func (bc *BeeConsensus) RegisterAgent(agentID types.AgentID) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.agents[agentID] = true
}

// UnregisterAgent removes an agent from consensus participation
func (bc *BeeConsensus) UnregisterAgent(agentID types.AgentID) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	delete(bc.agents, agentID)
}

// GetAgentCount returns the number of active agents
func (bc *BeeConsensus) GetAgentCount() int {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return len(bc.agents)
}

// CreateProposal creates a new consensus proposal with waggle dance
func (bc *BeeConsensus) CreateProposal(proposerID types.AgentID, proposalType types.ProposalType, content map[string]any) (*types.Proposal, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	proposal := &types.Proposal{
		ID:         types.NewProposalID(),
		ProposerID: proposerID,
		Type:       proposalType,
		Content:    content,
		Waggle:     GenerateWaggleDance(content),
		Votes:      make(map[types.AgentID]types.Vote),
		Status:     types.ProposalStatusPending,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(bc.config.ProposalTimeout),
	}

	bc.proposals[proposal.ID] = proposal

	bc.emitEvent(ConsensusEvent{
		Type:       ConsensusEventProposalCreated,
		ProposalID: proposal.ID,
		Proposal:   proposal,
		Timestamp:  time.Now(),
	})

	bc.logger.Info("Proposal created",
		zap.String("proposal_id", string(proposal.ID)),
		zap.String("proposer_id", string(proposerID)),
		zap.String("type", string(proposalType)),
		zap.Float64("waggle_intensity", proposal.Waggle.Intensity),
	)

	return proposal, nil
}

// Vote submits a vote for a proposal
func (bc *BeeConsensus) Vote(proposalID types.ProposalID, voterID types.AgentID, support bool, intensity float64) error {
	bc.mu.RLock()
	proposal, exists := bc.proposals[proposalID]
	bc.mu.RUnlock()

	if !exists {
		return fmt.Errorf("proposal %s not found", proposalID)
	}

	if proposal.Status != types.ProposalStatusPending {
		return fmt.Errorf("proposal %s is not pending (status: %s)", proposalID, proposal.Status)
	}

	vote := types.Vote{
		VoterID:   voterID,
		Support:   support,
		Intensity: intensity,
		Timestamp: time.Now(),
	}

	proposal.AddVote(vote)

	bc.emitEvent(ConsensusEvent{
		Type:       ConsensusEventVoteReceived,
		ProposalID: proposalID,
		Proposal:   proposal,
		Timestamp:  time.Now(),
	})

	// Check if quorum reached
	quorum := proposal.GetQuorum(bc.GetAgentCount())
	if quorum >= bc.config.QuorumThreshold {
		bc.finalizeProposal(proposal, types.ProposalStatusAccepted)
	}

	bc.logger.Debug("Vote received",
		zap.String("proposal_id", string(proposalID)),
		zap.String("voter_id", string(voterID)),
		zap.Bool("support", support),
		zap.Float64("quorum", quorum),
	)

	return nil
}

// GetProposal retrieves a proposal by ID
func (bc *BeeConsensus) GetProposal(proposalID types.ProposalID) (*types.Proposal, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	proposal, exists := bc.proposals[proposalID]
	if !exists {
		return nil, fmt.Errorf("proposal %s not found", proposalID)
	}
	return proposal, nil
}

// GetPendingProposals returns all pending proposals
func (bc *BeeConsensus) GetPendingProposals() []*types.Proposal {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	pending := []*types.Proposal{}
	for _, proposal := range bc.proposals {
		if proposal.Status == types.ProposalStatusPending {
			pending = append(pending, proposal)
		}
	}
	return pending
}

// finalizeProposal finalizes a proposal with the given status
func (bc *BeeConsensus) finalizeProposal(proposal *types.Proposal, status types.ProposalStatus) {
	bc.mu.Lock()
	proposal.Status = status
	bc.mu.Unlock()

	eventType := ConsensusEventProposalAccepted
	if status == types.ProposalStatusRejected {
		eventType = ConsensusEventProposalRejected
	} else if status == types.ProposalStatusExpired {
		eventType = ConsensusEventProposalExpired
	}

	bc.emitEvent(ConsensusEvent{
		Type:       eventType,
		ProposalID: proposal.ID,
		Proposal:   proposal,
		Timestamp:  time.Now(),
	})

	if status == types.ProposalStatusAccepted {
		bc.emitEvent(ConsensusEvent{
			Type:       ConsensusEventQuorumReached,
			ProposalID: proposal.ID,
			Proposal:   proposal,
			Timestamp:  time.Now(),
		})
	}

	bc.logger.Info("Proposal finalized",
		zap.String("proposal_id", string(proposal.ID)),
		zap.String("status", string(status)),
		zap.Int("votes", len(proposal.Votes)),
	)
}

// runExpirationLoop periodically checks for expired proposals
func (bc *BeeConsensus) runExpirationLoop(ctx context.Context) {
	defer bc.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-bc.stopCh:
			return
		case <-ticker.C:
			bc.checkExpiredProposals()
		}
	}
}

// checkExpiredProposals checks and expires pending proposals that have timed out
func (bc *BeeConsensus) checkExpiredProposals() {
	bc.mu.RLock()
	expiredProposals := []*types.Proposal{}
	now := time.Now()

	for _, proposal := range bc.proposals {
		if proposal.Status == types.ProposalStatusPending && now.After(proposal.ExpiresAt) {
			expiredProposals = append(expiredProposals, proposal)
		}
	}
	bc.mu.RUnlock()

	for _, proposal := range expiredProposals {
		bc.finalizeProposal(proposal, types.ProposalStatusExpired)
	}
}

// EventChannel returns the channel for consensus events
func (bc *BeeConsensus) EventChannel() <-chan ConsensusEvent {
	return bc.eventChan
}

// emitEvent sends a consensus event to the event channel
func (bc *BeeConsensus) emitEvent(event ConsensusEvent) {
	select {
	case bc.eventChan <- event:
	default:
		bc.logger.Warn("Consensus event channel full, dropping event",
			zap.String("event_type", string(event.Type)),
		)
	}
}

// GetStats returns consensus statistics
func (bc *BeeConsensus) GetStats() map[string]int {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	stats := map[string]int{
		"total_proposals":    len(bc.proposals),
		"pending_proposals":  0,
		"accepted_proposals": 0,
		"rejected_proposals": 0,
		"expired_proposals":  0,
		"active_agents":      len(bc.agents),
	}

	for _, proposal := range bc.proposals {
		switch proposal.Status {
		case types.ProposalStatusPending:
			stats["pending_proposals"]++
		case types.ProposalStatusAccepted:
			stats["accepted_proposals"]++
		case types.ProposalStatusRejected:
			stats["rejected_proposals"]++
		case types.ProposalStatusExpired:
			stats["expired_proposals"]++
		}
	}

	return stats
}
