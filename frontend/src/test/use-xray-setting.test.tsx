import type { ReactNode } from 'react';
import { act, renderHook, waitFor } from '@testing-library/react';
import { QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';

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

  it('becomes clean after saving an empty outbound test URL', async () => {
    let payload = xrayPayload();
    vi.spyOn(HttpUtil, 'post').mockImplementation(async (url) => {
      if (url === '/panel/api/xray/') return new Msg(true, '', JSON.stringify(payload));
      if (url === '/panel/api/xray/update') {
        payload = xrayPayload({ outboundTestUrl: 'https://www.google.com/generate_204' });
        return new Msg(true, '');
      }
      return new Msg(true, '');
    });
    const queryClient = makeTestQueryClient();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useXraySetting(), { wrapper });

    await waitFor(() => expect(result.current.fetched).toBe(true));
    act(() => result.current.setOutboundTestUrl(''));
    expect(result.current.saveDisabled).toBe(false);
    await act(async () => result.current.saveAll());

    await waitFor(() => expect(result.current.saveDisabled).toBe(true));
  });
});
