import type { ReactNode } from 'react';
import { act, renderHook, waitFor } from '@testing-library/react';
import { QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { useXraySetting } from '@/hooks/useXraySetting';
import { makeTestQueryClient } from '@/test/test-utils';
import { HttpUtil, Msg } from '@/utils';

function xrayPayload(overrides: Record<string, unknown> = {}) {
  return {
    xraySetting: {},
    inboundTags: [],
    clientReverseTags: [],
    outboundTestUrl: 'https://test.example',
    subscriptionOutbounds: [],
    subscriptionOutboundTags: [],
    ...overrides,
  };
}

afterEach(() => {
  vi.restoreAllMocks();
});

beforeEach(() => {
  vi.spyOn(HttpUtil, 'get').mockResolvedValue(new Msg(true, '', []));
});

describe('useXraySetting', () => {
  it('refreshes server-derived outbounds while the editor is dirty', async () => {
    let payload = xrayPayload({ subscriptionOutbounds: [{ tag: 'before' }] });
    vi.spyOn(HttpUtil, 'post').mockImplementation(async (url) => {
      if (url === '/panel/api/xray/') return new Msg(true, '', JSON.stringify(payload));
      return new Msg(true, '');
    });
    const queryClient = makeTestQueryClient();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useXraySetting(), { wrapper });

    await waitFor(() => expect(result.current.fetched).toBe(true));
    act(() => result.current.setXraySetting('{"outbounds":[]}'));
    payload = xrayPayload({ subscriptionOutbounds: [{ tag: 'after' }] });
    await act(async () => result.current.fetchAll());

    await waitFor(() => expect(result.current.subscriptionOutbounds).toEqual([{ tag: 'after' }]));
    expect(result.current.xraySetting).toBe('{"outbounds":[]}');
  });

  it('keeps the outbound test URL input empty when it is cleared', async () => {
    const payload = xrayPayload({ outboundTestUrl: 'https://www.google.com/generate_204' });
    vi.spyOn(HttpUtil, 'post').mockImplementation(async (url) => {
      if (url === '/panel/api/xray/') return new Msg(true, '', JSON.stringify(payload));
      return new Msg(true, '');
    });
    const queryClient = makeTestQueryClient();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useXraySetting(), { wrapper });

    await waitFor(() => expect(result.current.fetched).toBe(true));
    act(() => result.current.setOutboundTestUrl(''));

    expect(result.current.outboundTestUrl).toBe('');
    expect(result.current.saveDisabled).toBe(true);
  });
});
