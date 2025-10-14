// Initialize application
const port = window.location.port || 8080;
const wsUrl = 'ws://' + window.location.hostname + ':' + port + '/ws';
const wsManager = new WebSocketManager(wsUrl);
const graph = new ForceDirectedGraph('graph');

let stats = {
    agents: 0,
    edges: 0,
    activeEdges: 0,
    avgWeight: 0,
    reduction: 0,
    density: 0,
    proposals: 0,
    quorumCount: 0
};

// Listen to WebSocket messages
wsManager.addListener((data) => {
    switch (data.type) {
        case 'snapshot':
            handleSnapshot(data.snapshot);
            break;
        case 'topology':
            handleTopologyEvent(data.event);
            break;
        case 'consensus':
            handleConsensusEvent(data.event);
            break;
        case 'message':
            // Forward to message stream handler
            if (data.message) {
                addMessage(data.message);
            }
            break;
    }
});

function handleSnapshot(snapshot) {
    if (!snapshot || !snapshot.stats) return;

    stats.agents = snapshot.stats.total_agents;
    stats.edges = snapshot.stats.total_edges;
    stats.activeEdges = snapshot.stats.active_edges;
    stats.avgWeight = snapshot.stats.average_weight;
    stats.reduction = snapshot.stats.reduction_percent;
    stats.density = snapshot.stats.density;

    updateStatsUI();
    graph.update(snapshot);
}

function handleTopologyEvent(event) {
    const typeMap = {
        'agent_joined': 'Agent joined mesh',
        'agent_left': 'Agent left mesh',
        'edge_removed': 'Edge pruned',
        'edge_strength_changed': 'Edge reinforced'
    };

    const message = typeMap[event.type] || event.type;
    wsManager.addEvent(message, 'topology');
}

function handleConsensusEvent(event) {
    const typeMap = {
        'proposal_created': 'Proposal created',
        'proposal_accepted': 'Proposal ACCEPTED',
        'proposal_rejected': 'Proposal rejected',
        'quorum_reached': 'Quorum reached!'
    };

    if (event.type === 'proposal_created') {
        stats.proposals++;
    }
    if (event.type === 'quorum_reached') {
        stats.quorumCount++;
    }

    const message = typeMap[event.type] || event.type;
    wsManager.addEvent(message, 'consensus');
    updateStatsUI();
}

function updateStatsUI() {
    document.getElementById('agent-count').textContent = stats.agents;
    document.getElementById('edge-count').textContent = stats.edges;
    document.getElementById('active-edges').textContent = stats.activeEdges;
    document.getElementById('avg-weight').textContent = stats.avgWeight.toFixed(2);
    document.getElementById('reduction').textContent = stats.reduction.toFixed(1) + '%';
    document.getElementById('density').textContent = (stats.density * 100).toFixed(1) + '%';
    document.getElementById('proposals').textContent = stats.proposals;
    document.getElementById('quorum-count').textContent = stats.quorumCount;
}

// Initial load - fetch from API server which has the real topology from Redis
fetch('http://localhost:8080/api/topology')
    .then(res => res.json())
    .then(topology => {
        // Convert API topology format to snapshot format
        const totalAgents = Object.keys(topology.agents || {}).length;
        const totalEdges = Object.keys(topology.edges || {}).length;
        const activeEdges = Object.values(topology.edges || {}).filter(e => e.weight > 0.1).length;
        const avgWeight = topology.edges ? Object.values(topology.edges).reduce((sum, e) => sum + e.weight, 0) / Object.keys(topology.edges).length : 0;

        // Calculate density and reduction
        const maxPossibleEdges = totalAgents * (totalAgents - 1);
        const density = maxPossibleEdges > 0 ? totalEdges / maxPossibleEdges : 0;
        const reductionPercent = maxPossibleEdges > 0 ? ((maxPossibleEdges - totalEdges) / maxPossibleEdges) * 100 : 0;

        const snapshot = {
            agents: topology.agents || {},
            edges: topology.edges || {},
            stats: {
                total_agents: totalAgents,
                total_edges: totalEdges,
                active_edges: activeEdges,
                average_weight: avgWeight,
                reduction_percent: reductionPercent,
                density: density
            }
        };
        handleSnapshot(snapshot);
    })
    .catch(err => console.error('Failed to load initial snapshot:', err));
