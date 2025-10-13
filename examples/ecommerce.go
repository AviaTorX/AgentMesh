package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/avinashshinde/agentmesh-cortex/internal/agent"
	"github.com/avinashshinde/agentmesh-cortex/internal/config"
	"github.com/avinashshinde/agentmesh-cortex/internal/consensus"
	"github.com/avinashshinde/agentmesh-cortex/internal/messaging"
	"github.com/avinashshinde/agentmesh-cortex/internal/topology"
	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
)

func main() {
	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("TPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPW")
	logger.Info("Q   AgentMesh Cortex E-Commerce Demo           Q")
	logger.Info("Q   4 Agents: Sales, Support, Inventory, Fraud  Q")
	logger.Info("ZPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPP]")

	// Load configuration
	cfg := config.Default()

	// Initialize Kafka messaging
	kafkaMessaging := messaging.NewKafkaMessaging(cfg, logger)
	defer kafkaMessaging.Close()

	// Initialize SlimeMold topology
	slimeMold := topology.NewSlimeMoldTopology(cfg, logger)
	ctx := context.Background()
	if err := slimeMold.Start(ctx); err != nil {
		logger.Fatal("Failed to start SlimeMold topology", zap.Error(err))
	}
	defer slimeMold.Stop()

	// Initialize Bee consensus
	beeConsensus := consensus.NewBeeConsensus(cfg, logger)
	if err := beeConsensus.Start(ctx); err != nil {
		logger.Fatal("Failed to start Bee consensus", zap.Error(err))
	}
	defer beeConsensus.Stop()

	// Create e-commerce agents
	agents := createECommerceAgents(slimeMold, beeConsensus, kafkaMessaging, cfg, logger)

	// Start all agents
	for _, ag := range agents {
		if err := ag.Start(); err != nil {
			logger.Fatal("Failed to start agent", zap.Error(err))
		}
	}
	defer func() {
		for _, ag := range agents {
			ag.Stop()
		}
	}()

	logger.Info("[START] All agents started! Initial topology: FULL MESH (12 edges)")
	logger.Info("[WATCH] Watching topology evolve over next 2 minutes...")

	// Simulate e-commerce scenarios
	go simulateECommerceScenarios(agents, logger)

	// Monitor topology evolution
	go monitorTopologyEvolution(slimeMold, logger)

	// Print statistics periodically
	go printPeriodicStats(slimeMold, beeConsensus, logger)

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	logger.Info("Shutting down demo...")
}

// createECommerceAgents creates 4 e-commerce agents
func createECommerceAgents(
	topo *topology.SlimeMoldTopology,
	cons *consensus.BeeConsensus,
	msg *messaging.KafkaMessaging,
	cfg *types.Config,
	logger *zap.Logger,
) []*agent.AgentRuntime {
	now := time.Now()

	agents := []*agent.AgentRuntime{
		// Sales Agent
		agent.NewAgentRuntime(
			&types.Agent{
				ID:           types.NewAgentID(),
				Name:         "Sales Agent",
				Role:         "sales",
				Status:       types.AgentStatusActive,
				Capabilities: []string{"process_order", "upsell", "discount_approval"},
				CreatedAt:    now,
				LastSeenAt:   now,
			},
			topo, cons, msg, cfg, logger,
		),

		// Support Agent
		agent.NewAgentRuntime(
			&types.Agent{
				ID:           types.NewAgentID(),
				Name:         "Support Agent",
				Role:         "support",
				Status:       types.AgentStatusActive,
				Capabilities: []string{"handle_ticket", "refund_approval", "escalate"},
				CreatedAt:    now,
				LastSeenAt:   now,
			},
			topo, cons, msg, cfg, logger,
		),

		// Inventory Agent
		agent.NewAgentRuntime(
			&types.Agent{
				ID:           types.NewAgentID(),
				Name:         "Inventory Agent",
				Role:         "inventory",
				Status:       types.AgentStatusActive,
				Capabilities: []string{"check_stock", "reserve_items", "restock_alert"},
				CreatedAt:    now,
				LastSeenAt:   now,
			},
			topo, cons, msg, cfg, logger,
		),

		// Fraud Detection Agent
		agent.NewAgentRuntime(
			&types.Agent{
				ID:           types.NewAgentID(),
				Name:         "Fraud Agent",
				Role:         "fraud",
				Status:       types.AgentStatusActive,
				Capabilities: []string{"risk_assessment", "block_transaction", "verify_user"},
				CreatedAt:    now,
				LastSeenAt:   now,
			},
			topo, cons, msg, cfg, logger,
		),
	}

	return agents
}

