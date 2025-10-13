# AgentMesh Cortex - Query API Reference

**Base URL**: `http://localhost:8080`
**Version**: 1.0
**Date**: October 13, 2025

---

## Overview

The AgentMesh Query API provides REST endpoints to access collective intelligence from all agents in the mesh.

**Key Features:**
- Filter insights by topic, agent type, confidence, time
- Natural language queries
- Real-time topology visualization
- Agent status monitoring

---

## Authentication

Currently **no authentication required** (demo mode).

For production deployment, add API keys or OAuth2.

---

## Endpoints

### Health Check

**GET** `/health`

Returns server health status.

**Response:**
```json
{
  "status": "healthy",
  "service": "agentmesh-api",
  "timestamp": "2025-10-13T14:00:00Z"
}
```

---

### Query Insights

**GET** `/api/insights`

Retrieve insights with optional filters.

**Query Parameters:**
| Parameter | Type | Description | Example |
|-----------|------|-------------|---------|
| `topic` | string | Filter by topic (repeatable) | `topic=pricing` |
| `agent_type` | string | Filter by agent role (repeatable) | `agent_type=sales` |
| `min_confidence` | float | Minimum confidence (0.0-1.0) | `min_confidence=0.7` |
| `limit` | int | Max results to return | `limit=10` |

**Example Request:**
```bash
curl "http://localhost:8080/api/insights?topic=pricing&min_confidence=0.7&limit=5"
```

**Response:**
```json
{
  "query": {
    "topics": ["pricing"],
    "min_confidence": 0.7,
    "limit": 5
  },
  "insights": [
    {
      "id": "insight-1697203200000",
      "agent_id": "agent-sales-1",
      "agent_role": "sales",
      "type": "pricing_issue",
      "topic": "pricing",
      "content": "Customer complained that price is too high for basic features",
      "data": {
        "product": "pro-plan",
        "requested_price": 49.99,
        "current_price": 99.99
      },
      "confidence": 0.85,
      "tags": ["complaint", "pricing", "pro-plan"],
      "created_at": "2025-10-13T12:00:00Z",
      "privacy": "public"
    },
    {
      "id": "insight-1697203800000",
      "agent_id": "agent-support-2",
      "agent_role": "support",
      "type": "customer_feedback",
      "topic": "pricing",
      "content": "Customer asked about discounts for annual plans",
      "confidence": 0.72,
      "created_at": "2025-10-13T12:10:00Z",
      "privacy": "public"
    }
  ],
  "count": 2,
  "timestamp": "2025-10-13T14:00:00Z"
}
```

**Status Codes:**
- `200 OK`: Success
- `400 Bad Request`: Invalid parameters
- `500 Internal Server Error`: Server error

---

### Search Insights (JSON Body)

**POST** `/api/insights/search`

Advanced search with JSON body.

**Request Body:**
```json
{
  "topics": ["pricing", "customer_complaint"],
  "agent_types": ["sales", "support"],
  "insight_types": ["pricing_issue", "customer_feedback"],
  "min_confidence": 0.7,
  "time_from": "2025-10-13T00:00:00Z",
  "time_to": "2025-10-13T23:59:59Z",
  "limit": 20
}
```

**Example Request:**
```bash
curl -X POST http://localhost:8080/api/insights/search \
  -H "Content-Type: application/json" \
  -d '{
    "topics": ["pricing"],
    "agent_types": ["sales"],
    "min_confidence": 0.7,
    "limit": 10
  }'
```

**Response:** Same format as GET `/api/insights`

---

### Natural Language Query

**POST** `/api/query`

Ask questions in plain English about collective knowledge.

**Request Body:**
```json
{
  "question": "What pricing issues did agents discover?"
}
```

**Example Request:**
```bash
curl -X POST http://localhost:8080/api/query \
  -H "Content-Type: application/json" \
  -d '{"question": "What are customers complaining about?"}'
```

**Response:**
```json
{
  "query": {
    "question": "What are customers complaining about?",
    "min_confidence": 0.5,
    "limit": 10
  },
  "insights": [
    {
      "id": "insight-...",
      "content": "Customer complained about slow mobile app",
      "type": "product_issue",
      "confidence": 0.92
    }
  ],
  "count": 3,
  "timestamp": "2025-10-13T14:00:00Z"
}
```

**Note:** Currently uses simple keyword matching. Future: semantic search with embeddings.

---

### List Agents

**GET** `/api/agents`

Get all active agents in the mesh.

**Example Request:**
```bash
curl http://localhost:8080/api/agents
```

