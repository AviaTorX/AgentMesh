package topology

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
)

// Graph represents the agent communication network
type Graph struct {
	agents map[types.AgentID]*types.Agent
	edges  map[types.EdgeID]*types.Edge
	config *types.Config

	mu sync.RWMutex
}

// NewGraph creates a new graph with full mesh topology
func NewGraph(config *types.Config) *Graph {
	return &Graph{
		agents: make(map[types.AgentID]*types.Agent),
		edges:  make(map[types.EdgeID]*types.Edge),
		config: config,
	}
}

// AddAgent adds a new agent to the graph and creates edges to all existing agents (full mesh)
func (g *Graph) AddAgent(agent *types.Agent) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.agents[agent.ID]; exists {
		return fmt.Errorf("agent %s already exists", agent.ID)
	}

	g.agents[agent.ID] = agent

	// Create self-loop edge for the agent (to track its own activity)
	selfEdge := &types.Edge{
		ID:        types.NewEdgeID(agent.ID, agent.ID),
		SourceID:  agent.ID,
		TargetID:  agent.ID,
		Weight:    g.config.InitialEdgeWeight,
		Usage:     0,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
	}
	g.edges[selfEdge.ID] = selfEdge

	// Create bidirectional edges to all existing agents (full mesh initialization)
	for _, existingAgent := range g.agents {
		if existingAgent.ID == agent.ID {
			continue
		}

		// Edge from new agent to existing agent
		edge1 := &types.Edge{
			ID:        types.NewEdgeID(agent.ID, existingAgent.ID),
			SourceID:  agent.ID,
			TargetID:  existingAgent.ID,
			Weight:    g.config.InitialEdgeWeight,
			Usage:     0,
			CreatedAt: time.Now(),
			LastUsed:  time.Now(),
		}
		g.edges[edge1.ID] = edge1

		// Edge from existing agent to new agent
		edge2 := &types.Edge{
			ID:        types.NewEdgeID(existingAgent.ID, agent.ID),
			SourceID:  existingAgent.ID,
			TargetID:  agent.ID,
			Weight:    g.config.InitialEdgeWeight,
			Usage:     0,
			CreatedAt: time.Now(),
			LastUsed:  time.Now(),
		}
		g.edges[edge2.ID] = edge2
	}

	return nil
}

// RemoveAgent removes an agent and all its edges
func (g *Graph) RemoveAgent(agentID types.AgentID) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.agents[agentID]; !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	// Remove all edges connected to this agent
	edgesToRemove := []types.EdgeID{}
	for edgeID, edge := range g.edges {
		if edge.SourceID == agentID || edge.TargetID == agentID {
			edgesToRemove = append(edgesToRemove, edgeID)
		}
	}

	for _, edgeID := range edgesToRemove {
		delete(g.edges, edgeID)
	}

	delete(g.agents, agentID)
	return nil
}

// GetEdge retrieves an edge by ID
func (g *Graph) GetEdge(edgeID types.EdgeID) (*types.Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	edge, exists := g.edges[edgeID]
	if !exists {
		return nil, fmt.Errorf("edge %s not found", edgeID)
	}
	return edge, nil
}

// GetEdgeBetween retrieves the edge between two agents
func (g *Graph) GetEdgeBetween(sourceID, targetID types.AgentID) (*types.Edge, error) {
	edgeID := types.NewEdgeID(sourceID, targetID)
	return g.GetEdge(edgeID)
}

// ReinforceEdge strengthens an edge (called when message passes through it)
// If edge doesn't exist, it creates it first (SlimeMold behavior: paths form on first use)
func (g *Graph) ReinforceEdge(edgeID types.EdgeID) error {
	g.mu.Lock()
	edge, exists := g.edges[edgeID]

	if !exists {
		// Parse edge ID to get source and target
		// EdgeID format: "sourceID->targetID"
		parts := strings.Split(string(edgeID), "->")
		if len(parts) != 2 {
			g.mu.Unlock()
			return fmt.Errorf("invalid edge ID format: %s", edgeID)
		}

		sourceID := types.AgentID(parts[0])
		targetID := types.AgentID(parts[1])

		// Verify both agents exist
		if _, exists := g.agents[sourceID]; !exists {
			g.mu.Unlock()
			return fmt.Errorf("source agent %s not found", sourceID)
		}
		if _, exists := g.agents[targetID]; !exists {
			g.mu.Unlock()
			return fmt.Errorf("target agent %s not found", targetID)
		}

		// Create new edge with initial weight (0.5 - moderate strength)
		edge = &types.Edge{
			ID:        edgeID,
			SourceID:  sourceID,
			TargetID:  targetID,
			Weight:    0.5, // Initial weight for new paths
			Usage:     0,
			LastUsed:  time.Now(),
			CreatedAt: time.Now(),
		}
		g.edges[edgeID] = edge
	}
	g.mu.Unlock()

	// Reinforce the edge (whether newly created or existing)
	edge.Reinforce(g.config.ReinforcementAmount)
	return nil
}

