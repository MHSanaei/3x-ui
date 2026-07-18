import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';

import { getSharedWebSocketClient } from '@/api/websocket';
import { useWebSocketBridge } from '@/api/websocketBridge';
import { keys } from '@/api/queryKeys';

type ListenerMap = { listeners: Map<string, Set<(payload: unknown) => void>> };

describe('websocket bridge outbounds handler', () => {
  let originalWS: typeof WebSocket;

  beforeEach(() => {
    originalWS = globalThis.WebSocket;
    class FakeWebSocket {
      static readonly CONNECTING = 0;
      static readonly OPEN = 1;
      static readonly CLOSING = 2;
      static readonly CLOSED = 3;
      readyState = FakeWebSocket.CONNECTING;
      addEventListener() {}
      removeEventListener() {}
      close() {}
      send() {}
    }
    globalThis.WebSocket = FakeWebSocket as unknown as typeof WebSocket;
  });

  afterEach(() => {
    globalThis.WebSocket = originalWS;
  });

  it('ignores a non-array outbounds push instead of poisoning the cache', () => {
    const queryClient = new QueryClient();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );

    renderHook(() => useWebSocketBridge(), { wrapper });

    const client = getSharedWebSocketClient() as unknown as ListenerMap;
    const handlers = client.listeners.get('outbounds');
    expect(handlers && handlers.size).toBeGreaterThan(0);

    for (const handler of handlers ?? []) handler({ not: 'an array' });

    expect(queryClient.getQueryData(keys.xray.outboundsTraffic())).toBeUndefined();
  });
});