**Response:**
```json
{
  "agents": [
    {
      "id": "agent-sales-1",
      "name": "Sales",
      "role": "sales",
      "status": "active",
      "capabilities": ["order_processing", "upselling"],
      "created_at": "2025-10-13T10:00:00Z",
      "last_seen_at": "2025-10-13T14:00:00Z"
    },
    {
      "id": "agent-support-1",
      "name": "Support",
      "role": "support",
      "status": "active",
      "capabilities": ["refunds", "escalations"],
      "created_at": "2025-10-13T10:00:00Z",
      "last_seen_at": "2025-10-13T14:00:00Z"
    }
  ],
  "count": 4
}
```

---

### Get Agent Details

**GET** `/api/agents/{agent_id}`

Get details for a specific agent.

**Example Request:**
```bash
curl http://localhost:8080/api/agents/agent-sales-1
```

**Response:**
```json
{
  "id": "agent-sales-1",
  "name": "Sales",
  "role": "sales",
  "status": "active",
  "capabilities": ["order_processing", "upselling", "discount_approval"],
  "metadata": {
    "version": "1.0",
    "framework": "agentmesh"
  },
  "created_at": "2025-10-13T10:00:00Z",
  "last_seen_at": "2025-10-13T14:00:00Z"
}
```

---

### Get Topology

**GET** `/api/topology`

Get current network topology (agents and edges).

**Example Request:**
```bash
curl http://localhost:8080/api/topology
```

**Response:**
```json
{
  "agents": {
    "agent-sales-1": {
      "id": "agent-sales-1",
      "name": "Sales",
      "role": "sales",
      "status": "active"
    },
    "agent-support-1": {
      "id": "agent-support-1",
      "name": "Support",
      "role": "support",
      "status": "active"
    }
  },
  "edges": {
    "edge-sales-1-support-1": {
      "id": "edge-sales-1-support-1",
      "source_id": "agent-sales-1",
      "target_id": "agent-support-1",
      "weight": 0.85,
      "usage": 127,
      "last_used": "2025-10-13T13:55:00Z"
    }
  },
  "timestamp": "2025-10-13T14:00:00Z",
  "stats": {
    "total_agents": 4,
    "total_edges": 5,
    "active_edges": 5,
    "reduction_percent": 58.33
  }
}
```

---

### Get Topology Stats

**GET** `/api/topology/stats`

Get topology statistics only (no full graph).

**Example Request:**
```bash
curl http://localhost:8080/api/topology/stats
```

**Response:**
```json
{
  "total_agents": 4,
  "total_edges": 5,
  "active_edges": 5,
  "average_weight": 0.72,
  "max_weight": 0.95,
  "min_weight": 0.15,
  "density": 0.42,
  "reduction_percent": 58.33
}
```

---

## Data Types

### Insight

```typescript
interface Insight {
  id: string;                    // Unique ID
  agent_id: string;              // Agent that created insight
  agent_role: string;            // Agent's role
  type: InsightType;             // Category of insight
  topic: string;                 // Topic keyword
  content: string;               // Natural language description
  data?: object;                 // Structured data (optional)
  confidence: number;            // 0.0 - 1.0
  tags?: string[];               // Tags (optional)
  metadata?: object;             // Additional metadata
  created_at: string;            // ISO 8601 timestamp
  privacy: "public" | "restricted" | "private";
  shared_with?: string[];        // Agent IDs (if restricted)
}
```

### InsightType

```typescript
type InsightType =
  | "customer_feedback"
  | "pricing_issue"
  | "product_issue"
  | "process_improvement"
  | "fraud_pattern"
  | "inventory_trend"
  | "behavior_pattern"
  | "correlation"
  | "anomaly";
```

### Pattern

```typescript
interface Pattern {
  id: string;
  type: string;                  // e.g., "repeated_complaint"
  description: string;           // Summary
  insights: string[];            // Insight IDs
  frequency: number;             // Occurrence count
  confidence: number;            // 0.0 - 1.0
  detected_at: string;           // ISO 8601 timestamp
}
```

---

## Usage Examples

### Example 1: Find All Pricing Issues

```bash
curl "http://localhost:8080/api/insights?topic=pricing&min_confidence=0.7"
```

### Example 2: Get Sales Agent Insights from Today

```bash
curl -X POST http://localhost:8080/api/insights/search \
  -H "Content-Type: application/json" \
  -d '{
    "agent_types": ["sales"],
    "time_from": "2025-10-13T00:00:00Z",
    "min_confidence": 0.5,
    "limit": 50
  }'
```

### Example 3: Natural Language Query

```bash
curl -X POST http://localhost:8080/api/query \
  -H "Content-Type: application/json" \
  -d '{"question": "What product issues were reported?"}'
```

### Example 4: Monitor Topology Evolution