// simulateECommerceScenarios simulates realistic e-commerce interactions
func simulateECommerceScenarios(agents []*agent.AgentRuntime, logger *zap.Logger) {
	salesAgent := agents[0]
	supportAgent := agents[1]
	inventoryAgent := agents[2]
	fraudAgent := agents[3]

	time.Sleep(2 * time.Second)

	// Scenario 1: Large order requires consensus
	logger.Info("= Scenario 1: Large order ($50,000) - requires approval")
	proposal, _ := salesAgent.ProposeAction(types.ProposalTypeDecision, map[string]any{
		"type":       "approval",
		"order_id":   "ORD-12345",
		"amount":     50000.0,
		"priority":   "high",
		"urgent":     true,
		"confidence": 0.8,
	})

	// Agents vote on the proposal
	time.Sleep(1 * time.Second)
	salesAgent.VoteOnProposal(proposal.ID, true, 0.9)
	inventoryAgent.VoteOnProposal(proposal.ID, true, 0.7)
	fraudAgent.VoteOnProposal(proposal.ID, true, 0.6)
	supportAgent.VoteOnProposal(proposal.ID, true, 0.8)

	// Scenario 2: Frequent Sales <-> Inventory communication
	logger.Info("= Scenario 2: High-frequency Sales  Inventory communication")
	for i := 0; i < 20; i++ {
		salesAgent.SendMessage(inventoryAgent.GetAgent().ID, types.MessageTypeTask, map[string]any{
			"action": "check_stock",
			"sku":    fmt.Sprintf("SKU-%d", i),
		})
		inventoryAgent.SendMessage(salesAgent.GetAgent().ID, types.MessageTypeResponse, map[string]any{
			"status": "in_stock",
			"qty":    100 - i,
		})
		time.Sleep(200 * time.Millisecond)
	}

	// Scenario 3: Support occasionally contacts Fraud
	logger.Info("[CHECK] Scenario 3: Occasional Support -> Fraud checks")
	for i := 0; i < 3; i++ {
		supportAgent.SendMessage(fraudAgent.GetAgent().ID, types.MessageTypeTask, map[string]any{
			"action":  "verify_user",
			"user_id": fmt.Sprintf("USER-%d", i),
		})
		time.Sleep(2 * time.Second)
	}

	logger.Info(" Scenarios complete! Observe topology optimization...")
}

// monitorTopologyEvolution monitors how topology changes over time
func monitorTopologyEvolution(slimeMold *topology.SlimeMoldTopology, logger *zap.Logger) {
	initialSnapshot := slimeMold.GetSnapshot()
	initialEdges := initialSnapshot.Stats.TotalEdges

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		snapshot := slimeMold.GetSnapshot()
		reduction := snapshot.Stats.ReductionPercent

		logger.Info("= Topology Evolution",
			zap.Int("initial_edges", initialEdges),
			zap.Int("current_edges", snapshot.Stats.TotalEdges),
			zap.Int("active_edges", snapshot.Stats.ActiveEdges),
			zap.Float64("reduction", reduction),
			zap.Float64("avg_weight", snapshot.Stats.AverageWeight),
		)

		if reduction >= 50.0 {
			logger.Info("< TARGET ACHIEVED: 50%+ edge reduction!")
		}
	}
}

// printPeriodicStats prints statistics every 15 seconds
func printPeriodicStats(slimeMold *topology.SlimeMoldTopology, beeConsensus *consensus.BeeConsensus, logger *zap.Logger) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		snapshot := slimeMold.GetSnapshot()
		consensusStats := beeConsensus.GetStats()

		logger.Info("PPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPP")
		logger.Info("= SYSTEM STATS",
			zap.Int("agents", snapshot.Stats.TotalAgents),
			zap.Int("edges", snapshot.Stats.TotalEdges),
			zap.Int("active_edges", snapshot.Stats.ActiveEdges),
			zap.Float64("density", snapshot.Stats.Density),
			zap.Float64("reduction", snapshot.Stats.ReductionPercent),
			zap.Int("proposals", consensusStats["total_proposals"]),
			zap.Int("accepted", consensusStats["accepted_proposals"]),
		)
		logger.Info("PPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPP")
	}
}
