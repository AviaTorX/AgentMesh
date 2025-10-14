# Message Routing Diversity Fix

## Problem
User observed "all messages directed to Sales" in the Web UI message stream.

## Root Cause Analysis

**Previous routing configuration had Sales as a hardcoded target for 3 out of 4 business agents:**

1. **Support** → ALWAYS sent to Sales (escalations)
2. **Inventory** → ALWAYS sent to Sales (stock alerts)
3. **Fraud** → ALWAYS sent to Sales (fraud alerts)
4. **Sales** → Sent to Fraud (transaction verification)

Combined with:
- Research Agent rotating through: sales, support, inventory
- Market Analyst rotating through: sales, inventory, fraud
- Coordinator rotating through: all 6 agents

This created a situation where **Sales received messages from 6 out of 7 agents**, making it the most common target by far.

## Solution

Added rotation logic to business agents to distribute messages more evenly:

### Support Agent (NEW)
**Rotates between:** sales, inventory, fraud

- **sales** → escalate (pricing complaints)
- **inventory** → check_delivery (shipping delays)
- **fraud** → verify_account (suspicious activity)

### Inventory Agent (NEW)
**Rotates between:** sales, support

- **sales** → stock_alert (low stock)
- **support** → delivery_update (delayed shipments)

### Fraud Agent (NEW)
**Rotates between:** sales, support

- **sales** → fraud_alert (medium risk transactions)
- **support** → account_suspension (high risk accounts)

### Sales Agent (UNCHANGED)
**Always sends to:** fraud (transaction verification)

### Multi-Framework Agents (UNCHANGED)
- **Research (OpenAI)**: Rotates through sales, support, inventory
- **Market Analyst (LangChain)**: Rotates through sales, inventory, fraud
- **Coordinator (Anthropic)**: Rotates through all 6 agents

## Expected Outcome

**More balanced message distribution:**
- Sales: Receives from 5 agents (down from 6)
- Support: Receives from 4 agents (up from 1)
- Inventory: Receives from 4 agents (up from 2)
- Fraud: Receives from 3 agents (same)

**Message stream will show:**
- Support → Sales, Inventory, Fraud (rotating)
- Inventory → Sales, Support (alternating)
- Fraud → Sales, Support (alternating)
- Research → Sales, Support, Inventory (rotating)
- Analyst → Sales, Inventory, Fraud (rotating)
- Coordinator → All agents (rotating)
- Sales → Fraud (consistent)

## How to Apply

Run the unified startup script to rebuild and restart with the new routing:

```bash
cd /Users/avinashshinde/PrrProject/agentmesh
./scripts/start-all.sh
```

This will:
1. Kill all running processes
2. Clear Kafka state
3. Rebuild all binaries with new routing logic
4. Start fresh system with balanced message distribution

## Verification

After starting, check the Web UI (http://localhost:8081) message stream. You should now see messages flowing between many different agent pairs, not just "X → Sales".

Example messages you should see:
- "Support → Inventory"
- "Support → Fraud Detection"
- "Inventory → Support"
- "Fraud Detection → Support"
- "Market Analyst → Inventory"
- "Research Agent → Support"

Sales will still receive many messages (it's a central hub), but it won't be the ONLY target anymore.
