package test

import (
	"testing"
	"time"

	"github.com/avinashshinde/agentmesh-cortex/internal/topology"
	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
	"go.uber.org/zap"
)

func TestGraphAddAgent(t *testing.T) {
	config := &types.Config{
		InitialEdgeWeight: 0.5,
	}
	graph := topology.NewGraph(config)

	agent1 := &types.Agent{
		ID:        types.NewAgentID(),
		Name:      "Agent1",
		Role:      "test",
		Status:    types.AgentStatusActive,
		CreatedAt: time.Now(),
	}

	// Test adding first agent
	err := graph.AddAgent(agent1)
	if err != nil {
		t.Fatalf("Failed to add agent: %v", err)
	}

	if graph.GetAgentCount() != 1 {
		t.Errorf("Expected 1 agent, got %d", graph.GetAgentCount())
	}

	// Test duplicate agent
	err = graph.AddAgent(agent1)
	if err == nil {
		t.Error("Expected error when adding duplicate agent")
	}
}

func TestGraphFullMeshCreation(t *testing.T) {
	config := &types.Config{
		InitialEdgeWeight: 0.5,
	}
	graph := topology.NewGraph(config)

	// Add 4 agents
	agents := make([]*types.Agent, 4)
	for i := 0; i < 4; i++ {
		agents[i] = &types.Agent{
			ID:        types.NewAgentID(),
			Name:      "Agent" + string(rune('A'+i)),
			Role:      "test",
			Status:    types.AgentStatusActive,
			CreatedAt: time.Now(),
		}
		graph.AddAgent(agents[i])
	}

	// Full mesh for 4 agents = 4 * 3 = 12 bidirectional edges
	expectedEdges := 4 * 3
	if graph.GetEdgeCount() != expectedEdges {
		t.Errorf("Expected %d edges (full mesh), got %d", expectedEdges, graph.GetEdgeCount())
	}

	// Verify initial weights
	snapshot := graph.GetSnapshot()
	for _, edge := range snapshot.Edges {
		if edge.Weight != 0.5 {
			t.Errorf("Expected initial weight 0.5, got %f", edge.Weight)
		}
	}
}

func TestEdgeReinforcement(t *testing.T) {
	config := &types.Config{
		InitialEdgeWeight:   0.5,
		ReinforcementAmount: 0.1,
	}
	graph := topology.NewGraph(config)

	agent1 := &types.Agent{ID: types.NewAgentID(), Name: "A1", Role: "test", Status: types.AgentStatusActive, CreatedAt: time.Now()}
	agent2 := &types.Agent{ID: types.NewAgentID(), Name: "A2", Role: "test", Status: types.AgentStatusActive, CreatedAt: time.Now()}

	graph.AddAgent(agent1)
	graph.AddAgent(agent2)

	edgeID := types.NewEdgeID(agent1.ID, agent2.ID)

	// Initial weight
	edge, _ := graph.GetEdge(edgeID)
	initialWeight := edge.GetWeight()
	if initialWeight != 0.5 {
		t.Errorf("Expected initial weight 0.5, got %f", initialWeight)
	}

	// Reinforce edge
	graph.ReinforceEdge(edgeID)
	edge, _ = graph.GetEdge(edgeID)
	newWeight := edge.GetWeight()

	expectedWeight := 0.5 + 0.1
	if newWeight != expectedWeight {
		t.Errorf("Expected weight %f after reinforcement, got %f", expectedWeight, newWeight)
	}

	// Test saturation at 1.0
	for i := 0; i < 10; i++ {
		graph.ReinforceEdge(edgeID)
	}
	edge, _ = graph.GetEdge(edgeID)
	if edge.GetWeight() > 1.0 {
		t.Errorf("Weight exceeded maximum 1.0, got %f", edge.GetWeight())
	}
}

func TestEdgeDecay(t *testing.T) {
	config := &types.Config{
		InitialEdgeWeight: 0.5,
		DecayRate:         0.1,
	}
	graph := topology.NewGraph(config)

	agent1 := &types.Agent{ID: types.NewAgentID(), Name: "A1", Role: "test", Status: types.AgentStatusActive, CreatedAt: time.Now()}
	agent2 := &types.Agent{ID: types.NewAgentID(), Name: "A2", Role: "test", Status: types.AgentStatusActive, CreatedAt: time.Now()}

	graph.AddAgent(agent1)
	graph.AddAgent(agent2)

	edgeID := types.NewEdgeID(agent1.ID, agent2.ID)
	edge, _ := graph.GetEdge(edgeID)
	initialWeight := edge.GetWeight()

	// Apply decay
	graph.DecayAllEdges()

	edge, _ = graph.GetEdge(edgeID)
	newWeight := edge.GetWeight()

	expectedWeight := initialWeight - 0.1
	if newWeight != expectedWeight {
		t.Errorf("Expected weight %f after decay, got %f", expectedWeight, newWeight)
	}

	// Test floor at 0.0
	for i := 0; i < 10; i++ {
		graph.DecayAllEdges()
	}
	edge, _ = graph.GetEdge(edgeID)
	if edge.GetWeight() < 0.0 {
		t.Errorf("Weight went below minimum 0.0, got %f", edge.GetWeight())
	}
}

