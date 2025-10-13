# AgentMesh Cortex - Deployment Guide

## Quick Start (5 Minutes)

### 1. Prerequisites

- **Go 1.19+**
- **Docker & Docker Compose**
- **8GB RAM** (for Kafka, Redis, Prometheus, Grafana)

### 2. Clone and Setup

```bash
git clone https://github.com/avinashshinde/agentmesh-cortex.git
cd agentmesh-cortex

# Install dependencies
make deps
```

### 3. Start Infrastructure

```bash
# Starts Kafka, Redis, Prometheus, Grafana
make docker-up

# Verify services
make status
```

**Infrastructure URLs:**
- Kafka: `localhost:9092`
- Redis: `localhost:6379`
- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3000` (admin/admin)

### 4. Choose Deployment Mode

#### Option A: Distributed Mode (Recommended)

**Production-ready deployment with separate processes:**

```bash
make run-distributed
```

This launches 6 processes:
- `topology-manager` - Maintains SlimeMold graph
- `consensus-manager` - Handles Bee voting
- `agent-sales` - Sales agent process
- `agent-support` - Support agent process
- `agent-inventory` - Inventory agent process
- `agent-fraud` - Fraud detection agent process

**Monitor logs:**
```bash
# All logs
tail -f logs/*.log

# Specific agent
tail -f logs/agent-sales.log
```

**Shutdown:**
Press `Ctrl+C` - gracefully stops all processes

#### Option B: Demo Mode (Quick Testing)

**Single-process deployment for demos:**

```bash
make demo
```

Runs all agents in one process (original implementation).

---

## Production Deployment

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     AgentMesh Cortex Cluster                    │
└─────────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
┌───────▼────────┐   ┌───────▼────────┐   ┌───────▼────────┐
│ Manager Layer  │   │  Agent Layer   │   │ Infrastructure │
│                │   │                │   │     Layer      │
│ - Topology Mgr │   │ - Agent Sales  │   │ - Kafka        │
│ - Consensus Mgr│   │ - Agent Support│   │ - Redis        │
│                │   │ - Agent Inv.   │   │ - Prometheus   │
│                │   │ - Agent Fraud  │   │ - Grafana      │
└────────────────┘   └────────────────┘   └────────────────┘
```

### Deployment Steps

#### 1. Build Binaries

```bash
make build-distributed
```

**Output:**
```
bin/
├── agent                 (8.6 MB)
├── topology-manager      (9.1 MB)
└── consensus-manager     (9.1 MB)
```

#### 2. Configure Environment

Copy `.env.example` to `.env` and customize:

```bash
# Topology Configuration
INITIAL_EDGE_WEIGHT=0.5
REINFORCEMENT_AMOUNT=0.1
DECAY_RATE=0.05
DECAY_INTERVAL=5s
PRUNE_THRESHOLD=0.1

# Consensus Configuration
QUORUM_THRESHOLD=0.6
PROPOSAL_TIMEOUT=30s
WAGGLE_INTENSITY_MIN=0.3

# Infrastructure
KAFKA_BROKERS=localhost:9092
REDIS_ADDR=localhost:6379
```

#### 3. Deploy Services

**Option A: Shell Script (Development)**

```bash
./scripts/run-distributed.sh
```

**Option B: Systemd (Production)**

Create service files for each component:

```ini
# /etc/systemd/system/agentmesh-topology.service
[Unit]
Description=AgentMesh Topology Manager
After=network.target kafka.service redis.service

[Service]
Type=simple
User=agentmesh
WorkingDirectory=/opt/agentmesh
ExecStart=/opt/agentmesh/bin/topology-manager
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable agentmesh-topology
sudo systemctl start agentmesh-topology
sudo systemctl status agentmesh-topology
```

**Option C: Docker Compose (Cloud)**

```yaml
# docker-compose.yml
version: '3.8'

services:
  topology-manager:
    build: .
    command: /app/topology-manager
    depends_on:
      - kafka
      - redis
    environment:
      - KAFKA_BROKERS=kafka:9092
      - REDIS_ADDR=redis:6379
    restart: unless-stopped

  consensus-manager:
    build: .
    command: /app/consensus-manager
    depends_on:
      - kafka
      - redis
    restart: unless-stopped

  agent-sales:
    build: .
    command: /app/agent -name=Sales -role=sales -capabilities=order_processing
    depends_on:
      - topology-manager
      - consensus-manager
    restart: unless-stopped

  # ... other agents
```

#### 4. Verify Deployment

```bash
# Check processes
ps aux | grep -E "agent|topology|consensus"

# Check Kafka topics
docker exec -it kafka kafka-topics.sh --list --bootstrap-server localhost:9092

# Expected topics:
# - agentmesh.messages
# - agentmesh.proposals
# - agentmesh.topology
# - agentmesh.votes

# Check Redis keys
docker exec -it redis redis-cli KEYS "*"

# Expected keys:
# - graph:snapshot:latest
# - agent:*
# - proposal:*
```

#### 5. Monitor System

**Prometheus Metrics:**

Visit `http://localhost:9090` and query:
```promql
# Edge count over time
agentmesh_edge_count

# Edge reduction percentage
agentmesh_edge_reduction_percent

# Proposal counts by status
agentmesh_proposal_count
```

**Grafana Dashboard:**

Visit `http://localhost:3000` (admin/admin) and import dashboard from `deployments/grafana/dashboards/agentmesh.json`

**Application Logs:**

```bash
# Topology manager
tail -f logs/topology-manager.log

# Agents
tail -f logs/agent-*.log
```

---

## Scaling

### Horizontal Scaling (Add More Agents)

**1. Deploy new agent instance:**

```bash
./bin/agent -name=Shipping -role=shipping -capabilities=tracking,delivery &
```

**2. Agent automatically:**
- Publishes join event to Kafka
- Topology manager creates edges to all existing agents
- Consensus manager includes in quorum calculations

**3. No changes needed to existing agents!**

### Vertical Scaling (Manager Services)

**Topology Manager:**
- Run multiple instances with Kafka consumer groups
- Each instance handles subset of events
- Redis provides shared state

```bash
# Instance 1
./bin/topology-manager --consumer-group=topo-1 &

# Instance 2
./bin/topology-manager --consumer-group=topo-1 &
```

**Consensus Manager:**
- Similar pattern with consumer groups
- Proposals distributed across instances

---

## Multi-Machine Deployment

### Architecture

```
┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│   Machine A      │     │   Machine B      │     │   Machine C      │
│  (Managers)      │     │  (Agents 1-2)    │     │  (Agents 3-4)    │
├──────────────────┤     ├──────────────────┤     ├──────────────────┤
│ topology-manager │     │ agent-sales      │     │ agent-inventory  │
│ consensus-manager│     │ agent-support    │     │ agent-fraud      │
└──────────────────┘     └──────────────────┘     └──────────────────┘
        │                        │                        │
        └────────────[Kafka Cluster]──────────────────────┘
        └────────────[Redis Cluster]──────────────────────┘
```

### Configuration

**Machine A (10.0.1.10) - Managers:**
```bash
export KAFKA_BROKERS=10.0.1.20:9092,10.0.1.21:9092,10.0.1.22:9092
export REDIS_ADDR=10.0.1.30:6379
./bin/topology-manager &
./bin/consensus-manager &
```

**Machine B (10.0.1.11) - Agents 1-2:**
```bash
export KAFKA_BROKERS=10.0.1.20:9092,10.0.1.21:9092,10.0.1.22:9092
export REDIS_ADDR=10.0.1.30:6379
./bin/agent -name=Sales -role=sales -capabilities=order_processing &
./bin/agent -name=Support -role=support -capabilities=refunds &
```

**Machine C (10.0.1.12) - Agents 3-4:**
```bash
export KAFKA_BROKERS=10.0.1.20:9092,10.0.1.21:9092,10.0.1.22:9092
export REDIS_ADDR=10.0.1.30:6379
./bin/agent -name=Inventory -role=inventory -capabilities=stock_check &
./bin/agent -name=Fraud -role=fraud -capabilities=transaction_verification &
```

---

## Cloud Deployment (AWS)

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                            AWS VPC                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │   ECS Task   │  │   ECS Task   │  │   ECS Task   │         │
│  │  (Managers)  │  │  (Agent 1)   │  │  (Agent 2)   │         │
│  └──────────────┘  └──────────────┘  └──────────────┘         │
│         │                  │                  │                │
│         └──────────────────┼──────────────────┘                │
│                            │                                   │
│  ┌─────────────────────────┼─────────────────────────────┐    │
│  │                    MSK (Kafka)                         │    │
│  │  - 3 brokers (m5.large)                               │    │
│  │  - Multi-AZ deployment                                │    │
│  └────────────────────────────────────────────────────────┘    │
│                            │                                   │
│  ┌─────────────────────────┼─────────────────────────────┐    │
│  │              ElastiCache (Redis)                       │    │
│  │  - cache.t3.medium                                     │    │
│  │  - Cluster mode enabled                               │    │
│  └────────────────────────────────────────────────────────┘    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### ECS Task Definitions

**Topology Manager:**
```json
{
  "family": "agentmesh-topology",
  "containerDefinitions": [{
    "name": "topology-manager",
    "image": "agentmesh/topology-manager:latest",
    "memory": 512,
    "cpu": 256,
    "environment": [
      {"name": "KAFKA_BROKERS", "value": "kafka-1:9092,kafka-2:9092,kafka-3:9092"},
      {"name": "REDIS_ADDR", "value": "redis.cluster.amazonaws.com:6379"}
    ],
    "logConfiguration": {
      "logDriver": "awslogs",
      "options": {
        "awslogs-group": "/ecs/agentmesh-topology",
        "awslogs-region": "us-east-1"
      }
    }
  }]
}
```

**Agent:**
```json
{
  "family": "agentmesh-agent",
  "containerDefinitions": [{
    "name": "agent",
    "image": "agentmesh/agent:latest",
    "memory": 256,
    "cpu": 128,
    "command": ["-name=Sales", "-role=sales", "-capabilities=order_processing"],
    "environment": [
      {"name": "KAFKA_BROKERS", "value": "kafka-1:9092,kafka-2:9092,kafka-3:9092"},
      {"name": "REDIS_ADDR", "value": "redis.cluster.amazonaws.com:6379"}
    ]
  }]
}
```

### Deploy to ECS

```bash
# Build and push Docker images
docker build -t agentmesh/topology-manager:latest -f Dockerfile.topology .
docker push agentmesh/topology-manager:latest

docker build -t agentmesh/agent:latest -f Dockerfile.agent .
docker push agentmesh/agent:latest

# Create ECS services
aws ecs create-service \
  --cluster agentmesh-cluster \
  --service-name topology-manager \
  --task-definition agentmesh-topology \
  --desired-count 1

aws ecs create-service \
  --cluster agentmesh-cluster \
  --service-name agent-sales \
  --task-definition agentmesh-agent \
  --desired-count 1 \
  --overrides '{"containerOverrides":[{"name":"agent","command":["-name=Sales","-role=sales"]}]}'
```

---

## Monitoring & Observability

### Prometheus Metrics

**Topology Metrics:**
```promql
# Total edges in graph
agentmesh_edge_count

# Active edges (weight > 0)
agentmesh_active_edge_count

# Edge reduction percentage
agentmesh_edge_reduction_percent

# Average edge weight
agentmesh_average_edge_weight
```

**Consensus Metrics:**
```promql
# Proposal counts by status
agentmesh_proposal_count{status="pending"}
agentmesh_proposal_count{status="accepted"}
agentmesh_proposal_count{status="rejected"}

# Average quorum score
agentmesh_average_quorum
```

**Agent Metrics:**
```promql
# Total agents in mesh
agentmesh_agent_count

# Messages sent/received
agentmesh_messages_sent_total
agentmesh_messages_received_total
```

### Grafana Dashboard

Import `deployments/grafana/dashboards/agentmesh.json` for:
- Real-time topology visualization
- Edge weight heatmap
- Consensus proposal timeline
- Agent activity metrics

### Distributed Tracing (Future)

Integrate with Jaeger/Zipkin:
```go
// Add trace context to messages
message.Metadata["trace-id"] = traceID
message.Metadata["span-id"] = spanID
```

---

## Troubleshooting

### Common Issues

#### 1. Agents Not Joining Mesh

**Symptom:** `Failed to publish join event` in agent logs

**Debug:**
```bash
# Check Kafka connectivity
docker exec -it kafka kafka-broker-api-versions.sh --bootstrap-server localhost:9092

# Check topic exists
docker exec -it kafka kafka-topics.sh --describe --topic agentmesh.topology --bootstrap-server localhost:9092

# Manually produce test message
docker exec -it kafka kafka-console-producer.sh --topic agentmesh.topology --bootstrap-server localhost:9092
```

#### 2. Topology Manager Not Receiving Events

**Symptom:** No agent join logs in topology-manager.log

**Debug:**
```bash
# Check consumer group
docker exec -it kafka kafka-consumer-groups.sh --bootstrap-server localhost:9092 --describe --group topology-manager

# Manually consume topic
docker exec -it kafka kafka-console-consumer.sh --topic agentmesh.topology --bootstrap-server localhost:9092 --from-beginning
```

#### 3. Consensus Not Reaching Quorum

**Symptom:** Proposals timeout without reaching 60%

**Debug:**
```bash
# Check how many agents are active
redis-cli SCARD agents:active

# Check proposal details
redis-cli HGETALL proposal:PROPOSAL_ID

# Check vote distribution
redis-cli HGETALL proposal:PROPOSAL_ID:votes
```

#### 4. Memory Leak

**Symptom:** Process memory grows over time

**Debug:**
```bash
# Check goroutine leaks
curl http://localhost:6060/debug/pprof/goroutine?debug=1

# Check heap profile
go tool pprof http://localhost:6060/debug/pprof/heap
```

---

## Performance Tuning

### Kafka Optimization

```properties
# config/server.properties
num.network.threads=8
num.io.threads=16
socket.send.buffer.bytes=1048576
socket.receive.buffer.bytes=1048576
log.retention.hours=24
compression.type=lz4
```

### Redis Optimization

```conf
# redis.conf
maxmemory 2gb
maxmemory-policy allkeys-lru
save 900 1
save 300 10
```

### Application Tuning

```bash
# Increase Go GC target
export GOGC=200

# Adjust decay interval for faster convergence
export DECAY_INTERVAL=2s

# Increase reinforcement for stronger signal
export REINFORCEMENT_AMOUNT=0.2
```

---

## Security

### TLS/SSL

**Kafka:**
```properties
# Kafka SSL config
ssl.keystore.location=/path/to/keystore.jks
ssl.keystore.password=password
ssl.truststore.location=/path/to/truststore.jks
ssl.truststore.password=password
```

**Redis:**
```bash
# Connect with TLS
export REDIS_ADDR=rediss://redis.example.com:6380
```

### Authentication

**Kafka SASL:**
```properties
sasl.mechanism=SCRAM-SHA-256
sasl.jaas.config=org.apache.kafka.common.security.scram.ScramLoginModule required username="admin" password="password";
```

**Redis Auth:**
```bash
export REDIS_PASSWORD=your_secure_password
```

---

## Backup & Recovery

### Redis Snapshots

```bash
# Manual backup
redis-cli BGSAVE

# Automatic snapshots (redis.conf)
save 900 1    # After 900 sec if 1 key changed
save 300 10   # After 300 sec if 10 keys changed
save 60 10000 # After 60 sec if 10000 keys changed
```

### Kafka Retention

```properties
# Keep topology events for 7 days
log.retention.hours=168

# Keep message logs for 24 hours
log.retention.hours=24
```

### State Recovery

```bash
# Restore from Redis snapshot
redis-cli --rdb /backup/dump.rdb

# Replay Kafka events from specific offset
./bin/topology-manager --replay-from-offset=12345
```

---

## Next Steps

1. **Run your first distributed deployment**: `make run-distributed`
2. **Monitor in Grafana**: Open http://localhost:3000
3. **Scale horizontally**: Add more agents with `./bin/agent`
4. **Customize agents**: Implement your own agent logic
5. **Deploy to cloud**: Follow AWS/GCP deployment guide

For questions or issues, see:
- [GitHub Issues](https://github.com/avinashshinde/agentmesh-cortex/issues)
- [Architecture Documentation](docs/ARCHITECTURE.md)
- [API Reference](docs/API.md)

**Happy deploying! 🚀**
