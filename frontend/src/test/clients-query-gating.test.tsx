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

const emptyPage = {
  items: [],
  total: 0,
  filtered: 0,
  page: 1,
  pageSize: 25,
  groups: [],
  summary: {
    total: 0,
    active: 0,
    onlineCount: 0,
    depletedCount: 0,
    expiringCount: 0,
    deactiveCount: 0,
    online: [],
    depleted: [],
    expiring: [],
    deactive: [],
  },
};

function mockPanel(defaults: Record<string, unknown>) {
  const pagedUrls: string[] = [];
  vi.spyOn(HttpUtil, 'get').mockImplementation(async (url: string) => {
    if (url.includes('/clients/list/paged')) {
      pagedUrls.push(url);
      return new Msg(true, '', emptyPage);
    }
    if (url.includes('/inbounds/options')) return new Msg(true, '', []);
    return new Msg(true, '', null);
  });
  vi.spyOn(HttpUtil, 'post').mockImplementation(async (url: string) => {
    if (url.includes('/setting/defaultSettings')) return new Msg(true, '', defaults);
    if (url.includes('/clients/onlines')) return new Msg(true, '', []);
    return new Msg(true, '', null);
  });
  return pagedUrls;
}

function wrapperFor() {
  const queryClient = makeTestQueryClient();
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe('useClients query gating', () => {
  it('does not fetch the list until the page supplies a query', async () => {
    const pagedUrls = mockPanel({ pageSize: 25 });
    const { result } = renderHook(() => useClients(), { wrapper: wrapperFor() });

    await waitFor(() => expect(result.current.settingsReady).toBe(true));
    // The page has not called setQuery yet, so nothing should have gone out —
    // this is what used to cost a thrown-away round trip on every page load.
    expect(pagedUrls).toEqual([]);
    expect(result.current.fetched).toBe(false);
  });

  it('issues exactly one request for a page load that settles on one query', async () => {
    const pagedUrls = mockPanel({ pageSize: 50 });
    const { result } = renderHook(() => useClients(), { wrapper: wrapperFor() });

    await waitFor(() => expect(result.current.settingsReady).toBe(true));
    act(() => {
      result.current.setQuery({ page: 1, pageSize: 50, sort: 'createdAt', order: 'ascend' });
    });

    await waitFor(() => expect(result.current.fetched).toBe(true));
    expect(pagedUrls).toHaveLength(1);
    expect(pagedUrls[0]).toContain('pageSize=50');
    expect(pagedUrls[0]).toContain('sort=createdAt');
  });

  it('fetches as soon as a query arrives, without waiting for the settings', async () => {
    // The page remembers the previous visit's page size in localStorage, so on a
    // return visit it can supply a query on the first render. The hook must not
    // hold that back behind /setting/defaultSettings, or the two round trips
    // serialise and the list lands ~160ms later than it needs to.
    const pagedUrls = mockPanel({ pageSize: 25 });
    const { result } = renderHook(() => useClients(), { wrapper: wrapperFor() });

    act(() => {
      result.current.setQuery({ page: 1, pageSize: 25, sort: 'createdAt', order: 'ascend' });
    });
    await waitFor(() => expect(pagedUrls).toHaveLength(1));
  });

  it('reports settingsReady even when the settings request fails, so the page can still render', async () => {
    vi.spyOn(HttpUtil, 'get').mockImplementation(async (url: string) => new Msg(
      true,
      '',
      url.includes('/inbounds/options') ? [] : emptyPage,
    ));
    vi.spyOn(HttpUtil, 'post').mockResolvedValue(new Msg(false, 'boom', null));
    const { result } = renderHook(() => useClients(), { wrapper: wrapperFor() });

    await waitFor(() => expect(result.current.settingsReady).toBe(true));
  });

  it('skips the list, options and onlines queries for mutation-only callers', async () => {
    const pagedUrls = mockPanel({ pageSize: 25 });
    const postSpy = vi.mocked(HttpUtil.post);
    const { result } = renderHook(() => useClients({ list: false }), { wrapper: wrapperFor() });

    await waitFor(() => expect(result.current.settingsReady).toBe(true));
    act(() => {
      result.current.setQuery({ page: 1, pageSize: 25, sort: 'createdAt', order: 'ascend' });
    });

    await waitFor(() => expect(result.current.settingsReady).toBe(true));
    expect(pagedUrls).toEqual([]);
    // subSettings still needs defaultSettings; onlines must not be polled.
    const posted = postSpy.mock.calls.map((c) => String(c[0]));
    expect(posted.some((u) => u.includes('/setting/defaultSettings'))).toBe(true);
    expect(posted.some((u) => u.includes('/clients/onlines'))).toBe(false);
    expect(vi.mocked(HttpUtil.get).mock.calls.map((c) => String(c[0]))).toEqual([]);
  });
});
