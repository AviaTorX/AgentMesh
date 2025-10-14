#!/bin/bash
# AgentMesh Cortex - Unified Startup Script
# Kills everything, cleans state, rebuilds, and restarts the entire system

echo "========================================"
echo "AgentMesh Cortex - Full System Restart"
echo "========================================"
echo ""

# Change to project directory
cd /Users/avinashshinde/PrrProject/agentmesh || exit 1

# Step 1: Kill all running processes
echo "🛑 Killing all running processes..."
pkill -9 -f "bin/agent" 2>/dev/null || true
pkill -9 -f "bin/web-server" 2>/dev/null || true
pkill -9 -f "topology-manager" 2>/dev/null || true
pkill -9 -f "consensus-manager" 2>/dev/null || true
pkill -9 -f "knowledge-manager" 2>/dev/null || true
pkill -9 -f "api-server" 2>/dev/null || true
sleep 2
echo "✓ All processes killed"
echo ""

# Step 2: Stop Docker and clear volumes
echo "🐋 Stopping Docker and clearing Kafka state..."
make docker-down 2>/dev/null || true
sleep 2
docker volume prune -f 2>/dev/null || true
echo "✓ Docker stopped and volumes cleared"
echo ""

# Step 3: Rebuild all binaries
echo "🔨 Rebuilding all binaries..."
export PATH="/opt/homebrew/opt/go@1.23/bin:$PATH"

echo "  Building agent..."
go build -o bin/agent cmd/agent/main.go || { echo "❌ Failed to build agent"; exit 1; }

echo "  Building topology-manager..."
go build -o bin/topology-manager cmd/topology-manager/main.go || { echo "❌ Failed to build topology-manager"; exit 1; }

echo "  Building consensus-manager..."
go build -o bin/consensus-manager cmd/consensus-manager/main.go || { echo "❌ Failed to build consensus-manager"; exit 1; }

echo "  Building knowledge-manager..."
go build -o bin/knowledge-manager cmd/knowledge-manager/main.go || { echo "❌ Failed to build knowledge-manager"; exit 1; }

echo "  Building api-server..."
go build -o bin/api-server cmd/api-server/main.go || { echo "❌ Failed to build api-server"; exit 1; }

echo "  Building web-server..."
go build -o bin/web-server web/server.go || { echo "❌ Failed to build web-server"; exit 1; }

echo "✓ All binaries built"
echo ""

# Step 4: Create logs directory
mkdir -p logs
echo "✓ Logs directory ready"
echo ""

# Step 5: Start Docker infrastructure
echo "🚀 Starting Docker infrastructure..."
make docker-up
echo "  Waiting for Kafka to be ready..."
sleep 15

# Wait for Kafka to be truly ready
echo "  Verifying Kafka connectivity..."
MAX_RETRIES=30
RETRY_COUNT=0
while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if docker exec agentmesh-kafka kafka-topics --list --bootstrap-server localhost:9092 >/dev/null 2>&1; then
        echo "  ✓ Kafka is ready"
        break
    fi
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
        echo "  ✗ Kafka failed to start after $MAX_RETRIES attempts"
        echo "  Trying to continue anyway..."
        break
    fi
    sleep 1
done

# Create Kafka topics explicitly to avoid race conditions
echo "  Creating Kafka topics..."
docker exec agentmesh-kafka kafka-topics --create --if-not-exists --bootstrap-server localhost:9092 --topic agentmesh.topology --partitions 3 --replication-factor 1 2>/dev/null || true
docker exec agentmesh-kafka kafka-topics --create --if-not-exists --bootstrap-server localhost:9092 --topic agentmesh.messages --partitions 3 --replication-factor 1 2>/dev/null || true
docker exec agentmesh-kafka kafka-topics --create --if-not-exists --bootstrap-server localhost:9092 --topic agentmesh.consensus --partitions 3 --replication-factor 1 2>/dev/null || true
sleep 2
echo "✓ Docker infrastructure ready"
echo ""

# Step 6: Start backend managers
echo "🎯 Starting backend managers..."
./bin/topology-manager > logs/topology-manager.log 2>&1 &
TOPO_PID=$!
echo "  Started topology-manager (PID: $TOPO_PID)"

./bin/consensus-manager > logs/consensus-manager.log 2>&1 &
echo "  Started consensus-manager (PID: $!)"

./bin/knowledge-manager > logs/knowledge-manager.log 2>&1 &
echo "  Started knowledge-manager (PID: $!)"

./bin/api-server > logs/api-server.log 2>&1 &
API_PID=$!
echo "  Started api-server (PID: $API_PID)"

