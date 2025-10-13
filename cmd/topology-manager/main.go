package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/avinashshinde/agentmesh-cortex/internal/config"
	"github.com/avinashshinde/agentmesh-cortex/internal/messaging"
	"github.com/avinashshinde/agentmesh-cortex/internal/state"
	"github.com/avinashshinde/agentmesh-cortex/internal/topology"
	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
)

// Topology Manager: Central service that maintains the network graph
// Listens to Kafka for agent/message events
// Applies SlimeMold algorithm (reinforcement, decay, pruning)
// Publishes updates to Redis + Kafka

func main() {
	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting Topology Manager (SlimeMold)")

	// Load configuration
	cfg := config.Load()

	// Initialize Redis store
	redisStore, err := state.NewRedisStore(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to initialize Redis", zap.Error(err))
	}
	defer redisStore.Close()

	// Initialize Kafka messaging
	kafkaMessaging := messaging.NewKafkaMessaging(cfg, logger)
	defer kafkaMessaging.Close()

	// Initialize SlimeMold topology
	slimeMold := topology.NewSlimeMoldTopology(cfg, logger)
	ctx := context.Background()
	if err := slimeMold.Start(ctx); err != nil {
		logger.Fatal("Failed to start SlimeMold", zap.Error(err))
	}
	defer slimeMold.Stop()

	// Start listening to topology events from Kafka
	go listenToTopologyEvents(ctx, kafkaMessaging, slimeMold, logger)

	// Start listening to messages (for edge reinforcement)
	go listenToMessages(ctx, kafkaMessaging, slimeMold, logger)

	// Periodically save snapshot to Redis
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			snapshot := slimeMold.GetSnapshot()
			if err := redisStore.SaveGraphSnapshot(ctx, snapshot); err != nil {
				logger.Error("Failed to save snapshot", zap.Error(err))
			}
		}
	}()

	// Print stats periodically
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			slimeMold.PrintStats()
		}
	}()

	logger.Info("Topology Manager running")

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	logger.Info("Topology Manager shutting down...")
}

func listenToTopologyEvents(ctx context.Context, messaging *messaging.KafkaMessaging, slimeMold *topology.SlimeMoldTopology, logger *zap.Logger) {
	// Listen to topology events (agent joined/left)
	err := messaging.ConsumeMessages(ctx, "topology", "topology-manager", func(msg *types.Message) error {
		// Parse topology event from message
		eventType, ok := msg.Payload["type"].(string)
		if !ok {
			return nil
		}

		switch eventType {
		case "agent_joined":
			agentID := types.AgentID(msg.Payload["agent_id"].(string))
			agent := &types.Agent{
				ID:        agentID,
				Name:      msg.Payload["name"].(string),
				Role:      msg.Payload["role"].(string),
				Status:    types.AgentStatusActive,
				CreatedAt: time.Now(),
			}
			if err := slimeMold.AddAgent(agent); err != nil {
				logger.Error("Failed to add agent", zap.Error(err))
			} else {
				logger.Info("Agent added to topology", zap.String("agent_id", string(agentID)))
			}

		case "agent_left":
			agentID := types.AgentID(msg.Payload["agent_id"].(string))
			if err := slimeMold.RemoveAgent(agentID); err != nil {
				logger.Error("Failed to remove agent", zap.Error(err))
			} else {
				logger.Info("Agent removed from topology", zap.String("agent_id", string(agentID)))
			}
		}

		return nil
	})

	if err != nil && err != context.Canceled {
		logger.Error("Topology event listener stopped", zap.Error(err))
	}
}

func listenToMessages(ctx context.Context, messaging *messaging.KafkaMessaging, slimeMold *topology.SlimeMoldTopology, logger *zap.Logger) {
	// Listen to all messages for edge reinforcement
	err := messaging.ConsumeMessages(ctx, "messages", "topology-reinforcement", func(msg *types.Message) error {
		// Reinforce edge for every message
		if err := slimeMold.ReinforceEdge(msg.FromAgentID, msg.ToAgentID); err != nil {
			logger.Debug("Failed to reinforce edge", zap.Error(err))
		}
		return nil
	})

	if err != nil && err != context.Canceled {
		logger.Error("Message listener stopped", zap.Error(err))
	}
}
