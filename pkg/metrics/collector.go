package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Collector holds all Prometheus metrics
type Collector struct {
	EdgeCount       prometheus.Gauge
	ActiveEdgeCount prometheus.Gauge
	AgentCount      prometheus.Gauge
	EdgeWeight      prometheus.Histogram
	TopologyDensity prometheus.Gauge
	ReductionPercent prometheus.Gauge
	ProposalCount    *prometheus.CounterVec
	VoteCount        prometheus.Counter
	QuorumReached    prometheus.Counter
	ProposalDuration prometheus.Histogram
	MessagesSent     *prometheus.CounterVec
	MessagesReceived *prometheus.CounterVec
	MessageLatency   prometheus.Histogram
	EdgeReinforcements prometheus.Counter
	EdgePruned         prometheus.Counter
}

// NewCollector creates a new metrics collector with Prometheus metrics
func NewCollector() *Collector {
	return &Collector{
		EdgeCount: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "agentmesh_edge_count",
			Help: "Current number of edges in the topology",
		}),
		ActiveEdgeCount: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "agentmesh_active_edge_count",
			Help: "Number of edges with weight > 0.1",
		}),
		AgentCount: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "agentmesh_agent_count",
			Help: "Current number of agents in the mesh",
		}),
		EdgeWeight: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "agentmesh_edge_weight",
			Help:    "Distribution of edge weights",
			Buckets: prometheus.LinearBuckets(0, 0.1, 11),
		}),
		TopologyDensity: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "agentmesh_topology_density",
			Help: "Network density",
		}),
		ReductionPercent: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "agentmesh_reduction_percent",
			Help: "Percentage reduction from full mesh",
		}),
		ProposalCount: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "agentmesh_proposal_count",
				Help: "Number of proposals by status",
			},
			[]string{"status"},
		),
		VoteCount: promauto.NewCounter(prometheus.CounterOpts{
			Name: "agentmesh_vote_count",
			Help: "Total number of votes cast",
		}),
		QuorumReached: promauto.NewCounter(prometheus.CounterOpts{
			Name: "agentmesh_quorum_reached_count",
			Help: "Number of times quorum was reached",
		}),
		ProposalDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "agentmesh_proposal_duration_seconds",
			Help:    "Time from proposal creation to finalization",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
		}),
		MessagesSent: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "agentmesh_messages_sent_total",
				Help: "Total messages sent by type",
			},
			[]string{"type"},
		),
		MessagesReceived: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "agentmesh_messages_received_total",
				Help: "Total messages received by type",
			},
			[]string{"type"},
		),
		MessageLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "agentmesh_message_latency_seconds",
			Help:    "Message processing latency",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		}),
		EdgeReinforcements: promauto.NewCounter(prometheus.CounterOpts{
			Name: "agentmesh_edge_reinforcements_total",
			Help: "Total edge reinforcements",
		}),
		EdgePruned: promauto.NewCounter(prometheus.CounterOpts{
			Name: "agentmesh_edge_pruned_total",
			Help: "Total edges pruned",
		}),
	}
}
