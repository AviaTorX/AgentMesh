package metrics

import (
	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
)

// Reporter updates Prometheus metrics from system state
type Reporter struct {
	collector *Collector
}

// NewReporter creates a new metrics reporter
func NewReporter(collector *Collector) *Reporter {
	return &Reporter{collector: collector}
}

// UpdateTopologyMetrics updates topology-related metrics
func (r *Reporter) UpdateTopologyMetrics(snapshot *types.GraphSnapshot) {
	r.collector.EdgeCount.Set(float64(snapshot.Stats.TotalEdges))
	r.collector.ActiveEdgeCount.Set(float64(snapshot.Stats.ActiveEdges))
	r.collector.AgentCount.Set(float64(snapshot.Stats.TotalAgents))
	r.collector.TopologyDensity.Set(snapshot.Stats.Density)
	r.collector.ReductionPercent.Set(snapshot.Stats.ReductionPercent)
	for _, edge := range snapshot.Edges {
		r.collector.EdgeWeight.Observe(edge.GetWeight())
	}
}

// RecordProposal records a proposal status change
func (r *Reporter) RecordProposal(status types.ProposalStatus) {
	r.collector.ProposalCount.WithLabelValues(string(status)).Inc()
}

// RecordVote records a vote cast
func (r *Reporter) RecordVote() {
	r.collector.VoteCount.Inc()
}

// RecordQuorum records that quorum was reached
func (r *Reporter) RecordQuorum() {
	r.collector.QuorumReached.Inc()
}

// RecordProposalDuration records how long a proposal took
func (r *Reporter) RecordProposalDuration(seconds float64) {
	r.collector.ProposalDuration.Observe(seconds)
}

// RecordMessageSent records a message sent
func (r *Reporter) RecordMessageSent(msgType types.MessageType) {
	r.collector.MessagesSent.WithLabelValues(string(msgType)).Inc()
}

// RecordEdgeReinforcement records an edge reinforcement
func (r *Reporter) RecordEdgeReinforcement() {
	r.collector.EdgeReinforcements.Inc()
}

// RecordEdgePruned records an edge being pruned
func (r *Reporter) RecordEdgePruned() {
	r.collector.EdgePruned.Inc()
}
