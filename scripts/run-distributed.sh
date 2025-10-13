#!/bin/bash

# AgentMesh Cortex - Distributed Agent Deployment Script
# Launches 6 separate processes: topology-manager, consensus-manager, 4 agents

set -e

echo "[RUN] Starting AgentMesh Cortex Distributed System..."

# Directory setup
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BIN_DIR="$PROJECT_ROOT/bin"
LOGS_DIR="$PROJECT_ROOT/logs"

mkdir -p "$LOGS_DIR"

# Cleanup function
cleanup() {
    echo ""
    echo "[CLEANUP] Shutting down all processes..."

    if [ -f "$LOGS_DIR/topology-manager.pid" ]; then
        kill $(cat "$LOGS_DIR/topology-manager.pid") 2>/dev/null || true
        rm "$LOGS_DIR/topology-manager.pid"
    fi

    if [ -f "$LOGS_DIR/consensus-manager.pid" ]; then
        kill $(cat "$LOGS_DIR/consensus-manager.pid") 2>/dev/null || true
        rm "$LOGS_DIR/consensus-manager.pid"
    fi

    if [ -f "$LOGS_DIR/knowledge-manager.pid" ]; then
        kill $(cat "$LOGS_DIR/knowledge-manager.pid") 2>/dev/null || true
        rm "$LOGS_DIR/knowledge-manager.pid"
    fi

    if [ -f "$LOGS_DIR/api-server.pid" ]; then
        kill $(cat "$LOGS_DIR/api-server.pid") 2>/dev/null || true
        rm "$LOGS_DIR/api-server.pid"
    fi

    for agent in sales support inventory fraud; do
        if [ -f "$LOGS_DIR/agent-${agent}.pid" ]; then
            kill $(cat "$LOGS_DIR/agent-${agent}.pid") 2>/dev/null || true
            rm "$LOGS_DIR/agent-${agent}.pid"
        fi
    done

    echo "[CLEANUP] All processes stopped"
    exit 0
}

trap cleanup SIGINT SIGTERM

# Check binaries exist
if [ ! -f "$BIN_DIR/topology-manager" ] || [ ! -f "$BIN_DIR/consensus-manager" ] || [ ! -f "$BIN_DIR/agent" ]; then
    echo "[ERROR] Binaries not found. Run: make build-distributed"
    exit 1
fi

# Start topology manager
echo "[START] Topology Manager..."
"$BIN_DIR/topology-manager" > "$LOGS_DIR/topology-manager.log" 2>&1 &
echo $! > "$LOGS_DIR/topology-manager.pid"
sleep 2

# Start consensus manager
echo "[START] Consensus Manager..."
"$BIN_DIR/consensus-manager" > "$LOGS_DIR/consensus-manager.log" 2>&1 &
echo $! > "$LOGS_DIR/consensus-manager.pid"
sleep 2

# Start knowledge manager
echo "[START] Knowledge Manager..."
"$BIN_DIR/knowledge-manager" > "$LOGS_DIR/knowledge-manager.log" 2>&1 &
echo $! > "$LOGS_DIR/knowledge-manager.pid"
sleep 2

# Start API server
echo "[START] API Server (port 8080)..."
"$BIN_DIR/api-server" > "$LOGS_DIR/api-server.log" 2>&1 &
echo $! > "$LOGS_DIR/api-server.pid"
sleep 2

# Start agents
echo "[START] Agent: Sales..."
"$BIN_DIR/agent" -name=Sales -role=sales -capabilities=order_processing,upselling,discount_approval > "$LOGS_DIR/agent-sales.log" 2>&1 &
echo $! > "$LOGS_DIR/agent-sales.pid"
sleep 1

echo "[START] Agent: Support..."
"$BIN_DIR/agent" -name=Support -role=support -capabilities=refunds,escalations,customer_queries > "$LOGS_DIR/agent-support.log" 2>&1 &
echo $! > "$LOGS_DIR/agent-support.pid"
sleep 1

echo "[START] Agent: Inventory..."
"$BIN_DIR/agent" -name=Inventory -role=inventory -capabilities=stock_check,reservation,restock_alerts > "$LOGS_DIR/agent-inventory.log" 2>&1 &
echo $! > "$LOGS_DIR/agent-inventory.pid"
sleep 1

echo "[START] Agent: Fraud..."
"$BIN_DIR/agent" -name=Fraud -role=fraud -capabilities=transaction_verification,blocking,user_checks > "$LOGS_DIR/agent-fraud.log" 2>&1 &
echo $! > "$LOGS_DIR/agent-fraud.pid"
sleep 1

echo ""
echo "[SUCCESS] All processes started!"
echo ""
echo "Manager Services:"
echo "  Topology Manager: PID $(cat "$LOGS_DIR/topology-manager.pid")"
echo "  Consensus Manager: PID $(cat "$LOGS_DIR/consensus-manager.pid")"
echo "  Knowledge Manager: PID $(cat "$LOGS_DIR/knowledge-manager.pid")"
echo "  API Server: PID $(cat "$LOGS_DIR/api-server.pid")"
echo ""
echo "Agents:"
echo "  Sales: PID $(cat "$LOGS_DIR/agent-sales.pid")"
echo "  Support: PID $(cat "$LOGS_DIR/agent-support.pid")"
echo "  Inventory: PID $(cat "$LOGS_DIR/agent-inventory.pid")"
echo "  Fraud: PID $(cat "$LOGS_DIR/agent-fraud.pid")"
echo ""
echo "API Endpoints:"
echo "  http://localhost:8080/health"
echo "  http://localhost:8080/api/insights"
echo "  http://localhost:8080/api/topology"
echo ""
echo "Logs available in: $LOGS_DIR/"
echo "Press Ctrl+C to shutdown all processes"
echo ""

# Wait for interrupt
while true; do
    sleep 1
done
