type WebSocketListener = (...args: unknown[]) => void;

interface WebSocketMessage {
  type: string;
  payload?: unknown;
  time?: unknown;
}

export class WebSocketClient {
  static #MAX_PAYLOAD_BYTES = 10 * 1024 * 1024;
  static #BASE_RECONNECT_MS = 1000;
  static #MAX_RECONNECT_MS = 30_000;
  static #SLOW_RETRY_MS = 60_000;

  basePath: string;
  maxReconnectAttempts: number;
  reconnectAttempts: number;
  isConnected: boolean;

  private ws: WebSocket | null;
  private shouldReconnect: boolean;
  private reconnectTimer: ReturnType<typeof setTimeout> | null;
  private listeners: Map<string, Set<WebSocketListener>>;

  constructor(basePath = '') {
    this.basePath = basePath;
    this.maxReconnectAttempts = 10;
    this.reconnectAttempts = 0;
    this.isConnected = false;

    this.ws = null;
    this.shouldReconnect = true;
    this.reconnectTimer = null;
    this.listeners = new Map();
  }

  connect(): void {
    if (this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)) {
      return;
    }
    this.shouldReconnect = true;
    this.#cancelReconnect();
    this.#openSocket();
  }

  disconnect(): void {
    this.shouldReconnect = false;
    this.#cancelReconnect();
    this.reconnectAttempts = 0;
    if (this.ws) {
      try { this.ws.close(1000, 'client disconnect'); } catch {}
      this.ws = null;
    }
    this.isConnected = false;
  }

  on(event: string, callback: WebSocketListener): void {
    if (typeof callback !== 'function') return;
    let set = this.listeners.get(event);
    if (!set) {
      set = new Set();
      this.listeners.set(event, set);
    }
    set.add(callback);
  }

  off(event: string, callback: WebSocketListener): void {
    const set = this.listeners.get(event);
    if (!set) return;
    set.delete(callback);
    if (set.size === 0) this.listeners.delete(event);
  }

  send(data: unknown): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data));
    }
  }

  #openSocket(): void {
    const url = this.#buildUrl();
    let socket: WebSocket;
    try {
      socket = new WebSocket(url);
    } catch (err) {
      console.error('WebSocket: failed to construct connection', err);
      this.#emit('error', err);
      this.#scheduleReconnect();
      return;
    }
    this.ws = socket;

    socket.addEventListener('open', () => {
      if (this.ws !== socket) return;
      this.isConnected = true;
      this.reconnectAttempts = 0;
      this.#emit('connected');
    });

    socket.addEventListener('message', (event) => {
      if (this.ws !== socket) return;
      this.#onMessage(event);
    });

    socket.addEventListener('error', (event) => {
      if (this.ws !== socket) return;
      this.#emit('error', event);
    });

    socket.addEventListener('close', () => {
      if (this.ws !== socket) return;
      this.isConnected = false;
      this.ws = null;
      this.#emit('disconnected');
      if (this.shouldReconnect) this.#scheduleReconnect();
    });
  }

  #buildUrl(): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    let basePath = this.basePath || '/';
    if (!basePath.startsWith('/')) basePath = '/' + basePath;
    if (!basePath.endsWith('/')) basePath += '/';
    return `${protocol}//${window.location.host}${basePath}ws`;
  }

  #onMessage(event: MessageEvent): void {
    const data = event.data;
    if (typeof data === 'string') {
      const byteLen = new Blob([data]).size;
      if (byteLen > WebSocketClient.#MAX_PAYLOAD_BYTES) {
        console.error(`WebSocket: payload too large (${byteLen} bytes), closing`);
        try { this.ws?.close(1009, 'message too big'); } catch {}
        return;
      }
    }
    let message: unknown;
    try {
      message = JSON.parse(typeof data === 'string' ? data : '');
    } catch (err) {
      console.error('WebSocket: invalid JSON message', err);
      return;
    }
    if (!message || typeof message !== 'object' || typeof (message as { type?: unknown }).type !== 'string') {
      console.error('WebSocket: malformed message envelope');
      return;
    }
    const msg = message as WebSocketMessage;
    this.#emit(msg.type, msg.payload, msg.time);
    this.#emit('message', msg);
  }

  #emit(event: string, ...args: unknown[]): void {
    const set = this.listeners.get(event);
    if (!set) return;
    for (const callback of set) {
      try {
        callback(...args);
      } catch (err) {
        console.error(`WebSocket: handler for "${event}" threw`, err);
      }
    }
  }

  #scheduleReconnect(): void {
    if (!this.shouldReconnect) return;
    this.#cancelReconnect();

    let base: number;
    if (this.reconnectAttempts < this.maxReconnectAttempts) {
      this.reconnectAttempts += 1;
      const exp = WebSocketClient.#BASE_RECONNECT_MS * 2 ** (this.reconnectAttempts - 1);
      base = Math.min(WebSocketClient.#MAX_RECONNECT_MS, exp);
    } else {
      base = WebSocketClient.#SLOW_RETRY_MS;
    }
    const delay = base * (0.75 + Math.random() * 0.5);

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      if (!this.shouldReconnect) return;
      this.#openSocket();
    }, delay);
  }

  #cancelReconnect(): void {
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }
}
