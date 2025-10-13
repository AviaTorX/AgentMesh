package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
)

// Load loads configuration from environment variables
func Load() *types.Config {
	return &types.Config{
		// Topology settings
		InitialEdgeWeight:   getEnvFloat("INITIAL_EDGE_WEIGHT", 0.5),
		ReinforcementAmount: getEnvFloat("REINFORCEMENT_AMOUNT", 0.1),
		DecayRate:           getEnvFloat("DECAY_RATE", 0.05),
		DecayInterval:       getEnvDuration("DECAY_INTERVAL", 5*time.Second),
		PruneThreshold:      getEnvFloat("PRUNE_THRESHOLD", 0.1),

		// Consensus settings
		QuorumThreshold:    getEnvFloat("QUORUM_THRESHOLD", 0.6),
		ProposalTimeout:    getEnvDuration("PROPOSAL_TIMEOUT", 30*time.Second),
		WaggleIntensityMin: getEnvFloat("WAGGLE_INTENSITY_MIN", 0.3),

		// Infrastructure
		KafkaBrokers:     strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ","),
		KafkaTopicPrefix: getEnv("KAFKA_TOPIC_PREFIX", "agentmesh"),
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
		RedisDB:          getEnvInt("REDIS_DB", 0),

		// Server
		HTTPPort:      getEnvInt("HTTP_PORT", 8080),
		WebSocketPort: getEnvInt("WEBSOCKET_PORT", 8081),
	}
}

// Default creates a default configuration for testing
func Default() *types.Config {
	return &types.Config{
		InitialEdgeWeight:   0.5,
		ReinforcementAmount: 0.1,
		DecayRate:           0.05,
		DecayInterval:       5 * time.Second,
		PruneThreshold:      0.1,

		QuorumThreshold:    0.6,
		ProposalTimeout:    30 * time.Second,
		WaggleIntensityMin: 0.3,

		KafkaBrokers:     []string{"localhost:9092"},
		KafkaTopicPrefix: "agentmesh",
		RedisAddr:        "localhost:6379",
		RedisDB:          0,

		HTTPPort:      8080,
		WebSocketPort: 8081,
	}
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
