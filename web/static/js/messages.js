// Message Stream Management
let messagesPaused = false;
let messageBuffer = [];
const MAX_MESSAGES = 50;

// Toggle pause state
function togglePause() {
    messagesPaused = !messagesPaused;
    const btn = document.getElementById('pause-log');
    if (messagesPaused) {
        btn.textContent = '▶️ Resume';
        btn.style.background = 'rgba(76, 175, 80, 0.3)';
    } else {
        btn.textContent = '⏸️ Pause';
        btn.style.background = 'rgba(0, 212, 255, 0.2)';
    }
}

// Clear event log
function clearEventLog() {
    messageBuffer = [];
    document.getElementById('events').innerHTML = '';
    updateMessageCount();
}

// Update message counter
function updateMessageCount() {
    document.getElementById('message-count').textContent = messageBuffer.length;
}

// Format timestamp
function formatTime(timestamp) {
    const date = new Date(timestamp);
    return date.toLocaleTimeString('en-US', {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false
    });
}

// Get message type badge HTML
function getMessageTypeBadge(type) {
    const typeMap = {
        'task': 'TASK',
        'response': 'RESP',
        'vote': 'VOTE',
        'heartbeat': 'BEAT',
        'proposal': 'PROP',
        'consensus': 'CONS'
    };
    const label = typeMap[type] || type.toUpperCase();
    return `<span class="message-type ${type}">${label}</span>`;
}

// Get agent display name
function getAgentName(agentId) {
    if (!agentId) return 'Unknown';

    // Extract name from ID (format: "agent-sales-abc123" -> "Sales")
    const parts = agentId.split('-');
    if (parts.length >= 2) {
        return parts[1].charAt(0).toUpperCase() + parts[1].slice(1);
    }
    return agentId.substring(0, 12); // Fallback to truncated ID
}

// Format message content for display
function formatMessageContent(message) {
    const payload = message.payload || {};

    // If there's a description field, use it
    if (payload.description) {
        return payload.description;
    }

    // Generate human-readable content based on message type
    if (message.type === 'task') {
        const action = payload.action || 'unknown';
        const data = payload.data ? JSON.stringify(payload.data).substring(0, 50) : '';
        return `Action: ${action} ${data ? '- ' + data : ''}`;
    }

    if (message.type === 'response') {
        const status = payload.status || 'unknown';
        const result = payload.result ? JSON.stringify(payload.result).substring(0, 50) : '';
        return `Status: ${status} ${result ? '- ' + result : ''}`;
    }

    if (message.type === 'vote') {
        const proposalId = payload.proposal_id || '';
        const support = payload.support ? 'YES' : 'NO';
        return `Vote ${support} on proposal ${proposalId.substring(0, 8)}`;
    }

    if (message.type === 'heartbeat') {
        return 'Heartbeat ping';
    }

    // Fallback: show first 100 chars of payload
    return JSON.stringify(payload).substring(0, 100);
}

// Create message DOM element
function createMessageElement(message) {
    const div = document.createElement('div');
    div.className = 'message-event';

    // Use fromName/toName if provided by server, otherwise parse from ID
    const fromName = message.fromName || getAgentName(message.from);
    const toName = message.toName || getAgentName(message.to);
    const time = formatTime(message.timestamp);
    const typeBadge = getMessageTypeBadge(message.type);
    const content = formatMessageContent(message);

    div.innerHTML = `
        <div class="message-header">
            <div class="message-route">
                <span class="message-from">${fromName}</span>
                <span class="message-arrow">→</span>
                <span class="message-to">${toName}</span>
            </div>
            <span class="message-time">${time}</span>
        </div>
        <div class="message-content">
            ${typeBadge}
            ${content}
        </div>
    `;

    return div;
}

// Add message to display
function addMessage(message) {
    if (messagesPaused) return;

    // Add to buffer
    messageBuffer.push(message);

    // Keep only last MAX_MESSAGES
    if (messageBuffer.length > MAX_MESSAGES) {
        messageBuffer.shift();

        // Remove oldest from DOM
        const eventsDiv = document.getElementById('events');
        if (eventsDiv.firstChild) {
            eventsDiv.removeChild(eventsDiv.firstChild);
        }
    }

    // Add to DOM
    const messageEl = createMessageElement(message);
    const eventsDiv = document.getElementById('events');
    eventsDiv.appendChild(messageEl);

    // Auto-scroll to bottom
    eventsDiv.scrollTop = eventsDiv.scrollHeight;

    // Update counter
    updateMessageCount();
}

// Note: addMessage() is called from app.js when WebSocket receives message events
