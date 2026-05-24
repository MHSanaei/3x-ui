import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';

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

let invalidateTimer: number | null = null;

export function useWebSocketBridge() {
  const queryClient = useQueryClient();

  useEffect(() => {
    const client = getSharedClient();

    const onInvalidate: Handler = (payload) => {
      const p = payload as { type?: string } | undefined;
      if (!p || (p.type !== 'inbounds' && p.type !== 'clients')) return;
      if (invalidateTimer != null) clearTimeout(invalidateTimer);
      invalidateTimer = window.setTimeout(() => {
        invalidateTimer = null;
        if (p.type === 'inbounds') {
          queryClient.invalidateQueries({ queryKey: ['inbounds'] });
        } else {
          queryClient.invalidateQueries({ queryKey: ['clients'] });
        }
      }, 200);
    };

    const onOutbounds: Handler = (payload) => {
      queryClient.setQueryData(['xray', 'outboundsTraffic'], payload);
    };

    client.on('invalidate', onInvalidate);
    client.on('outbounds', onOutbounds);
    client.connect();

    return () => {
      client.off('invalidate', onInvalidate);
      client.off('outbounds', onOutbounds);
      if (invalidateTimer != null) {
        clearTimeout(invalidateTimer);
        invalidateTimer = null;
      }
    };
  }, [queryClient]);
}
