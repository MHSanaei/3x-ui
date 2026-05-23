import { useEffect } from 'react';
import { WebSocketClient } from '@/api/websocket.js';

type Handler = (payload: unknown) => void;

interface SharedClient {
  connect(): void;
  on(event: string, fn: Handler): void;
  off(event: string, fn: Handler): void;
}

let sharedClient: SharedClient | null = null;

function getSharedClient(): SharedClient {
  if (sharedClient) return sharedClient;
  const basePath = (typeof window !== 'undefined' && window.X_UI_BASE_PATH) || '';
  sharedClient = new WebSocketClient(basePath) as SharedClient;
  return sharedClient;
}

export function useWebSocket(handlers: Record<string, Handler>) {
  useEffect(() => {
    const client = getSharedClient();
    const entries = Object.entries(handlers);
    for (const [event, fn] of entries) client.on(event, fn);
    client.connect();
    return () => {
      for (const [event, fn] of entries) client.off(event, fn);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
}
