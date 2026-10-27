import { ConfigProvider } from 'antd';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import NodesPage from '@/pages/nodes/NodesPage';
import { HttpUtil, Msg } from '@/utils';
import type { NodeRecord } from '@/schemas/node';
import { renderWithProviders } from './test-utils';

const state = vi.hoisted(() => ({
  isMobile: false,
  nodes: [] as NodeRecord[],
  refetch: vi.fn(),
}));

vi.mock('@/hooks/useMediaQuery', () => ({ useMediaQuery: () => ({ isMobile: state.isMobile }) }));
vi.mock('@/api/queries/useNodesQuery', () => ({
  useNodesQuery: () => ({
    nodes: state.nodes,
    loading: false,
    fetched: true,
    fetchError: '',
    refetch: state.refetch,
    totals: {
      total: state.nodes.length,
      online: 0,
      offline: 1,
      avgLatency: 0,
      inbounds: 0,
      clients: 0,
      onlineClients: 0,
      depleted: 0,
    },
  }),
}));
vi.mock('@/pages/nodes/NodeFormModal', () => ({ default: () => null }));
vi.mock('@/pages/nodes/NodeHistoryPanel', () => ({ default: () => null }));
vi.mock('@/layouts/AppSidebar', () => ({ default: () => null }));

beforeEach(() => {
  state.isMobile = false;
  state.nodes = [{ id: 7, name: 'edge-seven', guid: 'seven', enable: true, status: 'offline' }];
  vi.spyOn(HttpUtil, 'get').mockResolvedValue(new Msg(true, '', {}));
});
afterEach(() => vi.restoreAllMocks());

async function openRestart(label: string) {
  fireEvent.click(screen.getByRole('button', { name: /^more$/i }));
  fireEvent.click(await screen.findByRole('menuitem', { name: label }));
  return screen.findByRole('dialog');
}

describe('remote node restart', () => {
  it.each([false, true])(
    'confirms each action and sends only the selected request (mobile=%s)',
    async (mobile) => {
      state.isMobile = mobile;
      const post = vi
        .spyOn(HttpUtil, 'post')
        .mockResolvedValue({ success: true, msg: '', obj: null });
      renderWithProviders(
        <ConfigProvider theme={{ token: { motion: false } }}>
          <NodesPage />
        </ConfigProvider>,
      );
      let dialog = await openRestart('Restart Xray');
      expect(dialog.textContent).toContain('edge-seven');
      expect(post).not.toHaveBeenCalled();
      fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));

      await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
      expect(post).not.toHaveBeenCalled();
      for (const [label, action, success] of [
        ['Restart Xray', 'restartXray', 'Xray restarted on the node.'],
        [
          'Restart Panel',
          'restartPanel',
          'Panel restart request accepted. The node may be temporarily unavailable.',
        ],
      ]) {
        dialog = await openRestart(label);
        fireEvent.click(within(dialog).getByRole('button', { name: label }));
        await screen.findByText(success);
        expect(post.mock.calls.at(-1)?.[0]).toBe(`/panel/api/nodes/${action}/7`);

        await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
      }
      expect(post).toHaveBeenCalledTimes(2);
    },
  );

  it('keeps both actions disabled while the request is pending and reports a node rejection without retrying', async () => {
    let finish!: (msg: Msg<never>) => void;
    const post = vi.spyOn(HttpUtil, 'post').mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    renderWithProviders(
      <ConfigProvider theme={{ token: { motion: false } }}>
        <NodesPage />
      </ConfigProvider>,
    );
    const dialog = await openRestart('Restart Panel');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Restart Panel' }));
    await waitFor(() => expect(post).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByRole('button', { name: /^more$/i }));
    for (const name of ['Restart Xray', 'Restart Panel']) {
      expect((await screen.findByRole('menuitem', { name })).getAttribute('aria-disabled')).toBe(
        'true',
      );
    }
    finish({ success: false, msg: 'remote permission denied', obj: null });
    await screen.findByText('remote permission denied');
    expect(post).toHaveBeenCalledTimes(1);
  });

  it.each([false, true])(
    'does not offer restart for transitive nodes and disables it for disabled nodes (mobile=%s)',
    async (mobile) => {
      state.isMobile = mobile;
      state.nodes = [
        { id: 0, name: 'child', guid: 'child', transitive: true, enable: true, status: 'online' },
      ];
      const view = renderWithProviders(
        <ConfigProvider theme={{ token: { motion: false } }}>
          <NodesPage />
        </ConfigProvider>,
      );
      expect(screen.queryByRole('button', { name: /^more$/i })).toBeNull();
      view.unmount();
      state.nodes = [{ id: 7, name: 'disabled', enable: false }];
      renderWithProviders(
        <ConfigProvider theme={{ token: { motion: false } }}>
          <NodesPage />
        </ConfigProvider>,
      );
      fireEvent.click(screen.getByRole('button', { name: /^more$/i }));
      for (const name of ['Restart Xray', 'Restart Panel']) {
        expect((await screen.findByRole('menuitem', { name })).getAttribute('aria-disabled')).toBe(
          'true',
        );
      }
    },
  );
});
