import type { ReactNode } from 'react';
import { act, renderHook, waitFor } from '@testing-library/react';
import { QueryClientProvider } from '@tanstack/react-query';
import { describe, expect, it } from 'vitest';

import { keys } from '@/api/queryKeys';
import { useInbounds } from '@/pages/inbounds/useInbounds';

import { makeTestQueryClient } from './test-utils';

function seedInbounds() {
  const rows = [1, 2].map((id) => ({
    id,
    protocol: 'vless',
    tag: `in-${id}`,
    enable: true,
    up: 10,
    down: 20,
    total: 0,
    expiryTime: 0,
    settings: JSON.stringify({ clients: [{ email: `c${id}@x`, enable: true }] }),
    clientStats: [
      { email: `c${id}@x`, up: 1, down: 2, total: 0, expiryTime: 0, enable: true, inboundId: id },
    ],
  }));
  const queryClient = makeTestQueryClient();
  queryClient.setQueryData(keys.inbounds.slim(), rows);
  queryClient.setQueryData(keys.clients.onlines(), []);
  queryClient.setQueryData(keys.clients.onlinesByGuid(), {});
  queryClient.setQueryData(keys.clients.activeInbounds(), {});
  queryClient.setQueryData(keys.clients.lastOnline(), {});
  queryClient.setQueryData(keys.settings.defaults(), {});
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return { rows, wrapper };
}

async function renderInbounds() {
  const { rows, wrapper } = seedInbounds();
  const hook = renderHook(() => useInbounds(), { wrapper });
  await waitFor(() => expect(hook.result.current.dbInbounds).toHaveLength(2));
  return { rows, result: hook.result };
}

// Every client_stats push carries all inbounds' totals, so rebuilding a row whether or
// not its numbers moved re-ran the client rollup and the whole table on each push.
describe('inbound websocket merges keep unchanged state', () => {
  it('keeps rows and the client rollup when a client_stats push changes nothing', async () => {
    const { rows, result } = await renderInbounds();
    const before = result.current.dbInbounds;
    const rollup = result.current.clientCount;

    act(() =>
      result.current.applyClientStatsEvent({
        inbounds: rows.map((r) => ({
          id: r.id,
          up: r.up,
          down: r.down,
          total: r.total,
          enable: r.enable,
        })),
        clients: [{ email: 'c1@x', up: 1, down: 2, total: 0, expiryTime: 0, enable: true }],
      }),
    );

    expect(result.current.dbInbounds).toBe(before);
    expect(result.current.clientCount).toBe(rollup);
  });

  it('still rebuilds exactly the rows whose numbers moved', async () => {
    const { result } = await renderInbounds();
    const before = result.current.dbInbounds;

    act(() =>
      result.current.applyClientStatsEvent({
        inbounds: [
          { id: 1, up: 99, down: 20, total: 0, enable: true },
          { id: 2, up: 10, down: 20, total: 0, enable: true },
        ],
        clients: [{ email: 'c2@x', up: 5, down: 2, total: 0, expiryTime: 0, enable: true }],
      }),
    );

    const [first, second] = result.current.dbInbounds;
    expect(first).not.toBe(before[0]);
    expect(first.up).toBe(99);
    expect(second).not.toBe(before[1]);
    expect(second.clientStats?.[0]?.up).toBe(5);
  });

  it('keeps the client rollup when a traffic push repeats the same online sets', async () => {
    const { result } = await renderInbounds();
    const push = () =>
      result.current.applyTrafficEvent({
        onlineClients: ['c1@x'],
        onlineByGuid: { 'node:1': ['c1@x'] },
        activeInbounds: { 'node:1': ['in-1'] },
      });
    act(push);
    const rollup = result.current.clientCount;

    act(push);

    expect(result.current.clientCount).toBe(rollup);
  });
});