// DecayAllEdges applies decay to all edges (simulates pheromone evaporation)
func (g *Graph) DecayAllEdges() {
	g.mu.RLock()
	edges := make([]*types.Edge, 0, len(g.edges))
	for _, edge := range g.edges {
		edges = append(edges, edge)
	}
	g.mu.RUnlock()

	for _, edge := range edges {
		edge.Decay(g.config.DecayRate)
	}
}

// PruneWeakEdges removes edges below the prune threshold
func (g *Graph) PruneWeakEdges() []types.EdgeID {
	g.mu.Lock()
	defer g.mu.Unlock()

	prunedEdges := []types.EdgeID{}

	for edgeID, edge := range g.edges {
		if edge.GetWeight() < g.config.PruneThreshold {
			prunedEdges = append(prunedEdges, edgeID)
			delete(g.edges, edgeID)
		}
	}

	return prunedEdges
}

// GetSnapshot returns a snapshot of the current graph state
func (g *Graph) GetSnapshot() *types.GraphSnapshot {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Deep copy agents and edges
	agentsCopy := make(map[types.AgentID]*types.Agent)
	for id, agent := range g.agents {
		agentCopy := *agent
		agentsCopy[id] = &agentCopy
	}

	edgesCopy := make(map[types.EdgeID]*types.Edge)
	for id, edge := range g.edges {
		edgeCopy := *edge
		edgesCopy[id] = &edgeCopy
	}

	stats := g.calculateStats()

	return &types.GraphSnapshot{
		Agents:    agentsCopy,
		Edges:     edgesCopy,
		Timestamp: time.Now(),
		Stats:     stats,
	}
}

// calculateStats computes graph statistics (must be called with read lock held)
func (g *Graph) calculateStats() types.GraphStats {
	numAgents := len(g.agents)
	numEdges := len(g.edges)

	if numEdges == 0 {
		return types.GraphStats{
			TotalAgents: numAgents,
			TotalEdges:  0,
		}
	}

	var totalWeight, maxWeight, minWeight float64
	activeEdges := 0
	minWeight = 1.0 // Initialize to max possible weight

	for _, edge := range g.edges {
		weight := edge.GetWeight()
		totalWeight += weight

		if weight > maxWeight {
			maxWeight = weight
		}
		if weight < minWeight {
			minWeight = weight
		}
		if weight > 0.1 {
			activeEdges++
		}
	}

	avgWeight := totalWeight / float64(numEdges)

	// Calculate density (actual edges / possible edges in full mesh)
	// In a directed full mesh, possible edges = n * (n - 1)
	possibleEdges := numAgents * (numAgents - 1)
	density := 0.0
	if possibleEdges > 0 {
		density = float64(numEdges) / float64(possibleEdges)
	}

	// Calculate reduction percentage from full mesh
	reductionPercent := 0.0
	if possibleEdges > 0 {
		reductionPercent = (1.0 - density) * 100.0
	}

	return types.GraphStats{
		TotalAgents:      numAgents,
		TotalEdges:       numEdges,
		ActiveEdges:      activeEdges,
		AverageWeight:    avgWeight,
		MaxWeight:        maxWeight,
		MinWeight:        minWeight,
		Density:          density,
		ReductionPercent: reductionPercent,
	}
}

// GetAgentCount returns the number of agents
func (g *Graph) GetAgentCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.agents)
}

// GetEdgeCount returns the number of edges
func (g *Graph) GetEdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.edges)
}

// GetAgent retrieves an agent by ID
func (g *Graph) GetAgent(agentID types.AgentID) (*types.Agent, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	agent, exists := g.agents[agentID]
	if !exists {
		return nil, fmt.Errorf("agent %s not found", agentID)
	}
	return agent, nil
}

// GetAllAgents returns all agents
func (g *Graph) GetAllAgents() []*types.Agent {
	g.mu.RLock()
	defer g.mu.RUnlock()

	agents := make([]*types.Agent, 0, len(g.agents))
	for _, agent := range g.agents {
		agents = append(agents, agent)
	}
	return agents
}

// GetNeighbors returns agents directly connected to the given agent (edges with weight > threshold)
func (g *Graph) GetNeighbors(agentID types.AgentID, minWeight float64) []types.AgentID {
	g.mu.RLock()
	defer g.mu.RUnlock()

	neighbors := []types.AgentID{}
	for _, edge := range g.edges {
		if edge.SourceID == agentID && edge.GetWeight() >= minWeight {
			neighbors = append(neighbors, edge.TargetID)
		}
	}
	return neighbors
}
