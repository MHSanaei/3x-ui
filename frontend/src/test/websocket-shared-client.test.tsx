import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';

import { useWebSocket } from '@/hooks/useWebSocket';
import { useWebSocketBridge } from '@/api/websocketBridge';

describe('shared WebSocket connection', () => {
  let socketCount = 0;
  let originalWS: typeof WebSocket;

  beforeEach(() => {
    socketCount = 0;
    originalWS = globalThis.WebSocket;
    class FakeWebSocket {
      static readonly CONNECTING = 0;
      static readonly OPEN = 1;
      static readonly CLOSING = 2;
      static readonly CLOSED = 3;
      readyState = FakeWebSocket.CONNECTING;
      constructor() {
        socketCount += 1;
      }
      addEventListener() {}
      removeEventListener() {}
      close() {}
      send() {}
    }
    globalThis.WebSocket = FakeWebSocket as unknown as typeof WebSocket;
  });

  afterEach(() => {
    globalThis.WebSocket = originalWS;
    vi.restoreAllMocks();
  });

  it('opens a single socket when the bridge and a page hook are both mounted', () => {
    const queryClient = new QueryClient();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );

    renderHook(() => useWebSocketBridge(), { wrapper });
    renderHook(() => useWebSocket({ traffic: () => {} }));

    expect(socketCount).toBe(1);
  });
});
