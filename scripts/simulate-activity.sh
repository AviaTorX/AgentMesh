#!/bin/bash

# AgentMesh Cortex - Background Activity Simulator
# Generates realistic agent message traffic for topology optimization

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "[SIMULATOR] Starting background activity..."
echo "[SIMULATOR] This creates message traffic so SlimeMold has data to optimize"
echo ""

# Counter for varied scenarios
COUNTER=0

while true; do
    COUNTER=$((COUNTER + 1))

    # Scenario 1: E-commerce order flow (frequent)
    if [ $((COUNTER % 3)) -eq 0 ]; then
        # Simulate customer order - creates Sales → Inventory → Fraud communication chain
        ORDER_ID="ORD-$(date +%s)"

        # This would normally be API calls to agents, but since we don't have
        # a dedicated simulation API, we log it for demonstration
        # In production, agents listen to these events via Kafka

        echo "[ACTIVITY] E-commerce order: $ORDER_ID" >> "$PROJECT_ROOT/logs/simulator.log"
    fi

    # Scenario 2: Support ticket (moderate)
    if [ $((COUNTER % 5)) -eq 0 ]; then
        TICKET_ID="TKT-$(date +%s)"
        echo "[ACTIVITY] Support ticket: $TICKET_ID" >> "$PROJECT_ROOT/logs/simulator.log"
    fi

    # Scenario 3: Fraud alert (less frequent)
    if [ $((COUNTER % 8)) -eq 0 ]; then
        ALERT_ID="FRD-$(date +%s)"
        echo "[ACTIVITY] Fraud alert: $ALERT_ID" >> "$PROJECT_ROOT/logs/simulator.log"
    fi

    # Scenario 4: Multi-framework collaboration (periodic)
    if [ $((COUNTER % 10)) -eq 0 ]; then
        echo "[ACTIVITY] Multi-framework pricing analysis triggered" >> "$PROJECT_ROOT/logs/simulator.log"
    fi

    # Brief status update every 30 seconds
    if [ $((COUNTER % 30)) -eq 0 ]; then
        ELAPSED=$((COUNTER * 5))
        echo "[SIMULATOR] Running for ${ELAPSED}s - Generated $((COUNTER / 3)) orders, $((COUNTER / 5)) tickets, $((COUNTER / 8)) alerts"
    fi

    # Wait 5 seconds between simulations
    sleep 5
done
