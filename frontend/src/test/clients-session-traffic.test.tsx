import type { ReactNode } from 'react';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { useClients } from '@/hooks/useClients';
import { makeTestQueryClient } from '@/test/test-utils';
import { HttpUtil, Msg } from '@/utils';

afterEach(() => {
  vi.restoreAllMocks();
});

// The session columns reach the page through two hand-written paths, the paged
// list's schema and the client_stats merge; either one can silently drop them.
describe('session traffic reaches the clients page', () => {
  const pagedResponse = {
    items: [
      {
        email: 'live@x',
        enable: true,
        inboundIds: [1],
        traffic: {
          up: 5000,
          down: 9000,
          lastOnline: 1735680000000,
          sessionStart: 1735676400000,
          sessionUp: 300,
          sessionDown: 700,
        },
      },
    ],
    total: 1,
    filtered: 1,
    page: 1,
    pageSize: 25,
    groups: [],
  };

  function mockPanel() {
    vi.spyOn(HttpUtil, 'get').mockImplementation(async (url: string) => {
      if (url.includes('/clients/list/paged')) return new Msg(true, '', pagedResponse);
      if (url.includes('/inbounds/options')) return new Msg(true, '', []);
      return new Msg(true, '', null);
    });
    vi.spyOn(HttpUtil, 'post').mockImplementation(async (url: string) => {
      if (url.includes('/setting/defaultSettings')) return new Msg(true, '', { pageSize: 25 });
      if (url.includes('/clients/onlines')) return new Msg(true, '', ['live@x']);
      return new Msg(true, '', null);
    });
  }

  async function loadedHook() {
    mockPanel();
    const queryClient = makeTestQueryClient();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useClients(), { wrapper });
    await waitFor(() => expect(result.current.settingsReady).toBe(true));
    act(() => {
      result.current.setQuery({ page: 1, pageSize: 25, sort: 'createdAt', order: 'ascend' });
    });
    await waitFor(() => expect(result.current.clients).toHaveLength(1));
    return result;
  }

  it('keeps the session columns of a paged list row', async () => {
    const result = await loadedHook();

    expect(result.current.clients[0]?.traffic).toMatchObject({
      sessionStart: 1735676400000,
      sessionUp: 300,
      sessionDown: 700,
    });
  });

  it('merges a client_stats push into the running session', async () => {
    const result = await loadedHook();

    act(() => {
      result.current.applyClientStatsEvent({
        snapshot: true,
        clients: [
          {
            email: 'live@x',
            up: 5100,
            down: 9200,
            lastOnline: 1735680005000,
            sessionStart: 1735676400000,
            sessionUp: 400,
            sessionDown: 900,
          },
        ],
      });
    });

    await waitFor(() =>
      expect(result.current.clients[0]?.traffic).toMatchObject({
        up: 5100,
        sessionStart: 1735676400000,
        sessionUp: 400,
        sessionDown: 900,
      }),
    );
  });
});
