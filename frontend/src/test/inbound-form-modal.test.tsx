import { describe, it, expect, vi } from 'vitest';
import { screen, act, render, cleanup, fireEvent, waitFor } from '@testing-library/react';

import InboundFormModal from '@/pages/inbounds/form/InboundFormModal';
import { DBInbound } from '@/models/dbinbound';
import { ThemeProvider } from '@/hooks/useTheme';
import { HttpUtil } from '@/utils';
import {
  renderWithProviders,
  fieldLabels,
  listSelectOptions,
  chooseSelectOption,
} from './test-utils';

const { messageError } = vi.hoisted(() => ({ messageError: vi.fn() }));

vi.mock('antd', async (importOriginal) => {
  const actual = await importOriginal<typeof import('antd')>();
  return {
    ...actual,
    message: {
      ...actual.message,
      useMessage: () => [{ error: messageError }, null],
    },
  };
});

function renderModal() {
  return renderWithProviders(
    <InboundFormModal
      open
      mode="add"
      dbInbound={null}
      dbInbounds={[]}
      availableNodes={[]}
      onClose={() => {}}
      onSaved={() => {}}
    />,
  );
}

function primaryButton(): HTMLElement {
  const button = document.querySelector('.ant-modal-footer .ant-btn-primary');
  if (!button) throw new Error('Primary modal button not found');
  return button as HTMLElement;
}

function cloneLikeVlessInbound(target: string) {
  return new DBInbound({
    id: 42,
    port: 41234,
    listen: '',
    protocol: 'vless',
    remark: 'source clone',
    enable: false,
    settings: {
      clients: [],
      decryption: 'none',
      encryption: 'none',
      fallbacks: [],
    },
    streamSettings: {
      network: 'tcp',
      security: 'reality',
      tcpSettings: { header: { type: 'none' } },
      realitySettings: {
        target,
        serverNames: ['example.com'],
        privateKey: 'test-private-key',
        shortIds: ['abcd'],
        settings: {
          publicKey: 'test-public-key',
          fingerprint: 'chrome',
          spiderX: '/',
        },
      },
    },
    sniffing: { enabled: false },
    nodeId: null,
    shareAddrStrategy: 'listen',
    shareAddr: '',
  });
}

function renderCloneLikeEdit(dbInbound: DBInbound) {
  renderWithProviders(
    <InboundFormModal
      open
      mode="edit"
      dbInbound={dbInbound}
      dbInbounds={[dbInbound]}
      availableNodes={[]}
      onClose={() => {}}
      onSaved={() => {}}
    />,
  );
}

