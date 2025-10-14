package topology

import (
	"context"
	"sync"
	"time"

	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
	"go.uber.org/zap"
)

// SlimeMoldTopology implements the slime mold-inspired network optimization
type SlimeMoldTopology struct {
	graph     *Graph
	config    *types.Config
	logger    *zap.Logger
	eventChan chan types.TopologyEvent

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewSlimeMoldTopology creates a new slime mold topology manager
func NewSlimeMoldTopology(config *types.Config, logger *zap.Logger) *SlimeMoldTopology {
	return &SlimeMoldTopology{
		graph:     NewGraph(config),
		config:    config,
		logger:    logger,
		eventChan: make(chan types.TopologyEvent, 500), // Increased from 100 to 500 to handle mass pruning
		stopCh:    make(chan struct{}),
	}
}

// Start begins the topology evolution process
func (sm *SlimeMoldTopology) Start(ctx context.Context) error {
	sm.logger.Info("Starting SlimeMold topology optimization",
		zap.Float64("decay_rate", sm.config.DecayRate),
		zap.Duration("decay_interval", sm.config.DecayInterval),
		zap.Float64("prune_threshold", sm.config.PruneThreshold),
	)

	// Start decay ticker
	sm.wg.Add(1)
	go sm.runDecayLoop(ctx)

	return nil
}

// Stop stops the topology manager
func (sm *SlimeMoldTopology) Stop() error {
	close(sm.stopCh)
	sm.wg.Wait()
	close(sm.eventChan)
	sm.logger.Info("SlimeMold topology optimization stopped")
	return nil
}

// runDecayLoop periodically decays all edges and prunes weak ones
func (sm *SlimeMoldTopology) runDecayLoop(ctx context.Context) {
	defer sm.wg.Done()

	ticker := time.NewTicker(sm.config.DecayInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sm.stopCh:
			return
		case <-ticker.C:
			sm.applyDecayAndPrune()
		}
	}
}

// applyDecayAndPrune applies decay to all edges and prunes weak ones
func (sm *SlimeMoldTopology) applyDecayAndPrune() {
	// Apply decay to all edges
	sm.graph.DecayAllEdges()

	// Prune weak edges
	prunedEdges := sm.graph.PruneWeakEdges()

	// Emit events for pruned edges
	for _, edgeID := range prunedEdges {
		sm.emitEvent(types.TopologyEvent{
			Type:      types.TopologyEventEdgeRemoved,
			EdgeID:    edgeID,
			Timestamp: time.Now(),
		})
	}

	if len(prunedEdges) > 0 {
		sm.logger.Debug("Pruned weak edges",
			zap.Int("count", len(prunedEdges)),
			zap.Int("remaining_edges", sm.graph.GetEdgeCount()),
		)
	}
}

// AddAgent adds a new agent to the topology
func (sm *SlimeMoldTopology) AddAgent(agent *types.Agent) error {
	if err := sm.graph.AddAgent(agent); err != nil {
		return err
	}

	sm.emitEvent(types.TopologyEvent{
		Type:      types.TopologyEventAgentJoined,
		AgentID:   agent.ID,
		Timestamp: time.Now(),
	})

	sm.logger.Info("Agent joined mesh",
		zap.String("agent_id", string(agent.ID)),
		zap.String("name", agent.Name),
		zap.String("role", agent.Role),
	)

	return nil
}

// RemoveAgent removes an agent from the topology
func (sm *SlimeMoldTopology) RemoveAgent(agentID types.AgentID) error {
	if err := sm.graph.RemoveAgent(agentID); err != nil {
		return err
	}

	sm.emitEvent(types.TopologyEvent{
		Type:      types.TopologyEventAgentLeft,
		AgentID:   agentID,
		Timestamp: time.Now(),
	})

	sm.logger.Info("Agent left mesh",
		zap.String("agent_id", string(agentID)),
	)

	return nil
}

// ReinforceEdge strengthens an edge when a message is sent through it
func (sm *SlimeMoldTopology) ReinforceEdge(sourceID, targetID types.AgentID) error {
	edgeID := types.NewEdgeID(sourceID, targetID)

	if err := sm.graph.ReinforceEdge(edgeID); err != nil {
		return err
	}

	// Get updated edge
	edge, _ := sm.graph.GetEdge(edgeID)
	if edge != nil {
		sm.emitEvent(types.TopologyEvent{
			Type:      types.TopologyEventEdgeStrength,
			EdgeID:    edgeID,
			Edge:      edge,
			Timestamp: time.Now(),
		})
	}

	return nil
}

// GetSnapshot returns the current graph snapshot
func (sm *SlimeMoldTopology) GetSnapshot() *types.GraphSnapshot {
	return sm.graph.GetSnapshot()
}

// GetGraph returns the underlying graph
func (sm *SlimeMoldTopology) GetGraph() *Graph {
	return sm.graph
}

// EventChannel returns the channel for topology events
func (sm *SlimeMoldTopology) EventChannel() <-chan types.TopologyEvent {
	return sm.eventChan
}

// emitEvent sends a topology event to the event channel
func (sm *SlimeMoldTopology) emitEvent(event types.TopologyEvent) {
	select {
	case sm.eventChan <- event:
	default:
		sm.logger.Warn("Topology event channel full, dropping event",
			zap.String("event_type", string(event.Type)),
		)
	}
}

// GetOptimalPath returns the strongest path between two agents (for routing)
func (sm *SlimeMoldTopology) GetOptimalPath(sourceID, targetID types.AgentID) ([]types.AgentID, error) {
	// Simple implementation: direct edge if strong enough, otherwise return empty (no path)
	edge, err := sm.graph.GetEdgeBetween(sourceID, targetID)
	if err == nil && edge.GetWeight() >= sm.config.PruneThreshold {
		return []types.AgentID{sourceID, targetID}, nil
	}

	// For now, return direct path only. In future, implement Dijkstra for multi-hop paths
	return []types.AgentID{sourceID, targetID}, nil
}

// PrintStats logs current topology statistics
func (sm *SlimeMoldTopology) PrintStats() {
	snapshot := sm.GetSnapshot()
	sm.logger.Info("Topology statistics",
		zap.Int("agents", snapshot.Stats.TotalAgents),
		zap.Int("edges", snapshot.Stats.TotalEdges),
		zap.Int("active_edges", snapshot.Stats.ActiveEdges),
		zap.Float64("avg_weight", snapshot.Stats.AverageWeight),
		zap.Float64("density", snapshot.Stats.Density),
		zap.Float64("reduction_percent", snapshot.Stats.ReductionPercent),
	)
}
