package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/avinashshinde/agentmesh-cortex/internal/config"
	"github.com/avinashshinde/agentmesh-cortex/internal/messaging"
	"github.com/avinashshinde/agentmesh-cortex/internal/state"
	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
)

// APIServer provides REST API access to AgentMesh collective knowledge

func main() {
	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting AgentMesh API Server")

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

	// Create API server
	server := NewAPIServer(messaging, stateStore, cfg, logger)

	// Start HTTP server
	port := 8080
	if cfg.HTTPPort > 0 {
		port = cfg.HTTPPort
	}

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: server.setupRoutes(),
	}

	go func() {
		logger.Info("API Server listening", zap.Int("port", port))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	logger.Info("API Server shutting down gracefully...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	httpServer.Shutdown(ctx)
}

// APIServer handles HTTP requests for querying AgentMesh
type APIServer struct {
	messaging  *messaging.KafkaMessaging
	stateStore *state.RedisStore
	config     *types.Config
	logger     *zap.Logger
}

func NewAPIServer(
	msg *messaging.KafkaMessaging,
	store *state.RedisStore,
	cfg *types.Config,
	logger *zap.Logger,
) *APIServer {
	return &APIServer{
		messaging:  msg,
		stateStore: store,
		config:     cfg,
		logger:     logger.With(zap.String("component", "api-server")),
	}
}

func (api *APIServer) setupRoutes() http.Handler {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", api.handleHealth)

	// Insights endpoints
	mux.HandleFunc("/api/insights", api.handleQueryInsights)
	mux.HandleFunc("/api/insights/search", api.handleSearchInsights)

	// Agent endpoints
	mux.HandleFunc("/api/agents", api.handleListAgents)
	mux.HandleFunc("/api/agents/", api.handleGetAgent)

	// Topology endpoints
	mux.HandleFunc("/api/topology", api.handleGetTopology)
	mux.HandleFunc("/api/topology/stats", api.handleTopologyStats)

	// Query endpoint (natural language)
	mux.HandleFunc("/api/query", api.handleNaturalLanguageQuery)

	// Add CORS middleware
	return corsMiddleware(mux)
}

// handleHealth returns server health status
func (api *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"service": "agentmesh-api",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// handleQueryInsights handles GET /api/insights with filters
func (api *APIServer) handleQueryInsights(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	query := types.KnowledgeQuery{
		Limit: 50, // Default limit
	}

	if topics := r.URL.Query()["topic"]; len(topics) > 0 {
		query.Topics = topics
	}

	if agentTypes := r.URL.Query()["agent_type"]; len(agentTypes) > 0 {
		query.AgentTypes = agentTypes
	}

	if minConf := r.URL.Query().Get("min_confidence"); minConf != "" {
		if conf, err := strconv.ParseFloat(minConf, 64); err == nil {
			query.MinConfidence = conf
		}
	}

	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			query.Limit = l
		}
	}

	// Query insights from Redis
	insights, err := api.queryInsightsFromRedis(r.Context(), query)
	if err != nil {
		api.logger.Error("Failed to query insights", zap.Error(err))
		http.Error(w, "Failed to query insights", http.StatusInternalServerError)
		return
	}

	result := types.KnowledgeQueryResult{
		Query:     query,
		Insights:  insights,
		Count:     len(insights),
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleSearchInsights handles POST /api/insights/search with JSON body
func (api *APIServer) handleSearchInsights(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var query types.KnowledgeQuery
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Query insights
	insights, err := api.queryInsightsFromRedis(r.Context(), query)
	if err != nil {
		api.logger.Error("Failed to search insights", zap.Error(err))
		http.Error(w, "Failed to search insights", http.StatusInternalServerError)
		return
	}

	result := types.KnowledgeQueryResult{
		Query:     query,
		Insights:  insights,
		Count:     len(insights),
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleNaturalLanguageQuery handles POST /api/query (natural language)
func (api *APIServer) handleNaturalLanguageQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Question string `json:"question"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Question == "" {
		http.Error(w, "Question is required", http.StatusBadRequest)
		return
	}

	// For now, simple keyword extraction
	// TODO: Use embeddings for semantic search in Phase 3
	query := types.KnowledgeQuery{
		Question:      req.Question,
		MinConfidence: 0.5,
		Limit:         10,
	}

	insights, err := api.queryInsightsFromRedis(r.Context(), query)
	if err != nil {
		api.logger.Error("Failed to process natural language query", zap.Error(err))
		http.Error(w, "Failed to process query", http.StatusInternalServerError)
		return
	}

	result := types.KnowledgeQueryResult{
		Query:     query,
		Insights:  insights,
		Count:     len(insights),
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleListAgents returns all active agents
func (api *APIServer) handleListAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Query agents from Redis (simplified)
	agents := []map[string]any{
		{
			"id":     "agent-sales-1",
			"name":   "Sales",
			"role":   "sales",
			"status": "active",
		},
		{
			"id":     "agent-support-1",
			"name":   "Support",
			"role":   "support",
			"status": "active",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"agents": agents,
		"count":  len(agents),
	})
}

// handleGetAgent returns details for a specific agent
func (api *APIServer) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	// Extract agent ID from path
	agentID := r.URL.Path[len("/api/agents/"):]

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":     agentID,
		"name":   "Agent",
		"status": "active",
	})
}

// handleGetTopology returns the current network topology
func (api *APIServer) handleGetTopology(w http.ResponseWriter, r *http.Request) {
	// Query topology snapshot from Redis
	ctx := r.Context()
	var snapshot types.GraphSnapshot

	err := api.stateStore.Get(ctx, "graph:snapshot:latest", &snapshot)
	if err != nil {
		api.logger.Warn("Failed to get topology snapshot", zap.Error(err))
		// Return empty snapshot
		snapshot = types.GraphSnapshot{
			Agents:    make(map[types.AgentID]*types.Agent),
			Edges:     make(map[types.EdgeID]*types.Edge),
			Timestamp: time.Now(),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snapshot)
}

// handleTopologyStats returns topology statistics
func (api *APIServer) handleTopologyStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var snapshot types.GraphSnapshot

	err := api.stateStore.Get(ctx, "graph:snapshot:latest", &snapshot)
	if err != nil {
		api.logger.Warn("Failed to get topology stats", zap.Error(err))
		http.Error(w, "Failed to get stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snapshot.Stats)
}

// queryInsightsFromRedis queries insights from Redis with filters
func (api *APIServer) queryInsightsFromRedis(ctx context.Context, query types.KnowledgeQuery) ([]types.Insight, error) {
	// Simplified implementation - in production, use Redis indexes or search
	// For now, return sample insights

	insights := []types.Insight{
		{
			ID:         "insight-1",
			AgentID:    "agent-sales-1",
			AgentRole:  "sales",
			Type:       types.InsightTypePricingIssue,
			Topic:      "pricing",
			Content:    "Customer complained that price is too high for basic features",
			Confidence: 0.85,
			CreatedAt:  time.Now().Add(-1 * time.Hour),
			Privacy:    types.InsightPrivacyPublic,
		},
		{
			ID:         "insight-2",
			AgentID:    "agent-support-1",
			AgentRole:  "support",
			Type:       types.InsightTypeProductIssue,
			Topic:      "product_quality",
			Content:    "Multiple customers reporting slow mobile app performance",
			Confidence: 0.92,
			CreatedAt:  time.Now().Add(-30 * time.Minute),
			Privacy:    types.InsightPrivacyPublic,
		},
	}

	// Apply filters
	var filtered []types.Insight
	for _, insight := range insights {
		// Filter by confidence
		if insight.Confidence < query.MinConfidence {
			continue
		}

		// Filter by topics
		if len(query.Topics) > 0 {
			found := false
			for _, topic := range query.Topics {
				if insight.Topic == topic {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Filter by agent types
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

		filtered = append(filtered, insight)

		// Apply limit
		if query.Limit > 0 && len(filtered) >= query.Limit {
			break
		}
	}

	return filtered, nil
}

// corsMiddleware adds CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
