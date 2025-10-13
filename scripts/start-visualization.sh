#!/bin/bash

# AgentMesh Cortex - Web UI Visualization Launcher
# Starts the D3.js visualization dashboard

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "   AgentMesh Cortex - Web UI Visualization"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Set Go path
export PATH="/opt/homebrew/opt/go@1.23/bin:$PATH"

# Build web server if needed
cd "$PROJECT_ROOT"

if [ ! -f "bin/web-server" ]; then
    echo "[BUILD] Building web server..."
    go build -o bin/web-server web/server.go
    echo "✅ Build complete"
    echo ""
fi

# Start web server
echo "[START] Starting web UI on http://localhost:8081..."
echo ""
echo "💡 Open your browser to http://localhost:8081"
echo ""
echo "What you'll see:"
echo "  - Live D3.js network topology graph"
echo "  - Real-time statistics panel"
echo "  - Agent activity event log"
echo "  - SlimeMold optimization in action"
echo ""
echo "Press Ctrl+C to stop..."
echo ""

./bin/web-server