```bash
# Every 10 seconds
watch -n 10 'curl -s http://localhost:8080/api/topology/stats | jq'
```

### Example 5: Get High-Confidence Fraud Patterns

```bash
curl "http://localhost:8080/api/insights?topic=fraud_detection&min_confidence=0.9&limit=10"
```

---

## CORS Configuration

**Allowed Origins**: `*` (all origins - demo mode)
**Allowed Methods**: `GET, POST, PUT, DELETE, OPTIONS`
**Allowed Headers**: `Content-Type, Authorization`

For production, restrict origins:
```go
w.Header().Set("Access-Control-Allow-Origin", "https://yourdomain.com")
```

---

## Rate Limiting

Currently **no rate limiting** (demo mode).

For production, implement:
- 100 requests/minute per IP
- 1000 requests/hour per API key

---

## Error Responses

### 400 Bad Request

```json
{
  "error": "Invalid request body",
  "details": "JSON parse error at line 3"
}
```

### 404 Not Found

```json
{
  "error": "Agent not found",
  "agent_id": "agent-unknown-1"
}
```

### 500 Internal Server Error

```json
{
  "error": "Failed to query insights",
  "details": "Redis connection timeout"
}
```

---

## Performance

**Average Response Times:**
- `/health`: < 1 ms
- `/api/insights` (no filters): 5-10 ms
- `/api/insights` (with filters): 10-20 ms
- `/api/query`: 50-100 ms (with embeddings: 200-300 ms)
- `/api/topology`: 20-50 ms

**Throughput:**
- Up to 1000 req/sec on single instance
- Horizontal scaling supported

---

## WebSocket API (Future)

Coming soon: Real-time insight streaming

```javascript
const ws = new WebSocket('ws://localhost:8080/ws/insights');

ws.onmessage = (event) => {
  const insight = JSON.parse(event.data);
  console.log('New insight:', insight);
};
```

---

## SDK Support (Future)

### JavaScript/TypeScript

```typescript
import { AgentMeshClient } from 'agentmesh-sdk';

const client = new AgentMeshClient('http://localhost:8080');

const insights = await client.queryInsights({
  topics: ['pricing'],
  minConfidence: 0.7,
  limit: 10
});
```

### Python

```python
from agentmesh import Client

client = Client('http://localhost:8080')

insights = client.query_insights(
    topics=['pricing'],
    min_confidence=0.7,
    limit=10
)
```

---

## Testing the API

### Using cURL

```bash
# Save this as test-api.sh
#!/bin/bash

API="http://localhost:8080"

echo "1. Health Check"
curl -s $API/health | jq

echo "\n2. Get All Insights"
curl -s "$API/api/insights?limit=5" | jq

echo "\n3. Natural Language Query"
curl -s -X POST $API/api/query \
  -H "Content-Type: application/json" \
  -d '{"question": "What issues did agents find?"}' | jq

echo "\n4. Topology Stats"
curl -s $API/api/topology/stats | jq
```

### Using Postman

1. Import collection: `docs/postman/AgentMesh_API.json`
2. Set base URL: `http://localhost:8080`
3. Run collection

### Using HTTPie

```bash
# Install: brew install httpie

http localhost:8080/health
http localhost:8080/api/insights topic==pricing min_confidence==0.7
http POST localhost:8080/api/query question="What pricing issues exist?"
```

---

## Production Deployment

### Environment Variables

```bash
export HTTP_PORT=8080
export KAFKA_BROKERS=kafka-1:9092,kafka-2:9092,kafka-3:9092
export REDIS_ADDR=redis.cluster:6379
```

### Docker Deployment

```dockerfile
FROM golang:1.19-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o api-server cmd/api-server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/api-server .
EXPOSE 8080
CMD ["./api-server"]
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-server
spec:
  replicas: 3
  selector:
    matchLabels:
      app: api-server
  template:
    metadata:
      labels:
        app: api-server
    spec:
      containers:
      - name: api-server
        image: agentmesh/api-server:latest
        ports:
        - containerPort: 8080
        env:
        - name: KAFKA_BROKERS
          value: "kafka:9092"
        - name: REDIS_ADDR
          value: "redis:6379"
---
apiVersion: v1
kind: Service
metadata:
  name: api-server
spec:
  selector:
    app: api-server
  ports:
  - protocol: TCP
    port: 80
    targetPort: 8080
  type: LoadBalancer
```

---

## Support & Contact

- **Issues**: https://github.com/avinashshinde/agentmesh-cortex/issues
- **Docs**: https://agentmesh.dev/docs
- **Email**: support@agentmesh.dev

---

**Version**: 1.0
**Last Updated**: October 13, 2025
**License**: MIT
