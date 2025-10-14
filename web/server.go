package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/avinashshinde/agentmesh-cortex/internal/config"
	"github.com/avinashshinde/agentmesh-cortex/internal/consensus"
	"github.com/avinashshinde/agentmesh-cortex/internal/messaging"
	"github.com/avinashshinde/agentmesh-cortex/internal/topology"
	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for demo
	},
}

type WebSocketHub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan interface{}
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.RWMutex
}

func newHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan interface{}, 100),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

func (h *WebSocketHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.RLock()
			data, _ := json.Marshal(message)
			for client := range h.clients {
				err := client.WriteMessage(websocket.TextMessage, data)
				if err != nil {
					client.Close()
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func main() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	logger.Info("Starting AgentMesh Cortex Web Server")

	cfg := config.Load()
	hub := newHub()
	go hub.run()

	// Initialize backend
	slimeMold := topology.NewSlimeMoldTopology(cfg, logger)
	ctx := context.Background()
	slimeMold.Start(ctx)
	defer slimeMold.Stop()

	beeConsensus := consensus.NewBeeConsensus(cfg, logger)
	beeConsensus.Start(ctx)
	defer beeConsensus.Stop()

	kafkaMessaging := messaging.NewKafkaMessaging(cfg, logger)
	defer kafkaMessaging.Close()

	// Fetch existing agents from API server to handle race condition
	go func() {
		time.Sleep(1 * time.Second) // Wait for API server to be ready
		resp, err := http.Get("http://localhost:8080/api/topology")
		if err == nil {
			defer resp.Body.Close()
			var topologyData struct {
				Agents map[types.AgentID]*types.Agent `json:"agents"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&topologyData); err == nil {
				for _, agent := range topologyData.Agents {
					if err := slimeMold.AddAgent(agent); err == nil {
						logger.Info("Loaded existing agent from API",
							zap.String("agent_id", string(agent.ID)),
							zap.String("name", agent.Name))
					}
				}
			}
		}
	}()

	// Monitor events and broadcast to WebSocket clients
	go func() {
		for event := range slimeMold.EventChannel() {
			hub.broadcast <- map[string]interface{}{
				"type":  "topology",
				"event": event,
			}
		}
	}()

	go func() {
		for event := range beeConsensus.EventChannel() {
			hub.broadcast <- map[string]interface{}{
				"type":  "consensus",
				"event": event,
			}
		}
	}()

	// Listen to Kafka for agent join/leave events
	go func() {
		err := kafkaMessaging.ConsumeTopologyEvents(ctx, "topology", "web-server", func(event types.TopologyEvent) error {
			switch event.Type {
			case types.TopologyEventAgentJoined:
				if event.Agent != nil {
					if err := slimeMold.AddAgent(event.Agent); err != nil {
						logger.Error("Failed to add agent to web topology", zap.Error(err))
					} else {
						logger.Info("Agent added to web topology",
							zap.String("agent_id", string(event.Agent.ID)),
							zap.String("name", event.Agent.Name))
					}
				}
			case types.TopologyEventAgentLeft:
				if err := slimeMold.RemoveAgent(event.AgentID); err != nil {
					logger.Error("Failed to remove agent from web topology", zap.Error(err))
				} else {
					logger.Info("Agent removed from web topology", zap.String("agent_id", string(event.AgentID)))
				}
			}
			return nil
		})
		if err != nil && err != context.Canceled {
			logger.Error("Topology event listener stopped", zap.Error(err))
		}
	}()

	// Listen to Kafka messages and broadcast to WebSocket for live message stream
	// Note: We don't reinforce edges in the web-server's local topology anymore
	// because we fetch the real topology from the API server (Redis-backed)
	go func() {
		err := kafkaMessaging.ConsumeMessages(ctx, "messages", "web-message-stream", func(msg *types.Message) error {
			// Resolve agent names from IDs
			fromName := string(msg.FromAgentID)
			toName := string(msg.ToAgentID)

			graph := slimeMold.GetGraph()
			if fromAgent, err := graph.GetAgent(msg.FromAgentID); err == nil {
				fromName = fromAgent.Name
			}
			if toAgent, err := graph.GetAgent(msg.ToAgentID); err == nil {
				toName = toAgent.Name
			}

			// Broadcast message to all WebSocket clients with agent names
			hub.broadcast <- map[string]interface{}{
				"type": "message",
				"message": map[string]interface{}{
					"from":      msg.FromAgentID,
					"to":        msg.ToAgentID,
					"fromName":  fromName,
					"toName":    toName,
					"type":      msg.Type,
					"payload":   msg.Payload,
					"timestamp": msg.Timestamp,
				},
			}
			return nil
		})
		if err != nil && err != context.Canceled {
			logger.Error("Message stream listener stopped", zap.Error(err))
		}
	}()

	// Periodically broadcast topology snapshot from API server (Redis-backed)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			// Fetch real topology from API server
			resp, err := http.Get("http://localhost:8080/api/topology")
			if err != nil {
				logger.Debug("Failed to fetch topology from API server", zap.Error(err))
				continue
			}
			defer resp.Body.Close()

			var topology struct {
				Agents map[types.AgentID]*types.Agent          `json:"agents"`
				Edges  map[string]map[string]interface{}       `json:"edges"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&topology); err != nil {
				logger.Debug("Failed to decode topology", zap.Error(err))
				continue
			}

			// Calculate stats
			totalAgents := len(topology.Agents)
			totalEdges := len(topology.Edges)
			activeEdges := 0
			var totalWeight float64
			for _, edge := range topology.Edges {
				weight := edge["weight"].(float64)
				if weight > 0.1 {
					activeEdges++
				}
				totalWeight += weight
			}
			avgWeight := 0.0
			if totalEdges > 0 {
				avgWeight = totalWeight / float64(totalEdges)
			}

			// Calculate density and reduction
			maxPossibleEdges := totalAgents * (totalAgents - 1)
			density := 0.0
			reductionPercent := 0.0
			if maxPossibleEdges > 0 {
				density = float64(totalEdges) / float64(maxPossibleEdges)
				reductionPercent = (float64(maxPossibleEdges-totalEdges) / float64(maxPossibleEdges)) * 100.0
			}

			snapshot := map[string]interface{}{
				"agents": topology.Agents,
				"edges":  topology.Edges,
				"stats": map[string]interface{}{
					"total_agents":      totalAgents,
					"total_edges":       totalEdges,
					"active_edges":      activeEdges,
					"average_weight":    avgWeight,
					"reduction_percent": reductionPercent,
					"density":           density,
				},
			}

			hub.broadcast <- map[string]interface{}{
				"type":     "snapshot",
				"snapshot": snapshot,
			}
		}
	}()

	// HTTP handlers
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("WebSocket upgrade failed", zap.Error(err))
			return
		}
		hub.register <- conn
		defer func() {
			hub.unregister <- conn
		}()

		// Keep connection alive
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	})

	http.HandleFunc("/api/snapshot", func(w http.ResponseWriter, r *http.Request) {
		snapshot := slimeMold.GetSnapshot()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(snapshot)
	})

	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/", http.FileServer(http.Dir("web/static")))

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.WebSocketPort),
		Handler: http.DefaultServeMux,
	}

	go func() {
		logger.Info("Web server listening", zap.Int("port", cfg.WebSocketPort))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed", zap.Error(err))
		}
	}()

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	logger.Info("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}