func TestEdgePruning(t *testing.T) {
	config := &types.Config{
		InitialEdgeWeight: 0.15,
		PruneThreshold:    0.1,
		DecayRate:         0.1,
	}
	graph := topology.NewGraph(config)

	agent1 := &types.Agent{ID: types.NewAgentID(), Name: "A1", Role: "test", Status: types.AgentStatusActive, CreatedAt: time.Now()}
	agent2 := &types.Agent{ID: types.NewAgentID(), Name: "A2", Role: "test", Status: types.AgentStatusActive, CreatedAt: time.Now()}

	graph.AddAgent(agent1)
	graph.AddAgent(agent2)

	initialEdgeCount := graph.GetEdgeCount()

	// Decay twice: 0.15 -> 0.05 (below threshold)
	graph.DecayAllEdges()
	graph.DecayAllEdges()

	// Prune weak edges
	prunedEdges := graph.PruneWeakEdges()

	if len(prunedEdges) != initialEdgeCount {
		t.Errorf("Expected %d edges pruned, got %d", initialEdgeCount, len(prunedEdges))
	}

	if graph.GetEdgeCount() != 0 {
		t.Errorf("Expected 0 edges after pruning, got %d", graph.GetEdgeCount())
	}
}

func TestSlimeMoldTopology(t *testing.T) {
	config := &types.Config{
		InitialEdgeWeight:   0.5,
		ReinforcementAmount: 0.2,
		DecayRate:           0.05,
		DecayInterval:       100 * time.Millisecond,
		PruneThreshold:      0.1,
	}

	logger, _ := zap.NewDevelopment()
	sm := topology.NewSlimeMoldTopology(config, logger)

	// Add agents
	agent1 := &types.Agent{ID: types.NewAgentID(), Name: "A1", Role: "test", Status: types.AgentStatusActive, CreatedAt: time.Now()}
	agent2 := &types.Agent{ID: types.NewAgentID(), Name: "A2", Role: "test", Status: types.AgentStatusActive, CreatedAt: time.Now()}

	sm.AddAgent(agent1)
	sm.AddAgent(agent2)

	// Verify full mesh created
	snapshot := sm.GetSnapshot()
	if snapshot.Stats.TotalEdges != 2 {
		t.Errorf("Expected 2 edges, got %d", snapshot.Stats.TotalEdges)
	}

	// Reinforce one edge multiple times
	for i := 0; i < 5; i++ {
		sm.ReinforceEdge(agent1.ID, agent2.ID)
	}

	// Check reinforcement worked
	edge, _ := sm.GetGraph().GetEdgeBetween(agent1.ID, agent2.ID)
	if edge.GetWeight() <= 0.5 {
		t.Errorf("Expected weight > 0.5 after reinforcement, got %f", edge.GetWeight())
	}
}

func TestTopologyEvolution(t *testing.T) {
	config := &types.Config{
		InitialEdgeWeight:   0.3,
		ReinforcementAmount: 0.2,
		DecayRate:           0.1,
		PruneThreshold:      0.15,
	}

	logger, _ := zap.NewDevelopment()
	sm := topology.NewSlimeMoldTopology(config, logger)

	// Create 4 agents (12 edges full mesh)
	agents := make([]*types.Agent, 4)
	for i := 0; i < 4; i++ {
		agents[i] = &types.Agent{
			ID:        types.NewAgentID(),
			Name:      "Agent" + string(rune('A'+i)),
			Role:      "test",
			Status:    types.AgentStatusActive,
			CreatedAt: time.Now(),
		}
		sm.AddAgent(agents[i])
	}

	initialSnapshot := sm.GetSnapshot()
	if initialSnapshot.Stats.TotalEdges != 12 {
		t.Errorf("Expected 12 initial edges, got %d", initialSnapshot.Stats.TotalEdges)
	}

	// Simulate high-frequency communication between A0 and A1
	for i := 0; i < 20; i++ {
		sm.ReinforceEdge(agents[0].ID, agents[1].ID)
		sm.ReinforceEdge(agents[1].ID, agents[0].ID)
	}

	// Apply decay to all edges
	for i := 0; i < 3; i++ {
		sm.GetGraph().DecayAllEdges()
	}

	// Prune weak edges
	sm.GetGraph().PruneWeakEdges()

	finalSnapshot := sm.GetSnapshot()

	// Should have fewer edges now
	if finalSnapshot.Stats.TotalEdges >= initialSnapshot.Stats.TotalEdges {
		t.Errorf("Expected topology to evolve (fewer edges), got %d edges", finalSnapshot.Stats.TotalEdges)
	}

	// The reinforced edges should still exist
	edge01, err := sm.GetGraph().GetEdgeBetween(agents[0].ID, agents[1].ID)
	if err != nil {
		t.Error("Expected reinforced edge A0->A1 to survive pruning")
	}
	if edge01.GetWeight() < config.PruneThreshold {
		t.Errorf("Expected reinforced edge weight >= %f, got %f", config.PruneThreshold, edge01.GetWeight())
	}
}

func TestGraphStatistics(t *testing.T) {
	config := &types.Config{
		InitialEdgeWeight: 0.5,
	}
	graph := topology.NewGraph(config)

	// Add 4 agents
	for i := 0; i < 4; i++ {
		agent := &types.Agent{
			ID:        types.NewAgentID(),
			Name:      "Agent" + string(rune('A'+i)),
			Role:      "test",
			Status:    types.AgentStatusActive,
			CreatedAt: time.Now(),
		}
		graph.AddAgent(agent)
	}

	snapshot := graph.GetSnapshot()
	stats := snapshot.Stats

	// Test statistics
	if stats.TotalAgents != 4 {
		t.Errorf("Expected 4 agents, got %d", stats.TotalAgents)
	}

	if stats.TotalEdges != 12 {
		t.Errorf("Expected 12 edges, got %d", stats.TotalEdges)
	}

	if stats.Density != 1.0 {
		t.Errorf("Expected density 1.0 (full mesh), got %f", stats.Density)
	}

	if stats.ReductionPercent != 0.0 {
		t.Errorf("Expected 0%% reduction (full mesh), got %f", stats.ReductionPercent)
	}

	if stats.AverageWeight != 0.5 {
		t.Errorf("Expected average weight 0.5, got %f", stats.AverageWeight)
	}
}
