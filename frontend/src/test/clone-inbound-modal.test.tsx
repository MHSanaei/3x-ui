import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';

import CloneInboundModal from '@/pages/inbounds/CloneInboundModal';
import { HttpUtil } from '@/utils';
import { DBInbound } from '@/models/dbinbound';
import { ThemeProvider } from '@/hooks/useTheme';
import type { NodeRecord } from '@/api/queries/useNodesQuery';

import { renderWithProviders } from './test-utils';

const postSpy = vi.mocked(HttpUtil.post);

const NODES = [
  { id: 2, name: 'arm2', enable: true, status: 'online' },
  { id: 3, name: 'arm3', enable: true, status: 'offline' },
  { id: 4, name: 'retired', enable: false, status: 'online' },
  { id: 5, name: 'arm5', enable: true, status: 'unknown' },
] as unknown as NodeRecord[];

function sourceInbound() {
  return new DBInbound({
    id: 7,
    port: 443,
    listen: '',
    protocol: 'vless',
    remark: 'edge',
    enable: true,
    settings: JSON.stringify({ clients: [{ id: 'uuid-1', email: 'a@test' }], decryption: 'none' }),
    streamSettings: JSON.stringify({ network: 'tcp', security: 'none' }),
    sniffing: '',
    nodeId: 2,
    shareAddrStrategy: 'node',
    shareAddr: '',
  });
}

function renderModal(onCloned = vi.fn(), onClose = vi.fn()) {
  renderWithProviders(
    <CloneInboundModal
      open
      dbInbound={sourceInbound()}
      nodes={NODES}
      portsInUse={new Map([[2, new Set([443])]])}
      onClose={onClose}
      onCloned={onCloned}
    />,
  );
  return { onCloned, onClose };
}

function openTargetDropdown() {
  // antd v6 Select has no .ant-select-selector; mouseDown on the root opens it.
  const selector = document.querySelector('.ant-select');
  if (!selector) throw new Error('target select not rendered');
  fireEvent.mouseDown(selector);
}

function clickOption(text: string) {
  const option = Array.from(document.querySelectorAll('.ant-select-item-option'))
    .find((o) => (o.textContent ?? '').trim() === text);
  if (!option) throw new Error(`option '${text}' not found`);
  fireEvent.click(option);
}

function clickOk() {
  fireEvent.click(screen.getByRole('button', { name: 'Clone' }));
}

type PostBody = Record<string, unknown> & { nodeId?: number };
const postedBodies = () => postSpy.mock.calls.map((c) => c[1] as PostBody);

const selectedTitles = () => Array.from(document.querySelectorAll('.ant-select-selection-item[title]'))
  .map((el) => el.getAttribute('title'));

beforeEach(() => {
  postSpy.mockClear();
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  postSpy.mockResolvedValue({ success: true, obj: {} } as any);
});

