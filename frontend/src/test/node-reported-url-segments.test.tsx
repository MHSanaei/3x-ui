import type { ReactNode } from 'react';
import { act, render, renderHook } from '@testing-library/react';
import { QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { useHostMutations } from '@/api/queries/useHostMutations';
import { BulkAddHostSchema } from '@/schemas/api/host';
import { DBInbound } from '@/models/dbinbound';
import InboundInfoModal from '@/pages/inbounds/info/InboundInfoModal';
import { makeTestQueryClient } from '@/test/test-utils';
import { HttpUtil } from '@/utils';

// Client emails and host group ids adopted from a node skip the panel's own
// character rules: unencoded, "../" and "?" re-target the admin's POST.
const hostile = '../../setting/update?tgBotEnable=true';
const encoded = encodeURIComponent(hostile);

describe('node-reported values stay one URL path segment', () => {
  const post = vi.mocked(HttpUtil.post);

  beforeEach(() => {
    post.mockClear();
  });

  it('encodes the client email in the inbound info IP-log request', () => {
    const dbInbound = new DBInbound({
      id: 2,
      protocol: 'http',
      port: 44301,
      settings: JSON.stringify({ clients: [{ email: hostile, enable: true, limitIp: 1 }] }),
      streamSettings: JSON.stringify({ network: 'tcp', security: 'none' }),
      sniffing: '',
      clientStats: [{ email: hostile, enable: true, up: 1, down: 1, total: 0, expiryTime: 0 }],
    });

    render(<InboundInfoModal open onClose={() => {}} dbInbound={dbInbound} ipLimitEnable />);

    expect(post).toHaveBeenCalledWith(`/panel/api/clients/ips/${encoded}`);
  });

  it('encodes the host group id in the host mutations', async () => {
    const queryClient = makeTestQueryClient();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useHostMutations(), { wrapper });

    await act(async () => {
      await result.current.update(
        hostile,
        BulkAddHostSchema.parse({ inboundIds: [1], remark: 'cdn' }),
      );
      await result.current.setEnable(hostile, false);
      await result.current.remove(hostile);
    });

    expect(post.mock.calls.map(([url]) => url)).toEqual([
      `/panel/api/hosts/update/${encoded}`,
      `/panel/api/hosts/setEnable/${encoded}`,
      `/panel/api/hosts/del/${encoded}`,
    ]);
  });
});
