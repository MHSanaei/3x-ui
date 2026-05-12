/**
 * WebSocket client for real-time panel updates.
 *
 * Public API (kept stable for index.html / inbounds.html / xray.html):
 *   - connect()                     — open the connection (idempotent)
 *   - disconnect()                  — close and stop reconnecting
 *   - on(event, callback)           — subscribe to event
 *   - off(event, callback)          — unsubscribe
 *   - send(data)                    — send JSON to the server
 *   - isConnected                   — boolean, current state
 *   - reconnectAttempts             — number, attempts since last success
 *   - maxReconnectAttempts          — number, give-up threshold
 *
 * Built-in events:
 *   'connected', 'disconnected', 'error', 'message',
 *   plus any server-emitted message type (status, traffic, client_stats, ...).
 */
export class WebSocketClient {
  static #MAX_PAYLOAD_BYTES = 10 * 1024 * 1024; // 10 MB, mirrors hub maxMessageSize.
  static #BASE_RECONNECT_MS = 1000;
  static #MAX_RECONNECT_MS = 30_000;
  // After exhausting maxReconnectAttempts we switch to a polite slow-retry
  // cadence rather than giving up forever — a panel that recovers an hour
  // later should reconnect without a manual page reload.
  static #SLOW_RETRY_MS = 60_000;

  constructor(basePath = '') {
    this.basePath = basePath;
    this.maxReconnectAttempts = 10;
    this.reconnectAttempts = 0;
    this.isConnected = false;

    this.ws = null;
    this.shouldReconnect = true;
    this.reconnectTimer = null;
    this.listeners = new Map(); // event → Set<callback>
  }

  // Open the connection. Safe to call repeatedly — no-op if already
  // open/connecting. Re-enables reconnects if previously disabled. Cancels
  // any pending reconnect timer so an external connect() can't race a
  // delayed retry into spawning a second socket.
  connect() {
    if (this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)) {
      return;
    }
    this.shouldReconnect = true;
    this.#cancelReconnect();
    this.#openSocket();
  }

  // Close the connection and stop any pending reconnect attempt. Resets the
  // attempt counter so a future connect() starts fresh from the small backoff.
  disconnect() {
    this.shouldReconnect = false;
    this.#cancelReconnect();
    this.reconnectAttempts = 0;
    if (this.ws) {
      try { this.ws.close(1000, 'client disconnect'); } catch { /* ignore */ }
      this.ws = null;
    }
    this.isConnected = false;
  }

  // Subscribe to an event. Re-subscribing the same callback is a no-op.
  on(event, callback) {
    if (typeof callback !== 'function') return;
    let set = this.listeners.get(event);
    if (!set) {
      set = new Set();
      this.listeners.set(event, set);
    }
    set.add(callback);
  }

  // Unsubscribe from an event.
  off(event, callback) {
    const set = this.listeners.get(event);
    if (!set) return;
    set.delete(callback);
    if (set.size === 0) this.listeners.delete(event);
  }

  // Send JSON to the server. Drops silently if not connected — callers
  // should rely on connect()/server pushes rather than client-initiated sends.
  send(data) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data));
    }
  }

  // ───── internals ─────

  #openSocket() {
    const url = this.#buildUrl();
    let socket;
    try {
      socket = new WebSocket(url);
    } catch (err) {
      console.error('WebSocket: failed to construct connection', err);
      this.#emit('error', err);
      this.#scheduleReconnect();
      return;
    }
    this.ws = socket;

    // Every handler must check `this.ws !== socket` first. A previous socket
    // can still fire events (especially `close`) after we've moved on to a
    // new one — e.g. connect() called while the old socket is in CLOSING
    // state. Without the guard, a stale close would null out the freshly
    // opened socket and silently break send().
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
      // Browsers fire 'error' before 'close' on failure. We surface it for
      // consumers (so polling fallbacks can engage) but don't log every blip
      // — bad networks would flood the console otherwise.
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

  #buildUrl() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    // basePath comes from window.X_UI_BASE_PATH which is only injected
    // by the Go binary in production. In dev (Vite serves directly) the
    // global is missing and basePath would be '' — without the fallback to
    // '/' we'd build `ws://host:portws` (no separator) and the WebSocket
    // constructor throws a SyntaxError.
    let basePath = this.basePath || '/';
    if (!basePath.startsWith('/')) basePath = '/' + basePath;
    if (!basePath.endsWith('/')) basePath += '/';
    return `${protocol}//${window.location.host}${basePath}ws`;
  }

  #onMessage(event) {
    const data = event.data;
    // Reject oversized payloads up front. We compare actual UTF-8 byte
    // length (via Blob.size) against the limit — string.length counts
    // UTF-16 code units, which can undercount real bytes by up to 4× for
    // payloads with non-ASCII characters and bypass the cap.
    if (typeof data === 'string') {
      const byteLen = new Blob([data]).size;
      if (byteLen > WebSocketClient.#MAX_PAYLOAD_BYTES) {
        console.error(`WebSocket: payload too large (${byteLen} bytes), closing`);
        try { this.ws?.close(1009, 'message too big'); } catch { /* ignore */ }
        return;
      }
    }
    let message;
    try {
      message = JSON.parse(data);
    } catch (err) {
      console.error('WebSocket: invalid JSON message', err);
      return;
    }
    if (!message || typeof message !== 'object' || typeof message.type !== 'string') {
      console.error('WebSocket: malformed message envelope');
      return;
    }
    this.#emit(message.type, message.payload, message.time);
    this.#emit('message', message);
  }

  #emit(event, ...args) {
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

  #scheduleReconnect() {
    if (!this.shouldReconnect) return;
    this.#cancelReconnect();

    let base;
    if (this.reconnectAttempts < this.maxReconnectAttempts) {
      this.reconnectAttempts += 1;
      // Exponential backoff inside the active window.
      const exp = WebSocketClient.#BASE_RECONNECT_MS * 2 ** (this.reconnectAttempts - 1);
      base = Math.min(WebSocketClient.#MAX_RECONNECT_MS, exp);
    } else {
      // Active window exhausted — keep trying once a minute. The page-level
      // polling fallback runs in parallel; this just brings WS back when the
      // network recovers.
      base = WebSocketClient.#SLOW_RETRY_MS;
    }
    // ±25% jitter so reloads after a panel restart don't reconnect in lockstep.
    const delay = base * (0.75 + Math.random() * 0.5);

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      // clearTimeout doesn't cancel a callback that has already fired but
      // whose macrotask hasn't run yet — re-check shouldReconnect here so
      // disconnect() called in that window can't be overridden.
      if (!this.shouldReconnect) return;
      this.#openSocket();
    }, delay);
  }

  #cancelReconnect() {
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }
}

