import { describe, it, expect, onTestFinished, vi } from 'vitest';
import { screen, act, render, cleanup, fireEvent, waitFor } from '@testing-library/react';

import InboundFormModal from '@/pages/inbounds/form/InboundFormModal';
import { DBInbound } from '@/models/dbinbound';
import { ThemeProvider } from '@/hooks/useTheme';
import { HttpUtil } from '@/utils';
import { Protocols } from '@/schemas/primitives';
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

// The protocol dropdown is virtualized, so jsdom renders only its first options; step
// the active option by keyboard from the current protocol in the options' own order.
function chooseVirtualizedProtocol(optionText: string) {
  const input = document.getElementById('protocol') as HTMLElement;
  const select = input.closest('.ant-select') as HTMLElement;
  const current = (select.textContent ?? '').trim();
  const order = Object.values(Protocols) as string[];
  const steps = order.indexOf(optionText) - order.indexOf(current);
  if (order.indexOf(optionText) < 0 || order.indexOf(current) < 0 || steps < 0) {
    throw new Error(`cannot step from '${current}' to '${optionText}'`);
  }
  fireEvent.mouseDown(select.querySelector('.ant-select-selector') ?? select);
  for (let i = 0; i < steps; i++) fireEvent.keyDown(input, { key: 'ArrowDown', keyCode: 40 });
  fireEvent.keyDown(input, { key: 'Enter', keyCode: 13 });
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
      await act(async () => {
        await new Promise((r) => setTimeout(r, 0));
      });
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
        dbInbound={
          new DBInbound({
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
          })
        }
        dbInbounds={[]}
        availableNodes={[]}
        onClose={() => {}}
        onSaved={() => {}}
      />,
    );

    const shareAddrInput = await screen.findByDisplayValue('edge.example.test');
    expect((shareAddrInput as HTMLInputElement).value).toBe('edge.example.test');
  });

  it('uses Hosts instead of showing the custom share address fields for MTProto', async () => {
    renderWithProviders(
      <InboundFormModal
        open
        mode="edit"
        dbInbound={
          new DBInbound({
            id: 2,
            port: 4060,
            listen: '',
            protocol: 'mtproto',
            remark: 'proxy',
            enable: true,
            settings: { clients: [] },
            streamSettings: {},
            sniffing: { enabled: false },
            nodeId: null,
            shareAddrStrategy: 'custom',
            shareAddr: 'proxy.example.test',
          })
        }
        dbInbounds={[]}
        availableNodes={[]}
        onClose={() => {}}
        onSaved={() => {}}
      />,
    );

    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    expect(fieldLabels()).not.toContain('Share address strategy');
    expect(screen.queryByDisplayValue('proxy.example.test')).toBeNull();
  });

  it('keeps the persisted node share strategy through the nodes-loading race (#5375)', async () => {
    const node = { id: 1, name: 'arm2', enable: true, status: 'online' } as never;
    const buildInbound = () =>
      new DBInbound({
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
    const flush = async () => {
      await act(async () => {
        await new Promise((r) => setTimeout(r, 0));
      });
    };
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

  it('blocks adding TLS without a certificate and directs the user to Security', async () => {
    const post = vi.mocked(HttpUtil.post);
    post.mockClear();
    messageError.mockClear();
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
    onTestFinished(() => consoleError.mockRestore());
    renderModal();

    fireEvent.click(screen.getByRole('tab', { name: 'Security' }));
    fireEvent.click(screen.getByRole('radio', { name: 'TLS' }));
    fireEvent.click(screen.getByRole('tab', { name: 'Basics' }));
    fireEvent.click(primaryButton());

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: 'Security' }).getAttribute('aria-selected')).toBe(
        'true',
      );
      expect(messageError).toHaveBeenCalledWith(
        expect.stringContaining('TLS certificate 1: Import a TLS certificate'),
      );
    });
    expect(consoleError).toHaveBeenCalledWith('[InboundFormModal] schema validation failed:', [
      'TLS certificate 1: Import a TLS certificate or enter its file path before saving',
      'TLS certificate 1: Import the TLS private key or enter its file path before saving',
    ]);
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

  // The core reads an XDRIVE inbound's TLS/REALITY as the storage API's client TLS, so
  // switching a REALITY inbound to XDRIVE must store security none with its secrets.
  it('switches a REALITY inbound to an XDRIVE stream with security none', async () => {
    const post = vi.mocked(HttpUtil.post);
    post.mockClear();
    renderCloneLikeEdit(cloneLikeVlessInbound('example.com:443'));

    fireEvent.click(screen.getByRole('tab', { name: 'Stream' }));
    const transmission = Array.from(document.querySelectorAll('.ant-form-item')).find(
      (el) =>
        el.querySelector('.ant-form-item-label label')?.textContent?.trim() === 'Transmission',
    );
    const select = transmission?.querySelector('.ant-select') as HTMLElement;
    fireEvent.mouseDown(select.querySelector('.ant-select-selector') ?? select);
    const option = Array.from(document.querySelectorAll('.ant-select-item-option')).find(
      (o) => (o.getAttribute('title') ?? o.textContent ?? '').trim() === 'XDRIVE',
    ) as HTMLElement;
    fireEvent.click(option);
    for (const [label, value] of [
      ['Remote folder', 'xray-tunnel'],
      ['Client ID', 'id.apps.googleusercontent.com'],
      ['Client secret', 'client-secret'],
      ['Refresh token', 'refresh-token'],
    ]) {
      fireEvent.change(await screen.findByLabelText(label), { target: { value } });
    }
    fireEvent.click(primaryButton());

    const isUpdate = ([url]: unknown[]) => url === '/panel/api/inbounds/update/42';
    await waitFor(() => expect(post.mock.calls.some(isUpdate)).toBe(true));
    const payload = post.mock.calls.find(isUpdate)![1] as { streamSettings: string };
    const stream = JSON.parse(payload.streamSettings) as Record<string, unknown>;
    expect(stream).toMatchObject({
      network: 'xdrive',
      security: 'none',
      xdriveSettings: {
        service: 'Google Drive',
        remoteFolder: 'xray-tunnel',
        secrets: ['id.apps.googleusercontent.com', 'client-secret', 'refresh-token'],
      },
    });
    expect(stream.realitySettings).toBeUndefined();
  });

  // xray-core runs MASQUE only on its own transport behind TLS, and leases tunnel
  // addresses from settings.address; picking the protocol must seed all three.
  it('adds a MASQUE inbound on its own transport with TLS and an address pool', async () => {
    const post = vi.mocked(HttpUtil.post);
    post.mockClear();
    renderModal();

    chooseVirtualizedProtocol('masque');
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
    fireEvent.click(screen.getByRole('tab', { name: 'Security' }));
    expect(screen.queryByRole('radio', { name: 'None' })).toBeNull();
    fireEvent.change(await screen.findByLabelText('Public Key'), {
      target: { value: '/etc/ssl/certs/m.crt' },
    });
    fireEvent.change(screen.getByLabelText('Private Key'), {
      target: { value: '/etc/ssl/private/m.key' },
    });
    fireEvent.click(primaryButton());

    const isAdd = ([url]: unknown[]) => url === '/panel/api/inbounds/add';
    await waitFor(() => expect(post.mock.calls.some(isAdd)).toBe(true));
    const payload = post.mock.calls.find(isAdd)![1] as {
      protocol: string;
      settings: string;
      streamSettings: string;
    };
    expect(payload.protocol).toBe('masque');
    expect(JSON.parse(payload.settings)).toMatchObject({ address: ['10.14.0.1/24', 'fd14::1/64'] });
    expect(JSON.parse(payload.streamSettings)).toMatchObject({
      network: 'masque',
      security: 'tls',
      masqueSettings: { path: '/.well-known/masque/ip/*/*/' },
      tlsSettings: { alpn: ['h3'] },
    });
  });

  // xray-core refuses a udphop mask on a listener ("client only"), so an inbound keeps its
  // advertised hop range in quicParams.udpHop: the client-side lift must not run here.
  it('keeps an inbound hop range in quicParams.udpHop on save', async () => {
    const post = vi.mocked(HttpUtil.post);
    post.mockClear();
    const hysteria = new DBInbound({
      id: 43,
      port: 443,
      listen: '',
      protocol: 'hysteria',
      remark: 'hy',
      enable: true,
      settings: { version: 2, clients: [] },
      streamSettings: {
        network: 'hysteria',
        security: 'tls',
        hysteriaSettings: { version: 2, auth: '', udpIdleTimeout: 60 },
        tlsSettings: {
          serverName: 'hy.example.com',
          alpn: ['h3'],
          certificates: [
            { certificateFile: '/etc/ssl/certs/hy.crt', keyFile: '/etc/ssl/private/hy.key' },
          ],
        },
        finalmask: { quicParams: { udpHop: { ports: '20000-30000', interval: '5-10' } } },
      },
      sniffing: { enabled: false },
      nodeId: null,
      shareAddrStrategy: 'listen',
      shareAddr: '',
    });
    renderCloneLikeEdit(hysteria);
    // Form watches flush in a macrotask; save only once the hop fields have emitted.
    await screen.findByDisplayValue('20000-30000');
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    fireEvent.click(primaryButton());

    const isUpdate = ([url]: unknown[]) => url === '/panel/api/inbounds/update/43';
    await waitFor(() => expect(post.mock.calls.some(isUpdate)).toBe(true));
    const payload = post.mock.calls.find(isUpdate)![1] as { streamSettings: string };
    const finalmask = JSON.parse(payload.streamSettings).finalmask as Record<string, unknown>;
    expect(finalmask.quicParams).toMatchObject({
      udpHop: { ports: '20000-30000', interval: '5-10' },
    });
    expect(JSON.stringify(finalmask.udp ?? [])).not.toContain('udphop');
  });

  // xray-core 26.9: the Wintun adapter type (desc) and the TUN auto-system leak guards.
  it('saves the TUN adapter type and auto-system options set in the form', async () => {
    const post = vi.mocked(HttpUtil.post);
    post.mockClear();
    const tun = new DBInbound({
      id: 44,
      port: 0,
      listen: '',
      protocol: 'tun',
      remark: 'tun',
      enable: true,
      settings: {
        name: 'xray0',
        mtu: 1500,
        gateway: ['10.0.0.1/16'],
        dns: ['1.1.1.1'],
        userLevel: 0,
        autoSystemRoutingTable: ['0.0.0.0/0'],
        autoOutboundsInterface: 'auto',
      },
      streamSettings: {},
      sniffing: { enabled: false },
      nodeId: null,
      shareAddrStrategy: 'listen',
      shareAddr: '',
    });
    renderCloneLikeEdit(tun);

    fireEvent.change(document.getElementById('tun-desc')!, { target: { value: 'MyTun' } });
    fireEvent.click(document.getElementById('tun-auto-dns-gateway')!);
    chooseSelectOption('tun-wfp-block-leak', 'dns');
    fireEvent.click(primaryButton());

    const isUpdate = ([url]: unknown[]) => url === '/panel/api/inbounds/update/44';
    await waitFor(() => expect(post.mock.calls.some(isUpdate)).toBe(true));
    const payload = post.mock.calls.find(isUpdate)![1] as { settings: string };
    expect(JSON.parse(payload.settings)).toMatchObject({
      desc: 'MyTun',
      autoSystemDnsToGateway: true,
      autoSystemWfpBlockLeak: ['dns'],
    });
  });

  // Clients and enable change through their own endpoints; the server keeps the
  // stored ones, so the edit form must neither send nor validate its stale copy.
  it('edit save neither sends nor validates the clients it loaded', async () => {
    const post = vi.mocked(HttpUtil.post);
    post.mockClear();
    const dbInbound = cloneLikeVlessInbound('example.com:443');
    const legacy = new DBInbound({
      ...dbInbound,
      settings: {
        ...(dbInbound.settings as Record<string, unknown>),
        clients: [{ email: 'legacy', id: '' }],
      },
    });
    renderCloneLikeEdit(legacy);

    fireEvent.click(primaryButton());

    await waitFor(() => expect(post).toHaveBeenCalled());
    const payload = post.mock.calls[0][1] as { settings: string };
    expect(JSON.parse(payload.settings)).not.toHaveProperty('clients');
  });

  it('offers the enable switch when adding an inbound but not when editing one', () => {
    renderModal();
    expect(document.getElementById('inbound-enable')).not.toBeNull();
    cleanup();
    renderCloneLikeEdit(cloneLikeVlessInbound('example.com:443'));
    expect(document.getElementById('inbound-enable')).toBeNull();
  });
});
