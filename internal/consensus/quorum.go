package consensus

import (
	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
)

// QuorumSensor monitors and detects quorum in consensus proposals
type QuorumSensor struct {
	threshold float64 // Quorum threshold (e.g., 0.6 for 60%)
}

// NewQuorumSensor creates a new quorum sensor
func NewQuorumSensor(threshold float64) *QuorumSensor {
	return &QuorumSensor{
		threshold: threshold,
	}
}

// CheckQuorum checks if a proposal has reached quorum
func (qs *QuorumSensor) CheckQuorum(proposal *types.Proposal, totalAgents int) (bool, float64) {
	quorum := proposal.GetQuorum(totalAgents)
	return quorum >= qs.threshold, quorum
}

// CalculateWeightedQuorum calculates quorum with vote intensity weights
// In bee colonies, more enthusiastic dancing influences the swarm more
func (qs *QuorumSensor) CalculateWeightedQuorum(proposal *types.Proposal, totalAgents int) float64 {
	if totalAgents == 0 {
		return 0.0
	}

	var totalWeight float64
	var supportWeight float64

	for _, vote := range proposal.Votes {
		weight := vote.Intensity // Use intensity as weight
		totalWeight += weight

		if vote.Support {
			supportWeight += weight
		}
	}

	if totalWeight == 0 {
		return 0.0
	}

	// Weighted quorum: (sum of supporting votes' intensities) / (sum of all votes' intensities)
	return supportWeight / totalWeight
}

// PredictQuorumTime estimates time to reach quorum based on voting velocity
func (qs *QuorumSensor) PredictQuorumTime(proposal *types.Proposal, totalAgents int) float64 {
	if len(proposal.Votes) == 0 {
		return -1.0 // Cannot predict without votes
	}

	// Calculate voting velocity (votes per second)
	elapsed := proposal.Votes[types.AgentID("")].Timestamp.Sub(proposal.CreatedAt).Seconds()
	if elapsed == 0 {
		return -1.0
	}

	velocity := float64(len(proposal.Votes)) / elapsed

	// Calculate votes needed for quorum
	votesNeeded := int(float64(totalAgents)*qs.threshold) - len(proposal.Votes)

	if votesNeeded <= 0 {
		return 0.0 // Already at quorum
	}

	if velocity == 0 {
		return -1.0
	}

	// Estimated time to quorum (in seconds)
	return float64(votesNeeded) / velocity
}

// GetQuorumStatus returns detailed quorum status
func (qs *QuorumSensor) GetQuorumStatus(proposal *types.Proposal, totalAgents int) QuorumStatus {
	currentQuorum := proposal.GetQuorum(totalAgents)
	reached, _ := qs.CheckQuorum(proposal, totalAgents)

	supportCount := 0
	rejectCount := 0
	avgIntensity := 0.0

	for _, vote := range proposal.Votes {
		if vote.Support {
			supportCount++
		} else {
			rejectCount++
		}
		avgIntensity += vote.Intensity
	}

	if len(proposal.Votes) > 0 {
		avgIntensity /= float64(len(proposal.Votes))
	}

	return QuorumStatus{
		Reached:          reached,
		CurrentQuorum:    currentQuorum,
		RequiredQuorum:   qs.threshold,
		SupportCount:     supportCount,
		RejectCount:      rejectCount,
		TotalVotes:       len(proposal.Votes),
		TotalAgents:      totalAgents,
		AverageIntensity: avgIntensity,
	}
}

// QuorumStatus contains detailed information about quorum state
type QuorumStatus struct {
	Reached          bool    `json:"reached"`
	CurrentQuorum    float64 `json:"current_quorum"`
	RequiredQuorum   float64 `json:"required_quorum"`
	SupportCount     int     `json:"support_count"`
	RejectCount      int     `json:"reject_count"`
	TotalVotes       int     `json:"total_votes"`
	TotalAgents      int     `json:"total_agents"`
	AverageIntensity float64 `json:"average_intensity"`
}

// IsStrongQuorum checks if quorum is reached with high intensity votes
func (qs *QuorumSensor) IsStrongQuorum(proposal *types.Proposal, totalAgents int, minIntensity float64) bool {
	currentQuorum := proposal.GetQuorum(totalAgents)
	if currentQuorum < qs.threshold {
		return false
	}

	// Check if supporting votes have high enough intensity
	strongVotes := 0
	for _, vote := range proposal.Votes {
		if vote.Support && vote.Intensity >= minIntensity {
			strongVotes++
		}
	}

	strongQuorum := float64(strongVotes) / float64(totalAgents)
	return strongQuorum >= qs.threshold
}

// DetectConsensusPattern analyzes voting patterns to detect emerging consensus
func (qs *QuorumSensor) DetectConsensusPattern(proposal *types.Proposal) ConsensusPattern {
	if len(proposal.Votes) == 0 {
		return ConsensusPatternUnknown
	}

	supportCount := 0
	rejectCount := 0
	avgSupportIntensity := 0.0
	avgRejectIntensity := 0.0

	for _, vote := range proposal.Votes {
		if vote.Support {
			supportCount++
			avgSupportIntensity += vote.Intensity
		} else {
			rejectCount++
			avgRejectIntensity += vote.Intensity
		}
	}

	if supportCount > 0 {
		avgSupportIntensity /= float64(supportCount)
	}
	if rejectCount > 0 {
		avgRejectIntensity /= float64(rejectCount)
	}

	// Determine pattern
	if supportCount > rejectCount*3 {
		if avgSupportIntensity > 0.7 {
			return ConsensusPatternStrongSupport
		}
		return ConsensusPatternSupport
	} else if rejectCount > supportCount*3 {
		if avgRejectIntensity > 0.7 {
			return ConsensusPatternStrongOpposition
		}
		return ConsensusPatternOpposition
	} else if supportCount == rejectCount {
		return ConsensusPatternDeadlock
	} else {
		return ConsensusPatternDivided
	}
}

// ConsensusPattern represents the voting pattern
type ConsensusPattern string

const (
	ConsensusPatternStrongSupport    ConsensusPattern = "strong_support"
	ConsensusPatternSupport          ConsensusPattern = "support"
	ConsensusPatternDivided          ConsensusPattern = "divided"
	ConsensusPatternOpposition       ConsensusPattern = "opposition"
	ConsensusPatternStrongOpposition ConsensusPattern = "strong_opposition"
	ConsensusPatternDeadlock         ConsensusPattern = "deadlock"
	ConsensusPatternUnknown          ConsensusPattern = "unknown"
)
