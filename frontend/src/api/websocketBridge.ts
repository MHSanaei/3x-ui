import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';

import { WebSocketClient } from '@/api/websocket';
import { keys } from '@/api/queryKeys';

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
      queryClient.setQueryData(keys.xray.outboundsTraffic(), payload);
    };

    const onNodes: Handler = (payload) => {
      if (!Array.isArray(payload)) return;
      queryClient.setQueryData(keys.nodes.list(), payload);
    };

    const onInbounds: Handler = (payload) => {
      if (!Array.isArray(payload)) return;
      queryClient.setQueryData(keys.inbounds.slim(), payload);
    };

    client.on('invalidate', onInvalidate);
    client.on('outbounds', onOutbounds);
    client.on('nodes', onNodes);
    client.on('inbounds', onInbounds);
    client.connect();

    return () => {
      client.off('invalidate', onInvalidate);
      client.off('outbounds', onOutbounds);
      client.off('nodes', onNodes);
      client.off('inbounds', onInbounds);
      if (invalidateTimer != null) {
        clearTimeout(invalidateTimer);
        invalidateTimer = null;
      }
    };
  }, [queryClient]);
}
