import { onBeforeUnmount, onMounted } from 'vue';
import { WebSocketClient } from '@/api/websocket.js';

// One client per browser tab (= per multi-page entry). WebSocketClient is
// idempotent: repeated connect() calls while the socket is already open
// are no-ops, so multiple components on the same page can share a single
// underlying connection without each spawning their own.
let sharedClient = null;

function getSharedClient() {
  if (sharedClient) return sharedClient;
  const basePath = (typeof window !== 'undefined' && window.X_UI_BASE_PATH) || '';
  sharedClient = new WebSocketClient(basePath);
  return sharedClient;
}

// useWebSocket lets a Vue component subscribe to live server-pushed
// events. Pass a map of { eventName: handler } and the composable wires
// connect()/disconnect() into the component lifecycle and unsubscribes
// every handler on unmount so a stale closure can't fire after the
// page has moved on.
//
// Example:
//   useWebSocket({
//     traffic: (payload) => applyTrafficEvent(payload),
//     client_stats: (payload) => applyClientStatsEvent(payload),
//     invalidate: ({ type }) => { if (type === 'inbounds') refresh(); },
//   });
//
// Built-in lifecycle events ('connected' / 'disconnected' / 'error')
// can be subscribed to alongside server-emitted types.
export function useWebSocket(handlers) {
  const client = getSharedClient();
  const entries = Object.entries(handlers || {});

  onMounted(() => {
    for (const [event, fn] of entries) client.on(event, fn);
    client.connect();
  });

  onBeforeUnmount(() => {
    for (const [event, fn] of entries) client.off(event, fn);
    // Don't disconnect — another mounted component on the same page may
    // still be subscribed. The client closes naturally on page unload.
  });

  return { client };
}