describe('InboundFormModal', () => {
  it('renders add mode without crashing', () => {
    renderModal();
    expect(document.querySelector('.ant-modal')).toBeTruthy();
    expect(fieldLabels().length).toBeGreaterThan(0);
  });

  it('field structure differs per protocol (not a vacuous snapshot loop)', async () => {
    renderModal();
    const protocols = listSelectOptions('protocol');
    expect(protocols.length).toBeGreaterThan(3);

    const labelsByProto: Record<string, string[]> = {};
    for (const proto of protocols) {
      chooseSelectOption('protocol', proto);
      // Flush antd Form.useWatch('protocol') before reading — without it every iteration
      // sees the same pre-update DOM and the loop asserts nothing (the original bug here).
      await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
      labelsByProto[proto] = fieldLabels();
    }

    // The loop must actually exercise protocol-specific rendering: distinct protocols
    // must yield distinct field sets (a vacuous loop makes them all identical).
    const distinctShapes = new Set(Object.values(labelsByProto).map((l) => l.join('|')));
    expect(distinctShapes.size).toBeGreaterThan(1);

    // Spot-check a protocol-distinguishing field that must appear after the switch.
    if (labelsByProto.shadowsocks) {
      expect(labelsByProto.shadowsocks).toContain('Encryption method');
    }
  }, 30000); // iterates every protocol, re-rendering a heavy modal each time — slow on CI runners

  it('preserves custom share address strategy when editing a local inbound', async () => {
    renderWithProviders(
      <InboundFormModal
        open
        mode="edit"
        dbInbound={new DBInbound({
          id: 1,
          port: 12345,
          listen: '',
          protocol: 'shadowsocks',
          remark: 'edge',
          enable: true,
          settings: {
            method: '2022-blake3-aes-128-gcm',
            password: 'server-password',
            network: 'tcp,udp',
            clients: [],
          },
          streamSettings: { network: 'tcp', security: 'none', tcpSettings: {} },
          sniffing: { enabled: false },
          nodeId: null,
          shareAddrStrategy: 'custom',
          shareAddr: 'edge.example.test',
        })}
        dbInbounds={[]}
        availableNodes={[]}
        onClose={() => {}}
        onSaved={() => {}}
      />,
    );

    const shareAddrInput = await screen.findByDisplayValue('edge.example.test');
    expect((shareAddrInput as HTMLInputElement).value).toBe('edge.example.test');
  });

  it('keeps the persisted node share strategy through the nodes-loading race (#5375)', async () => {
    const node = { id: 1, name: 'arm2', enable: true, status: 'online' } as never;
    const buildInbound = () => new DBInbound({
      id: 1,
      port: 23456,
      listen: '',
      protocol: 'vless',
      remark: 'noded',
      enable: true,
      settings: { clients: [] },
      streamSettings: { network: 'tcp', security: 'none', tcpSettings: {} },
      sniffing: { enabled: false },
      nodeId: 1,
      shareAddrStrategy: 'node',
    });
    const flush = async () => { await act(async () => { await new Promise((r) => setTimeout(r, 0)); }); };
    const strategyItem = (title: string) =>
      document.querySelector(`.ant-select-content[title="${title}"]`);
    const modal = (nodes: never[], fetched: boolean) => (
      <ThemeProvider>
        <InboundFormModal
          open
          mode="edit"
          dbInbound={buildInbound()}
          dbInbounds={[]}
          availableNodes={nodes}
          availableNodesFetched={fetched}
          onClose={() => {}}
          onSaved={() => {}}
        />
      </ThemeProvider>
    );

    // Baseline: nodes already loaded, so the node option is offered and selected.
    render(modal([node], true));
    await flush();
    expect(strategyItem('Node address')).toBeTruthy();
    cleanup();

    // Race: the modal mounts before /nodes/list resolves (empty placeholder),
    // then nodes arrive. The persisted 'node' strategy must survive the gap and
    // stay selected once the option reappears — not silently revert to listen.
    const { rerender } = render(modal([], false));
    await flush();
    rerender(modal([node], true));
    await flush();
    expect(strategyItem('Node address')).toBeTruthy();
    expect(strategyItem('Inbound listen')).toBeFalsy();
  });

  it('surfaces a Reality validation error and switches to its tab', async () => {
    const post = vi.mocked(HttpUtil.post);
    post.mockClear();
    messageError.mockClear();
    renderCloneLikeEdit(cloneLikeVlessInbound('example.com'));

    fireEvent.click(primaryButton());

    await waitFor(() => {
      const securityTab = screen.getByRole('tab', { name: 'Security' });
      expect(securityTab.getAttribute('aria-selected')).toBe('true');
    });
    expect(messageError).toHaveBeenCalledWith(
      expect.stringContaining('REALITY target must include a port'),
    );
    expect(post).not.toHaveBeenCalled();
  });

  it('submits a valid clone-like Reality inbound', async () => {
    const post = vi.mocked(HttpUtil.post);
    post.mockClear();
    renderCloneLikeEdit(cloneLikeVlessInbound('example.com:443'));

    fireEvent.click(primaryButton());

    await waitFor(() => {
      expect(post).toHaveBeenCalledWith(
        '/panel/api/inbounds/update/42',
        expect.objectContaining({ enable: false, port: 41234, protocol: 'vless' }),
      );
    });
  });
});
