class WebSocketManager {
    constructor(url) {
        this.url = url;
        this.ws = null;
        this.reconnectInterval = 3000;
        this.listeners = [];
        this.connect();
    }

    connect() {
        this.ws = new WebSocket(this.url);
        
        this.ws.onopen = () => {
            console.log('WebSocket connected');
            this.addEvent('WebSocket connected', 'system');
        };

        this.ws.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                this.listeners.forEach(listener => listener(data));
            } catch (e) {
                console.error('Failed to parse message:', e);
            }
        };

        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
            this.addEvent('WebSocket error', 'error');
        };

        this.ws.onclose = () => {
            console.log('WebSocket closed, reconnecting...');
            this.addEvent('WebSocket closed, reconnecting...', 'system');
            setTimeout(() => this.connect(), this.reconnectInterval);
        };
    }

    addListener(callback) {
        this.listeners.push(callback);
    }

    addEvent(message, type = 'info') {
        const eventsDiv = document.getElementById('events');
        if (eventsDiv) {
            const event = document.createElement('div');
            event.className = `event ${type}`;
            const timestamp = new Date().toLocaleTimeString();
            event.textContent = `[${timestamp}] ${message}`;
            eventsDiv.prepend(event);
            
            // Keep only last 50 events
            while (eventsDiv.children.length > 50) {
                eventsDiv.removeChild(eventsDiv.lastChild);
            }
        }
    }
}
