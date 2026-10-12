import type { ReactNode } from 'react';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { sameSpeedMap, useClients } from '@/hooks/useClients';
import { makeTestQueryClient } from '@/test/test-utils';
import { HttpUtil, Msg } from '@/utils';
import type { ClientsSummary } from '@/schemas/client';

afterEach(() => {
  vi.restoreAllMocks();
});

describe('websocket payload identity preservation', () => {
  const speed = (up: number, down: number) => ({ up, down });

  it('treats an unchanged speed map as unchanged', () => {
    const a = { 'a@x': speed(1, 2), 'b@x': speed(3, 4) };
    expect(sameSpeedMap(a, { 'a@x': speed(1, 2), 'b@x': speed(3, 4) })).toBe(true);
    expect(sameSpeedMap(a, { 'a@x': speed(1, 2) })).toBe(false);
    expect(sameSpeedMap(a, { 'a@x': speed(1, 2), 'b@x': speed(3, 5) })).toBe(false);
    expect(sameSpeedMap(a, { 'a@x': speed(1, 2), 'c@x': speed(3, 4) })).toBe(false);
    expect(sameSpeedMap({}, {})).toBe(true);
  });
});

describe('client summary always reflects the server, never a client_stats recompute (#6116)', () => {
  const serverSummary: ClientsSummary = {
    total: 3,
    active: 3,
    onlineCount: 0,
    depletedCount: 0,
    expiringCount: 0,
    deactiveCount: 0,
    online: [],
    depleted: [],
    expiring: [],
    deactive: [],
  };

  const pagedResponse = {
    items: [],
    total: 3,
    filtered: 3,
    page: 1,
    pageSize: 25,
    groups: [],
    summary: serverSummary,
  };

  function mockPanel() {
    vi.spyOn(HttpUtil, 'get').mockImplementation(async (url: string) => {
      if (url.includes('/clients/list/paged')) return new Msg(true, '', pagedResponse);
      if (url.includes('/inbounds/options')) return new Msg(true, '', []);
      return new Msg(true, '', null);
    });
    vi.spyOn(HttpUtil, 'post').mockImplementation(async (url: string) => {
      if (url.includes('/setting/defaultSettings')) return new Msg(true, '', { pageSize: 25 });
      if (url.includes('/clients/onlines')) return new Msg(true, '', []);
      return new Msg(true, '', null);
    });
  }

  function wrapperFor() {
    const queryClient = makeTestQueryClient();
    return ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  }

  async function loadedHook() {
    mockPanel();
    const { result } = renderHook(() => useClients(), { wrapper: wrapperFor() });
    await waitFor(() => expect(result.current.settingsReady).toBe(true));
    act(() => {
      result.current.setQuery({ page: 1, pageSize: 25, sort: 'createdAt', order: 'ascend' });
    });
    await waitFor(() => expect(result.current.fetched).toBe(true));
    expect(result.current.summary).toEqual(serverSummary);
    return result;
  }

  it('stays pinned to the server summary across a client_stats push carrying an orphan row with no matching gap', async () => {
    const result = await loadedHook();

    act(() => {
      result.current.applyClientStatsEvent({
        snapshot: true,
        clients: [
          { email: 'a@x', enable: true, up: 0, down: 0, total: 0, expiryTime: 0 },
          { email: 'b@x', enable: true, up: 0, down: 0, total: 0, expiryTime: 0 },
          { email: 'c@x', enable: true, up: 0, down: 0, total: 0, expiryTime: 0 },
          { email: 'ghost@x', enable: false, up: 0, down: 0, total: 1, expiryTime: 1 },
        ],
      });
    });

    expect(result.current.summary).toEqual(serverSummary);
  });

  it('stays pinned to the server summary across a client_stats push where an orphan and a gap net out to the server total', async () => {
    const result = await loadedHook();

    act(() => {
      result.current.applyClientStatsEvent({
        snapshot: true,
        clients: [
          { email: 'a@x', enable: true, up: 0, down: 0, total: 0, expiryTime: 0 },
          { email: 'b@x', enable: true, up: 0, down: 0, total: 0, expiryTime: 0 },
          { email: 'ghost@x', enable: false, up: 0, down: 0, total: 1, expiryTime: 1 },
        ],
      });
    });

    expect(result.current.summary).toEqual(serverSummary);
  });

  it('combines independently sampled TUIC and Xray speeds and replaces each source snapshot', async () => {
    const result = await loadedHook();

    act(() => {
      result.current.applyTrafficEvent({
        clientTraffics: [{ email: 'shared@example.test', up: 100, down: 150 }],
      });
    });
    expect(result.current.clientSpeed['shared@example.test']).toEqual({ up: 20, down: 30 });

    act(() => {
      result.current.applyTrafficEvent({
        clientTrafficSource: 'tuic',
        clientTrafficIntervalMs: 10_000,
        clientTraffics: [
          { email: 'shared@example.test', up: 300, down: 100 },
          { email: 'shared@example.test', up: 100, down: 100 },
        ],
      });
    });
    expect(result.current.clientSpeed['shared@example.test']).toEqual({ up: 60, down: 50 });

    act(() => {
      result.current.applyTrafficEvent({
        clientTraffics: [{ email: 'shared@example.test', up: 50, down: 25 }],
      });
    });
    expect(result.current.clientSpeed['shared@example.test']).toEqual({ up: 50, down: 25 });

    act(() => {
      result.current.applyTrafficEvent({
        clientTrafficSource: 'tuic',
        clientTrafficIntervalMs: 10_000,
        clientTraffics: [],
      });
    });
    expect(result.current.clientSpeed['shared@example.test']).toEqual({ up: 10, down: 5 });
  });

  it('expires stale per-source speeds without clearing the other source', async () => {
    const result = await loadedHook();
    vi.useFakeTimers();
    try {
      act(() => {
        result.current.applyTrafficEvent({
          clientTrafficSource: 'xray',
          clientTrafficIntervalMs: 1_000,
          clientTraffics: [{ email: 'shared@example.test', up: 100, down: 200 }],
        });
        result.current.applyTrafficEvent({
          clientTrafficSource: 'tuic',
          clientTrafficIntervalMs: 2_000,
          clientTraffics: [{ email: 'shared@example.test', up: 300, down: 400 }],
        });
      });
      expect(result.current.clientSpeed['shared@example.test']).toEqual({ up: 250, down: 400 });

      act(() => vi.advanceTimersByTime(2_000));
      expect(result.current.clientSpeed['shared@example.test']).toEqual({ up: 150, down: 200 });

      act(() => vi.advanceTimersByTime(2_000));
      expect(result.current.clientSpeed).toEqual({});
    } finally {
      vi.useRealTimers();
    }
  });
});
