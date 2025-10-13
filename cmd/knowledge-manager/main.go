package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/avinashshinde/agentmesh-cortex/internal/config"
	"github.com/avinashshinde/agentmesh-cortex/internal/messaging"
	"github.com/avinashshinde/agentmesh-cortex/internal/state"
	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
)

// KnowledgeManager is a centralized service that collects and indexes insights from all agents
// It provides the "collective intelligence" layer for the AgentMesh

func main() {
	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting AgentMesh Knowledge Manager")

	// Load configuration
	cfg := config.Load()

	// Initialize Kafka messaging
	messaging := messaging.NewKafkaMessaging(cfg, logger)
	defer messaging.Close()

	// Initialize Redis state store
	stateStore, err := state.NewRedisStore(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer stateStore.Close()

	// Create knowledge manager
	km := NewKnowledgeManager(messaging, stateStore, cfg, logger)

	// Start knowledge manager
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := km.Start(ctx); err != nil {
		logger.Fatal("Failed to start knowledge manager", zap.Error(err))
	}

	logger.Info("Knowledge Manager running - collecting agent insights")

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	logger.Info("Knowledge Manager shutting down gracefully...")
}

// KnowledgeManager manages the collective knowledge from all agents
type KnowledgeManager struct {
	messaging  *messaging.KafkaMessaging
	stateStore *state.RedisStore
	config     *types.Config
	logger     *zap.Logger

	// In-memory cache for fast queries
	insights      map[types.InsightID]*types.Insight
	insightsMutex sync.RWMutex

	// Indexes for fast querying
	indexByTopic     map[string][]types.InsightID
	indexByAgent     map[types.AgentID][]types.InsightID
	indexByType      map[types.InsightType][]types.InsightID
	indexMutex       sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

func NewKnowledgeManager(
	msg *messaging.KafkaMessaging,
	store *state.RedisStore,
	cfg *types.Config,
	logger *zap.Logger,
) *KnowledgeManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &KnowledgeManager{
		messaging:  msg,
		stateStore: store,
		config:     cfg,
		logger:     logger.With(zap.String("component", "knowledge-manager")),
		insights:   make(map[types.InsightID]*types.Insight),
		indexByTopic: make(map[string][]types.InsightID),
		indexByAgent: make(map[types.AgentID][]types.InsightID),
		indexByType:  make(map[types.InsightType][]types.InsightID),
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (km *KnowledgeManager) Start(ctx context.Context) error {
	km.logger.Info("Knowledge Manager starting")

	// Load existing insights from Redis
	if err := km.loadInsightsFromRedis(); err != nil {
		km.logger.Warn("Failed to load insights from Redis", zap.Error(err))
	}

	// Start insight consumer
	go km.consumeInsights()

	// Start periodic persistence
	go km.periodicPersistence()

	// Start pattern detection
	go km.detectPatterns()

	return nil
}

func (km *KnowledgeManager) Stop() error {
	km.logger.Info("Knowledge Manager stopping")

	// Save insights to Redis before shutdown
	if err := km.saveInsightsToRedis(); err != nil {
		km.logger.Error("Failed to save insights to Redis", zap.Error(err))
	}

	km.cancel()
	return nil
}

// consumeInsights listens to Kafka for insights published by agents
func (km *KnowledgeManager) consumeInsights() {
	groupID := "knowledge-manager"
	err := km.messaging.ConsumeMessages(km.ctx, "insights", groupID, func(msg *types.Message) error {
		// Parse insight from message payload
		insightData, ok := msg.Payload["insight"]
		if !ok {
			return fmt.Errorf("message missing insight data")
		}

		// Convert to JSON and back to Insight struct
		jsonData, err := json.Marshal(insightData)
		if err != nil {
			return fmt.Errorf("failed to marshal insight: %w", err)
		}

		var insight types.Insight
		if err := json.Unmarshal(jsonData, &insight); err != nil {
			return fmt.Errorf("failed to unmarshal insight: %w", err)
		}

		// Add to knowledge base
		km.addInsight(&insight)

		km.logger.Info("Received insight",
			zap.String("insight_id", string(insight.ID)),
			zap.String("agent_id", string(insight.AgentID)),
			zap.String("type", string(insight.Type)),
			zap.String("topic", insight.Topic),
			zap.Float64("confidence", insight.Confidence),
		)

		return nil
	})

	if err != nil && err != context.Canceled {
		km.logger.Error("Insight consumption stopped", zap.Error(err))
	}
}

// addInsight adds an insight to the knowledge base and updates indexes
func (km *KnowledgeManager) addInsight(insight *types.Insight) {
	km.insightsMutex.Lock()
	km.insights[insight.ID] = insight
	km.insightsMutex.Unlock()

	// Update indexes
	km.indexMutex.Lock()
	defer km.indexMutex.Unlock()

	// Index by topic
	km.indexByTopic[insight.Topic] = append(km.indexByTopic[insight.Topic], insight.ID)

	// Index by agent
	km.indexByAgent[insight.AgentID] = append(km.indexByAgent[insight.AgentID], insight.ID)

	// Index by type
	km.indexByType[insight.Type] = append(km.indexByType[insight.Type], insight.ID)
}

// QueryInsights queries the knowledge base with filters
func (km *KnowledgeManager) QueryInsights(query types.KnowledgeQuery) types.KnowledgeQueryResult {
	km.insightsMutex.RLock()
	defer km.insightsMutex.RUnlock()

	var matchingInsights []types.Insight

	// Get candidate insights from indexes
	var candidateIDs []types.InsightID

	if len(query.Topics) > 0 {
		// Filter by topics
		km.indexMutex.RLock()
		for _, topic := range query.Topics {
			candidateIDs = append(candidateIDs, km.indexByTopic[topic]...)
		}
		km.indexMutex.RUnlock()
	} else if len(query.InsightTypes) > 0 {
		// Filter by insight types
		km.indexMutex.RLock()
		for _, insightType := range query.InsightTypes {
			candidateIDs = append(candidateIDs, km.indexByType[insightType]...)
		}
		km.indexMutex.RUnlock()
	} else {
		// No filters - check all insights
		for id := range km.insights {
			candidateIDs = append(candidateIDs, id)
		}
	}

	// Apply filters
	for _, insightID := range candidateIDs {
		insight, ok := km.insights[insightID]
		if !ok {
			continue
		}

		// Check confidence threshold
		if insight.Confidence < query.MinConfidence {
			continue
		}

		// Check time range
		if query.TimeFrom != nil && insight.CreatedAt.Before(*query.TimeFrom) {
			continue
		}
		if query.TimeTo != nil && insight.CreatedAt.After(*query.TimeTo) {
			continue
		}

		// Check agent types
		if len(query.AgentTypes) > 0 {
			found := false
			for _, agentType := range query.AgentTypes {
				if insight.AgentRole == agentType {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		matchingInsights = append(matchingInsights, *insight)

		// Apply limit
		if query.Limit > 0 && len(matchingInsights) >= query.Limit {
			break
		}
	}

	return types.KnowledgeQueryResult{
		Query:     query,
		Insights:  matchingInsights,
		Count:     len(matchingInsights),
		Timestamp: time.Now(),
	}
}

// detectPatterns analyzes insights to detect emergent patterns
func (km *KnowledgeManager) detectPatterns() {
	ticker := time.NewTicker(60 * time.Second) // Check every minute
	defer ticker.Stop()

	for {
		select {
		case <-km.ctx.Done():
			return
		case <-ticker.C:
			km.analyzePatterns()
		}
	}
}

// analyzePatterns looks for repeated topics or correlations across insights
func (km *KnowledgeManager) analyzePatterns() {
	km.insightsMutex.RLock()
	defer km.insightsMutex.RUnlock()

	// Count insights by topic
	topicCounts := make(map[string]int)
	for _, insight := range km.insights {
		topicCounts[insight.Topic]++
	}

	// Log patterns where topic appears 3+ times
	for topic, count := range topicCounts {
		if count >= 3 {
			km.logger.Info("Pattern detected",
				zap.String("type", "repeated_topic"),
				zap.String("topic", topic),
				zap.Int("frequency", count),
			)
		}
	}
}

// periodicPersistence saves insights to Redis every 30 seconds
func (km *KnowledgeManager) periodicPersistence() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-km.ctx.Done():
			return
		case <-ticker.C:
			if err := km.saveInsightsToRedis(); err != nil {
				km.logger.Error("Failed to persist insights", zap.Error(err))
			}
		}
	}
}

// saveInsightsToRedis persists all insights to Redis
func (km *KnowledgeManager) saveInsightsToRedis() error {
	km.insightsMutex.RLock()
	defer km.insightsMutex.RUnlock()

	for id, insight := range km.insights {
		key := fmt.Sprintf("insight:%s", id)
		if err := km.stateStore.Set(km.ctx, key, insight, 7*24*time.Hour); err != nil {
			return fmt.Errorf("failed to save insight %s: %w", id, err)
		}
	}

	km.logger.Debug("Persisted insights to Redis", zap.Int("count", len(km.insights)))
	return nil
}

// loadInsightsFromRedis loads existing insights from Redis
func (km *KnowledgeManager) loadInsightsFromRedis() error {
	// Note: This is a simplified version
	// In production, you'd use SCAN to iterate through all insight:* keys
	km.logger.Info("Loading insights from Redis")
	return nil
}
