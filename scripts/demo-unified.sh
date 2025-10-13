#!/bin/bash

# AgentMesh Cortex - Unified Demo Launcher
# Starts EVERYTHING for one cohesive demonstration

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "   AgentMesh Cortex - Unified Demo System"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "This demo will start:"
echo "  1. Distributed System (8 processes)"
echo "  2. Web UI Visualization (D3.js)"
echo "  3. Multi-Framework Agents (OpenAI + LangChain + Native)"
echo "  4. Background Activity Simulation"
echo ""
echo "All components work together in ONE unified mesh!"
echo ""

# Check prerequisites
echo "[CHECK] Verifying prerequisites..."

if ! command -v docker &> /dev/null; then
    echo "❌ Docker not found. Please install Docker first."
    exit 1
fi

if ! docker ps &> /dev/null; then
    echo "❌ Docker is not running. Please start Docker first."
    exit 1
fi

# Check if infrastructure is running
if ! docker ps | grep -q "agentmesh-kafka"; then
    echo ""
    echo "⚠️  Infrastructure not running. Starting with 'make docker-up'..."
    echo ""
    cd "$PROJECT_ROOT" && make docker-up
    echo ""
    echo "[WAIT] Waiting 30 seconds for infrastructure to initialize..."
    sleep 30
fi

echo "✅ Prerequisites OK"
echo ""

# Build binaries if needed
echo "[BUILD] Building binaries..."
cd "$PROJECT_ROOT"
export PATH="/opt/homebrew/opt/go@1.23/bin:$PATH"

if [ ! -f "bin/agent" ] || [ ! -f "bin/topology-manager" ]; then
    make build-distributed
fi

if [ ! -f "bin/multi-framework-demo" ]; then
    go build -o bin/multi-framework-demo examples/multi_framework_demo.go
fi

echo "✅ Binaries ready"
echo ""

# Start distributed system
echo "[START] Launching distributed system (8 processes)..."
"$SCRIPT_DIR/run-distributed.sh" &
DISTRIBUTED_PID=$!

sleep 8

# Start web visualization
echo "[START] Launching web UI visualization..."
cd "$PROJECT_ROOT"
go run web/server.go > logs/web-ui.log 2>&1 &
WEB_PID=$!

sleep 3

# Start multi-framework agents
echo "[START] Launching multi-framework agents..."
./bin/multi-framework-demo > logs/multi-framework.log 2>&1 &
MULTI_PID=$!

sleep 2

# Start background activity simulation
echo "[START] Starting background activity simulation..."
"$SCRIPT_DIR/simulate-activity.sh" &
SIM_PID=$!

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ UNIFIED DEMO RUNNING!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "🎨 WEB VISUALIZATION:"
echo "   http://localhost:8081"
echo ""
echo "🔌 REST API:"
echo "   http://localhost:8080/health"
echo "   http://localhost:8080/api/insights"
echo "   http://localhost:8080/api/topology"
echo ""
echo "📊 MONITORING:"
echo "   Grafana:    http://localhost:3000 (admin/admin)"
echo "   Prometheus: http://localhost:9090"
echo ""
echo "📝 LOGS:"
echo "   logs/web-ui.log"
echo "   logs/multi-framework.log"
echo "   logs/agent-*.log"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "💡 WHAT YOU'RE SEEING:"
echo "   - 6 agents from 3 frameworks collaborating"
echo "   - SlimeMold topology optimizing in real-time"
echo "   - Collective intelligence emerging"
echo "   - All in ONE unified system!"
echo ""
echo "🎬 Perfect for video recording!"
echo ""
echo "Press Ctrl+C to stop all processes..."
echo ""

# Open browser automatically (macOS)
if [[ "$OSTYPE" == "darwin"* ]]; then
    sleep 2
    open http://localhost:8081 2>/dev/null || true
fi

# Wait for interrupt
trap "echo ''; echo '[SHUTDOWN] Stopping all processes...'; kill $DISTRIBUTED_PID $WEB_PID $MULTI_PID $SIM_PID 2>/dev/null; exit 0" INT TERM

# Keep script running
wait