describe('CloneInboundModal', () => {
  it('pre-selects the source node and clones onto it with a fresh port and no clients', async () => {
    const { onCloned, onClose } = renderModal();

    expect(document.querySelector('.ant-select-selection-item[title="arm2"]')).toBeTruthy();

    clickOk();
    await waitFor(() => expect(postSpy).toHaveBeenCalledTimes(1));

    expect(postSpy.mock.calls[0][0]).toBe('/panel/api/inbounds/add');
    const body = postedBodies()[0];
    expect(body.nodeId).toBe(2);
    expect(body.enable).toBe(false);
    expect(body.remark).toBe('edge (clone)');
    expect(body.port).not.toBe(443);
    expect(body).not.toHaveProperty('tag');
    expect(JSON.parse(body.settings as string).clients).toEqual([]);

    await waitFor(() => expect(onCloned).toHaveBeenCalledTimes(1));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('posts once per selected target and omits nodeId for the local panel', async () => {
    renderModal();
    openTargetDropdown();
    clickOption('Local panel');

    clickOk();
    await waitFor(() => expect(postSpy).toHaveBeenCalledTimes(2));

    const [nodeBody, localBody] = postedBodies();
    expect(nodeBody.nodeId).toBe(2);
    expect(localBody).not.toHaveProperty('nodeId');
    expect(nodeBody.port).not.toBe(443);
  });

  it('disables non-online nodes and hides disabled nodes from the target list', () => {
    renderModal();
    openTargetDropdown();

    const option = (text: string) => Array.from(document.querySelectorAll('.ant-select-item-option'))
      .find((o) => (o.textContent ?? '').trim() === text);
    // Only `online` is selectable — `offline` and `unknown` (no heartbeat
    // yet) are both shown but disabled.
    expect(option('arm3 (offline)')?.className).toContain('ant-select-item-option-disabled');
    expect(option('arm5 (unknown)')?.className).toContain('ant-select-item-option-disabled');
    expect(option('arm2')?.className).not.toContain('ant-select-item-option-disabled');

    const labels = Array.from(document.querySelectorAll('.ant-select-item-option'))
      .map((o) => (o.textContent ?? '').trim());
    expect(labels).toEqual(['Local panel', 'arm2', 'arm3 (offline)', 'arm5 (unknown)']);
  });

  it('select-all picks only selectable targets and clear-all blocks submit', () => {
    renderModal();

    const selectAll = screen.getByRole('button', { name: 'Select all' });
    fireEvent.click(selectAll);

    // Local panel + online node; offline/unknown nodes stay unpickable.
    expect(selectedTitles().sort()).toEqual(['Local panel', 'arm2']);
    expect((selectAll as HTMLButtonElement).disabled).toBe(true);

    // Clear all empties the selection and disables OK.
    fireEvent.click(screen.getByRole('button', { name: 'Clear all' }));
    expect(selectedTitles()).toEqual([]);
    expect((screen.getByRole('button', { name: 'Clone' }) as HTMLButtonElement).disabled).toBe(true);
  });

  it('keeps a cleared selection when the nodes list refetches mid-dialog', () => {
    // The page LazyMounts the modal once and keeps it mounted; heartbeats give
    // `nodes` a new array identity on every refetch. The reset effect must not
    // refire on that — only on the open transition.
    const modal = (nodes: NodeRecord[], open = true) => (
      <ThemeProvider>
        <CloneInboundModal
          open={open}
          dbInbound={sourceInbound()}
          nodes={nodes}
          portsInUse={new Map()}
          onClose={() => {}}
          onCloned={() => {}}
        />
      </ThemeProvider>
    );
    const { rerender } = render(modal(NODES));

    fireEvent.click(screen.getByRole('button', { name: 'Clear all' }));
    expect(selectedTitles()).toEqual([]);

    rerender(modal(NODES.map((n) => ({ ...n, latencyMs: 42 })) as unknown as NodeRecord[]));
    expect(selectedTitles()).toEqual([]);
  });

  it('resets the selection to the source node on each reopen', () => {
    const modal = (open: boolean) => (
      <ThemeProvider>
        <CloneInboundModal
          open={open}
          dbInbound={sourceInbound()}
          nodes={NODES}
          portsInUse={new Map()}
          onClose={() => {}}
          onCloned={() => {}}
        />
      </ThemeProvider>
    );
    const { rerender } = render(modal(true));

    fireEvent.click(screen.getByRole('button', { name: 'Clear all' }));
    expect(selectedTitles()).toEqual([]);

    rerender(modal(false));
    rerender(modal(true));
    expect(selectedTitles()).toEqual(['arm2']);
  });

  it('reports a partial failure with the backend reason and still closes', async () => {
    const { onCloned, onClose } = renderModal();
    postSpy.mockImplementation(async (_url, data) => {
      const body = data as PostBody;
      if (body.nodeId === 2) {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        return { success: false, msg: "port 23456 (tcp) already used by inbound 'x' (#1) on *" } as any;
      }
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      return { success: true, obj: {} } as any;
    });

    openTargetDropdown();
    clickOption('Local panel');
    clickOk();

    await screen.findByText(/port 23456 \(tcp\) already used/);
    await waitFor(() => expect(onCloned).toHaveBeenCalledTimes(1));
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
