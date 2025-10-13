package state

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
)

// RedisStore handles Redis-based state management
type RedisStore struct {
	client *redis.Client
	config *types.Config
	logger *zap.Logger
}

// NewRedisStore creates a new Redis store
func NewRedisStore(config *types.Config, logger *zap.Logger) (*RedisStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr: config.RedisAddr,
		DB:   config.RedisDB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info("Connected to Redis", zap.String("addr", config.RedisAddr))

	return &RedisStore{
		client: client,
		config: config,
		logger: logger,
	}, nil
}

// SaveGraphSnapshot saves a graph snapshot to Redis
func (rs *RedisStore) SaveGraphSnapshot(ctx context.Context, snapshot *types.GraphSnapshot) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	key := "graph:snapshot:latest"
	if err := rs.client.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}

	// Also save with timestamp for history
	timestampKey := fmt.Sprintf("graph:snapshot:%d", snapshot.Timestamp.Unix())
	if err := rs.client.Set(ctx, timestampKey, data, 24*time.Hour).Err(); err != nil {
		rs.logger.Warn("Failed to save timestamped snapshot", zap.Error(err))
	}

	return nil
}

// LoadGraphSnapshot loads the latest graph snapshot from Redis
func (rs *RedisStore) LoadGraphSnapshot(ctx context.Context) (*types.GraphSnapshot, error) {
	key := "graph:snapshot:latest"
	data, err := rs.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("no snapshot found")
	} else if err != nil {
		return nil, fmt.Errorf("failed to load snapshot: %w", err)
	}

	var snapshot types.GraphSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	return &snapshot, nil
}

// SaveAgent saves an agent to Redis
func (rs *RedisStore) SaveAgent(ctx context.Context, agent *types.Agent) error {
	data, err := json.Marshal(agent)
	if err != nil {
		return fmt.Errorf("failed to marshal agent: %w", err)
	}

	key := fmt.Sprintf("agent:%s", agent.ID)
	if err := rs.client.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to save agent: %w", err)
	}

	// Add to agents set
	if err := rs.client.SAdd(ctx, "agents:all", string(agent.ID)).Err(); err != nil {
		return fmt.Errorf("failed to add agent to set: %w", err)
	}

	return nil
}

// LoadAgent loads an agent from Redis
func (rs *RedisStore) LoadAgent(ctx context.Context, agentID types.AgentID) (*types.Agent, error) {
	key := fmt.Sprintf("agent:%s", agentID)
	data, err := rs.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("agent not found")
	} else if err != nil {
		return nil, fmt.Errorf("failed to load agent: %w", err)
	}

	var agent types.Agent
	if err := json.Unmarshal(data, &agent); err != nil {
		return nil, fmt.Errorf("failed to unmarshal agent: %w", err)
	}

	return &agent, nil
}

// SaveProposal saves a proposal to Redis
func (rs *RedisStore) SaveProposal(ctx context.Context, proposal *types.Proposal) error {
	data, err := json.Marshal(proposal)
	if err != nil {
		return fmt.Errorf("failed to marshal proposal: %w", err)
	}

	key := fmt.Sprintf("proposal:%s", proposal.ID)
	ttl := time.Until(proposal.ExpiresAt) + time.Hour // Keep for 1 hour after expiry
	if err := rs.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to save proposal: %w", err)
	}

	// Add to proposals set
	if err := rs.client.SAdd(ctx, "proposals:all", string(proposal.ID)).Err(); err != nil {
		return fmt.Errorf("failed to add proposal to set: %w", err)
	}

	return nil
}

// LoadProposal loads a proposal from Redis
func (rs *RedisStore) LoadProposal(ctx context.Context, proposalID types.ProposalID) (*types.Proposal, error) {
	key := fmt.Sprintf("proposal:%s", proposalID)
	data, err := rs.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("proposal not found")
	} else if err != nil {
		return nil, fmt.Errorf("failed to load proposal: %w", err)
	}

	var proposal types.Proposal
	if err := json.Unmarshal(data, &proposal); err != nil {
		return nil, fmt.Errorf("failed to unmarshal proposal: %w", err)
	}

	return &proposal, nil
}

// IncrementCounter increments a counter in Redis
func (rs *RedisStore) IncrementCounter(ctx context.Context, key string) (int64, error) {
	return rs.client.Incr(ctx, key).Result()
}

// GetCounter gets a counter value from Redis
func (rs *RedisStore) GetCounter(ctx context.Context, key string) (int64, error) {
	val, err := rs.client.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// SetMetric sets a metric value in Redis
func (rs *RedisStore) SetMetric(ctx context.Context, key string, value float64) error {
	return rs.client.Set(ctx, fmt.Sprintf("metric:%s", key), value, time.Hour).Err()
}

// GetMetric gets a metric value from Redis
func (rs *RedisStore) GetMetric(ctx context.Context, key string) (float64, error) {
	val, err := rs.client.Get(ctx, fmt.Sprintf("metric:%s", key)).Float64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// Close closes the Redis connection
func (rs *RedisStore) Close() error {
	if err := rs.client.Close(); err != nil {
		return fmt.Errorf("failed to close Redis client: %w", err)
	}
	rs.logger.Info("Redis store closed")
	return nil
}

// DeleteAgent deletes an agent from Redis
func (rs *RedisStore) DeleteAgent(ctx context.Context, agentID types.AgentID) error {
	key := fmt.Sprintf("agent:%s", agentID)
	if err := rs.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	// Remove from agents set
	if err := rs.client.SRem(ctx, "agents:all", string(agentID)).Err(); err != nil {
		return fmt.Errorf("failed to remove agent from set: %w", err)
	}

	return nil
}

// Set stores a generic value in Redis with TTL
func (rs *RedisStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	if err := rs.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set key: %w", err)
	}

	return nil
}

// Get retrieves a generic value from Redis
func (rs *RedisStore) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := rs.client.Get(ctx, key).Bytes()
	if err != nil {
		return fmt.Errorf("failed to get key: %w", err)
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("failed to unmarshal value: %w", err)
	}

	return nil
}

// ListAgents lists all agent IDs
func (rs *RedisStore) ListAgents(ctx context.Context) ([]types.AgentID, error) {
	members, err := rs.client.SMembers(ctx, "agents:all").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	agentIDs := make([]types.AgentID, len(members))
	for i, member := range members {
		agentIDs[i] = types.AgentID(member)
	}

	return agentIDs, nil
}