./bin/web-server > logs/web-ui.log 2>&1 &
WEB_PID=$!
echo "  Started web-server (PID: $WEB_PID)"

echo "  Waiting 8 seconds for managers to initialize..."
sleep 8

# Verify managers started successfully
if ! ps -p $TOPO_PID > /dev/null 2>&1; then
    echo "  ⚠️  Warning: topology-manager may have crashed. Check logs/topology-manager.log"
fi
if ! ps -p $API_PID > /dev/null 2>&1; then
    echo "  ⚠️  Warning: api-server may have crashed. Check logs/api-server.log"
fi
if ! ps -p $WEB_PID > /dev/null 2>&1; then
    echo "  ⚠️  Warning: web-server may have crashed. Check logs/web-ui.log"
fi

echo "✓ All managers started"
echo ""

# Step 7: Start all agents with retry logic
echo "🤖 Starting all agents..."

# Function to start agent with retry
start_agent() {
    local name=$1
    local role=$2
    local capabilities=$3
    local metadata=$4
    local logfile=$5

    for i in {1..3}; do
        ./bin/agent -name="$name" -role=$role -capabilities=$capabilities -metadata="$metadata" > $logfile 2>&1 &
        local pid=$!
        sleep 2

        # Check if agent is still running
        if ps -p $pid > /dev/null 2>&1; then
            echo "  ✓ Started $name (PID: $pid)"
            return 0
        else
            if [ $i -lt 3 ]; then
                echo "  ⚠️  $name failed to start, retrying ($i/3)..."
                sleep 2
            fi
        fi
    done

    echo "  ✗ Failed to start $name after 3 attempts. Check $logfile"
    return 1
}

start_agent "Sales" "sales" "order_processing,upselling,discount_approval" "framework:native,language:go" "logs/agent-sales.log"
start_agent "Support" "support" "refunds,escalations,customer_queries" "framework:native,language:go" "logs/agent-support.log"
start_agent "Inventory" "inventory" "stock_check,reservation,restock_alerts" "framework:native,language:go" "logs/agent-inventory.log"
start_agent "Fraud Detection" "fraud" "transaction_verification,blocking,user_checks" "framework:native,language:go" "logs/agent-fraud.log"
start_agent "Research Agent" "research" "web_search,data_analysis,report_generation" "framework:openai,model:gpt-4,api:assistant" "logs/agent-research.log"
start_agent "Market Analyst" "analyst" "market_research,trend_analysis,forecasting" "framework:langchain,llm:gpt-4,chain:ConversationalRetrievalChain" "logs/agent-analyst.log"
start_agent "Coordinator" "coordinator" "coordination,synthesis,decision_making" "framework:anthropic,model:claude-3-sonnet" "logs/agent-coordinator.log"

echo ""
echo "  Waiting 5 seconds for agents to register..."
sleep 5
echo "✓ All agents started"
echo ""

# Step 8: Verify system health
echo "🔍 Verifying system health..."
AGENT_COUNT=$(ps aux | grep "bin/agent" | grep -v grep | wc -l | tr -d ' ')
echo "  Running agents: $AGENT_COUNT/7"

if [ "$AGENT_COUNT" -lt 7 ]; then
    echo "  ⚠️  Warning: Not all agents started successfully"
    echo "  Check log files in logs/ directory"
fi

# Wait for API to be ready
for i in {1..10}; do
    if curl -s http://localhost:8080/health >/dev/null 2>&1; then
        echo "  ✓ API server is responding"
        break
    fi
    sleep 1
done

echo ""
echo "========================================"
echo "✅ SYSTEM READY"
echo "========================================"
echo ""
echo "🌐 Web UI:        http://localhost:8081"
echo "📊 API:           http://localhost:8080"
echo "📈 Metrics:       http://localhost:8081/metrics"
echo ""
echo "📁 Logs Location: /Users/avinashshinde/PrrProject/agentmesh/logs/"
echo ""
echo "To monitor logs:"
echo "  tail -f logs/topology-manager.log"
echo "  tail -f logs/agent-sales.log"
echo "  tail -f logs/web-ui.log"
echo ""
echo "To stop everything:"
echo "  pkill -f 'bin/agent|bin/web-server|topology-manager|consensus-manager|knowledge-manager|api-server'"
echo "  make docker-down"
echo ""

# Step 9: Open browser
echo "🌐 Opening browser..."
if command -v open >/dev/null 2>&1; then
    open http://localhost:8081
elif command -v xdg-open >/dev/null 2>&1; then
    xdg-open http://localhost:8081
else
    echo "  Please open http://localhost:8081 in your browser"
fi

echo ""
echo "✨ AgentMesh Cortex is running!"
