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
	"github.com/avinashshinde/agentmesh-cortex/internal/consensus"
	"github.com/avinashshinde/agentmesh-cortex/internal/messaging"
	"github.com/avinashshinde/agentmesh-cortex/internal/state"
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

	logger.Info("TPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPW")
	logger.Info("Q   AgentMesh Cortex - Bio-Inspired Agents    Q")
	logger.Info("Q   SlimeMold Topology + Bee Consensus         Q")
	logger.Info("ZPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPP]")

	// Load configuration
	cfg := config.Load()
	logger.Info("Configuration loaded",
		zap.Strings("kafka_brokers", cfg.KafkaBrokers),
		zap.String("redis_addr", cfg.RedisAddr),
	)

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
		logger.Fatal("Failed to start SlimeMold topology", zap.Error(err))
	}
	defer slimeMold.Stop()

	// Initialize Bee consensus
	beeConsensus := consensus.NewBeeConsensus(cfg, logger)
	if err := beeConsensus.Start(ctx); err != nil {
		logger.Fatal("Failed to start Bee consensus", zap.Error(err))
	}
	defer beeConsensus.Stop()

	// Start monitoring topology events
	go monitorTopologyEvents(slimeMold, logger)

	// Start monitoring consensus events
	go monitorConsensusEvents(beeConsensus, logger)

	// Print stats periodically
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			slimeMold.PrintStats()

			stats := beeConsensus.GetStats()
			logger.Info("Consensus stats",
				zap.Int("total_proposals", stats["total_proposals"]),
				zap.Int("pending", stats["pending_proposals"]),
				zap.Int("accepted", stats["accepted_proposals"]),
				zap.Int("active_agents", stats["active_agents"]),
			)
		}
	}()

	logger.Info("TPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPW")
	logger.Info("Q   AgentMesh Cortex is running!                Q")
	logger.Info("Q   Press Ctrl+C to stop                        Q")
	logger.Info("ZPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPP]")

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	logger.Info("Shutting down gracefully...")
}

// monitorTopologyEvents monitors and logs topology events
func monitorTopologyEvents(slimeMold *topology.SlimeMoldTopology, logger *zap.Logger) {
	for event := range slimeMold.EventChannel() {
		switch event.Type {
		case types.TopologyEventAgentJoined:
			logger.Info("[+] Agent joined mesh",
				zap.String("agent_id", string(event.AgentID)),
			)
		case types.TopologyEventAgentLeft:
			logger.Info("[-] Agent left mesh",
				zap.String("agent_id", string(event.AgentID)),
			)
		case types.TopologyEventEdgeRemoved:
			logger.Debug("[PRUNED] Edge pruned",
				zap.String("edge_id", string(event.EdgeID)),
			)
		case types.TopologyEventEdgeStrength:
			if event.Edge != nil {
				logger.Debug("[REINFORCE] Edge reinforced",
					zap.String("edge_id", string(event.EdgeID)),
					zap.Float64("weight", event.Edge.GetWeight()),
				)
			}
		}
	}
}

// monitorConsensusEvents monitors and logs consensus events
func monitorConsensusEvents(beeConsensus *consensus.BeeConsensus, logger *zap.Logger) {
	for event := range beeConsensus.EventChannel() {
		switch event.Type {
		case consensus.ConsensusEventProposalCreated:
			logger.Info("=� Proposal created",
				zap.String("proposal_id", string(event.ProposalID)),
				zap.String("proposer", string(event.Proposal.ProposerID)),
			)
		case consensus.ConsensusEventQuorumReached:
			logger.Info(" Quorum reached!",
				zap.String("proposal_id", string(event.ProposalID)),
				zap.Int("votes", len(event.Proposal.Votes)),
			)
		case consensus.ConsensusEventProposalAccepted:
			logger.Info("<� Proposal ACCEPTED",
				zap.String("proposal_id", string(event.ProposalID)),
			)
		case consensus.ConsensusEventProposalRejected:
			logger.Info("L Proposal REJECTED",
				zap.String("proposal_id", string(event.ProposalID)),
			)
		case consensus.ConsensusEventProposalExpired:
			logger.Warn("� Proposal expired",
				zap.String("proposal_id", string(event.ProposalID)),
			)
		}
	}
}
