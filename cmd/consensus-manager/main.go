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
	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
)

// Consensus Manager: Central service that manages proposals and voting
// Listens to Kafka for proposals and votes
// Applies Bee consensus algorithm (quorum detection)
// Publishes results to Redis + Kafka

func main() {
	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting Consensus Manager (Bee Swarm)")

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

	// Initialize Bee consensus
	beeConsensus := consensus.NewBeeConsensus(cfg, logger)
	ctx := context.Background()
	if err := beeConsensus.Start(ctx); err != nil {
		logger.Fatal("Failed to start Bee consensus", zap.Error(err))
	}
	defer beeConsensus.Stop()

	// Listen to proposals from Kafka
	go listenToProposals(ctx, kafkaMessaging, beeConsensus, redisStore, logger)

	// Listen to votes from Kafka
	go listenToVotes(ctx, kafkaMessaging, beeConsensus, logger)

	// Monitor consensus events
	go monitorConsensusEvents(beeConsensus, kafkaMessaging, logger)

	// Print stats periodically
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			stats := beeConsensus.GetStats()
			logger.Info("Consensus stats",
				zap.Int("total_proposals", stats["total_proposals"]),
				zap.Int("pending", stats["pending_proposals"]),
				zap.Int("accepted", stats["accepted_proposals"]),
				zap.Int("active_agents", stats["active_agents"]),
			)
		}
	}()

	logger.Info("Consensus Manager running")

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	logger.Info("Consensus Manager shutting down...")
}

func listenToProposals(ctx context.Context, messaging *messaging.KafkaMessaging, beeConsensus *consensus.BeeConsensus, redisStore *state.RedisStore, logger *zap.Logger) {
	err := messaging.ConsumeMessages(ctx, "proposals", "consensus-manager", func(msg *types.Message) error {
		// Parse proposal from message
		proposalData, ok := msg.Payload["proposal"].(map[string]any)
		if !ok {
			return nil
		}

		proposerID := types.AgentID(proposalData["proposer_id"].(string))
		proposalType := types.ProposalType(proposalData["type"].(string))
		content := proposalData["content"].(map[string]any)

		// Create proposal in consensus engine
		proposal, err := beeConsensus.CreateProposal(proposerID, proposalType, content)
		if err != nil {
			logger.Error("Failed to create proposal", zap.Error(err))
			return err
		}

		// Save to Redis
		if err := redisStore.SaveProposal(ctx, proposal); err != nil {
			logger.Error("Failed to save proposal to Redis", zap.Error(err))
		}

		logger.Info("Proposal created",
			zap.String("proposal_id", string(proposal.ID)),
			zap.String("proposer", string(proposerID)),
		)

		return nil
	})

	if err != nil && err != context.Canceled {
		logger.Error("Proposal listener stopped", zap.Error(err))
	}
}

func listenToVotes(ctx context.Context, messaging *messaging.KafkaMessaging, beeConsensus *consensus.BeeConsensus, logger *zap.Logger) {
	err := messaging.ConsumeMessages(ctx, "votes", "consensus-manager", func(msg *types.Message) error {
		// Parse vote from message
		voteData, ok := msg.Payload["vote"].(map[string]any)
		if !ok {
			return nil
		}

		proposalID := types.ProposalID(voteData["proposal_id"].(string))
		voterID := types.AgentID(voteData["voter_id"].(string))
		support := voteData["support"].(bool)
		intensity := voteData["intensity"].(float64)

		// Register vote
		if err := beeConsensus.Vote(proposalID, voterID, support, intensity); err != nil {
			logger.Error("Failed to register vote", zap.Error(err))
			return err
		}

		logger.Debug("Vote registered",
			zap.String("proposal_id", string(proposalID)),
			zap.String("voter_id", string(voterID)),
			zap.Bool("support", support),
		)

		return nil
	})

	if err != nil && err != context.Canceled {
		logger.Error("Vote listener stopped", zap.Error(err))
	}
}

func monitorConsensusEvents(beeConsensus *consensus.BeeConsensus, messaging *messaging.KafkaMessaging, logger *zap.Logger) {
	for event := range beeConsensus.EventChannel() {
		switch event.Type {
		case consensus.ConsensusEventProposalCreated:
			logger.Info("[PROPOSAL] Proposal created",
				zap.String("proposal_id", string(event.ProposalID)),
			)
		case consensus.ConsensusEventQuorumReached:
			logger.Info("[QUORUM] Quorum reached!",
				zap.String("proposal_id", string(event.ProposalID)),
			)
		case consensus.ConsensusEventProposalAccepted:
			logger.Info("[ACCEPTED] Proposal ACCEPTED",
				zap.String("proposal_id", string(event.ProposalID)),
			)
		case consensus.ConsensusEventProposalRejected:
			logger.Info("[REJECTED] Proposal REJECTED",
				zap.String("proposal_id", string(event.ProposalID)),
			)
		}
	}
}
