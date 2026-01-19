/**
 * WebSocket client for real-time updates
 */
class WebSocketClient {
  constructor(basePath = '') {
    this.basePath = basePath;
    this.ws = null;
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 10;
    this.reconnectDelay = 1000;
    this.listeners = new Map();
    this.isConnected = false;
    this.shouldReconnect = true;
  }

  connect() {
    if (this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)) {
      return;
    }

    this.shouldReconnect = true;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    // Ensure basePath ends with '/' for proper URL construction
    let basePath = this.basePath || '';
    if (basePath && !basePath.endsWith('/')) {
      basePath += '/';
    }
    const wsUrl = `${protocol}//${window.location.host}${basePath}ws`;
    
    console.log('WebSocket connecting to:', wsUrl, 'basePath:', this.basePath);
    
    try {
      this.ws = new WebSocket(wsUrl);
      
      this.ws.onopen = () => {
        console.log('WebSocket connected');
        this.isConnected = true;
        this.reconnectAttempts = 0;
        this.emit('connected');
      };

      this.ws.onmessage = (event) => {
        try {
          // Validate message size (prevent memory issues)
          const maxMessageSize = 10 * 1024 * 1024; // 10MB
          if (event.data && event.data.length > maxMessageSize) {
            console.error('WebSocket message too large:', event.data.length, 'bytes');
            this.ws.close();
            return;
          }
          
          const message = JSON.parse(event.data);
          if (!message || typeof message !== 'object') {
            console.error('Invalid WebSocket message format');
            return;
          }
          
          this.handleMessage(message);
        } catch (e) {
          console.error('Failed to parse WebSocket message:', e);
        }
      };

      this.ws.onerror = (error) => {
        console.error('WebSocket error:', error);
        this.emit('error', error);
      };

      this.ws.onclose = () => {
        console.log('WebSocket disconnected');
        this.isConnected = false;
        this.emit('disconnected');
        
        if (this.shouldReconnect && this.reconnectAttempts < this.maxReconnectAttempts) {
          this.reconnectAttempts++;
          const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);
          console.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts})`);
          setTimeout(() => this.connect(), delay);
        }
      };
    } catch (e) {
      console.error('Failed to create WebSocket connection:', e);
      this.emit('error', e);
    }
  }

  handleMessage(message) {
    const { type, payload, time } = message;
    
    // Emit to specific type listeners
    this.emit(type, payload, time);
    
    // Emit to all listeners
    this.emit('message', { type, payload, time });
  }

  on(event, callback) {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, []);
    }
    const callbacks = this.listeners.get(event);
    if (!callbacks.includes(callback)) {
      callbacks.push(callback);
    }
  }

  off(event, callback) {
    if (!this.listeners.has(event)) {
      return;
    }
    const callbacks = this.listeners.get(event);
    const index = callbacks.indexOf(callback);
    if (index > -1) {
      callbacks.splice(index, 1);
    }
  }

  emit(event, ...args) {
    if (this.listeners.has(event)) {
      this.listeners.get(event).forEach(callback => {
        try {
          callback(...args);
        } catch (e) {
          console.error('Error in WebSocket event handler:', e);
        }
      });
    }
  }

  disconnect() {
    this.shouldReconnect = false;
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  send(data) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data));
    } else {
      console.warn('WebSocket is not connected');
    }
  }
}

// Create global WebSocket client instance
// Safely get basePath from global scope (defined in page.html)
window.wsClient = new WebSocketClient(typeof basePath !== 'undefined' ? basePath : '');
