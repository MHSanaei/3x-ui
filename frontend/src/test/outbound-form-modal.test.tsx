import { describe, it, expect, vi } from 'vitest';
import { act, fireEvent, render, screen } from '@testing-library/react';
import { QueryClientProvider } from '@tanstack/react-query';

import { ThemeProvider } from '@/hooks/useTheme';
import OutboundFormModal from '@/pages/xray/outbounds/OutboundFormModal';
import {
  renderWithProviders,
  fieldLabels,
  listSelectOptions,
  chooseSelectOption,
  makeTestQueryClient,
} from './test-utils';

function renderModal(outbound: Record<string, unknown> | null = null) {
  return renderWithProviders(
    <OutboundFormModal
      open
      outbound={outbound}
      existingTags={[]}
      onClose={() => {}}
      onConfirm={() => {}}
    />,
  );
}

function formItemByLabel(label: string): Element | undefined {
  return Array.from(document.querySelectorAll('.ant-form-item')).find(
    (el) => (el.querySelector('.ant-form-item-label label')?.textContent ?? '').trim() === label,
  );
}

function chooseOptionByLabel(label: string, optionText: string) {
  const select = formItemByLabel(label)?.querySelector('.ant-select');
  if (!select) throw new Error(`${label} select not found`);
  fireEvent.mouseDown(select.querySelector('.ant-select-selector') ?? select);
  const option = Array.from(document.querySelectorAll('.ant-select-item-option')).find(
    (o) => (o.getAttribute('title') ?? o.textContent ?? '').trim() === optionText,
  );
  if (!option) throw new Error(`Option '${optionText}' not found for ${label}`);
  fireEvent.click(option);
}

function toggleSwitch(label: string) {
  const item = Array.from(document.querySelectorAll('.ant-form-item')).find(
    (el) => (el.querySelector('.ant-form-item-label label')?.textContent ?? '').trim() === label,
  );
  const control = item?.querySelector('.ant-switch');
  if (!control) throw new Error(`${label} switch not found`);
  fireEvent.click(control);
}

describe('OutboundFormModal', () => {
  it('renders add mode without crashing', () => {
    renderModal(null);
    expect(document.querySelector('.ant-modal')).toBeTruthy();
    expect(fieldLabels().length).toBeGreaterThan(0);
  });

  it('field structure differs per protocol (not a vacuous snapshot loop)', async () => {
    renderModal(null);
    const protocols = listSelectOptions('protocol');
    expect(protocols.length).toBeGreaterThan(3);

    const labelsByProto: Record<string, string[]> = {};
    for (const proto of protocols) {
      chooseSelectOption('protocol', proto);
      // Flush antd Form.useWatch('protocol') so protocol-specific fields render before
      // reading; otherwise every iteration sees the same default (vless) DOM.
      await act(async () => {
        await new Promise((r) => setTimeout(r, 0));
      });
      labelsByProto[proto] = fieldLabels();
    }

    // Distinct protocols must yield distinct field sets (a vacuous loop is all-identical).
    const distinctShapes = new Set(Object.values(labelsByProto).map((l) => l.join('|')));
    expect(distinctShapes.size).toBeGreaterThan(1);

    // vless carries an Encryption field; wireguard does not — proves real protocol switching.
    if (labelsByProto.vless) {
      expect(labelsByProto.vless).toContain('Encryption');
    }
    if (labelsByProto.wireguard) {
      expect(labelsByProto.wireguard).not.toContain('Encryption');
    }
  }, 30000); // iterates every protocol, re-rendering a heavy modal each time — slow on CI runners

  // Freedom's card and the Transport tab's Sockopts block both write
  // sockopt.domainStrategy, so freedom must show only one control for it.
  it('hides the Transport sockopt strategy for freedom', () => {
    renderModal({ protocol: 'freedom', tag: 'direct', settings: {} });
    toggleSwitch('Sockopts');

    expect(fieldLabels()).toContain('Sockopts');
    expect(fieldLabels()).not.toContain('Domain Strategy');
    expect(fieldLabels()).toContain('Strategy');
  });

  it('keeps the Transport sockopt strategy for protocols without a card field', () => {
    renderModal({ protocol: 'vless', tag: 'proxy', settings: {} });
    toggleSwitch('Sockopts');

    expect(fieldLabels()).toContain('Domain Strategy');
  });

  // The core migrates freedom's outbound-root targetStrategy into the very same
  // sockopt.domainStrategy the card writes, so the modal must not offer both.
  it('offers freedom one strategy knob and other protocols the root one', async () => {
    renderModal(null);

    chooseSelectOption('protocol', 'freedom');
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
    const freedomLabels = fieldLabels();
    expect(freedomLabels).toContain('Strategy');
    expect(freedomLabels).not.toContain('Target Strategy');

    chooseSelectOption('protocol', 'vless');
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
    expect(fieldLabels()).toContain('Target Strategy');
  });

  it('saves a vless reverse outbound while reverse sniffing stays disabled', async () => {
    const onConfirm = vi.fn();
    renderWithProviders(
      <OutboundFormModal
        open
        outbound={{
          protocol: 'vless',
          tag: 'reverse-out',
          settings: {
            vnext: [
              {
                address: 'example.com',
                port: 443,
                users: [{ id: 'c9f0c2d0-0000-4000-8000-000000000000', encryption: 'none' }],
              },
            ],
            reverse: { tag: 'r1', sniffing: {} },
          },
          streamSettings: { network: 'tcp', security: 'none' },
        }}
        existingTags={[]}
        onClose={() => {}}
        onConfirm={onConfirm}
      />,
    );
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
    expect(ok).toBeTruthy();
    await act(async () => {
      fireEvent.click(ok);
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    expect(onConfirm).toHaveBeenCalledTimes(1);
    const payload = onConfirm.mock.calls[0][0] as {
      settings: { reverse?: { tag?: string } };
    };
    expect(payload.settings.reverse?.tag).toBe('r1');
  });

  // xray-core 26.10.10 reads a 26.9.30 xdns mask as no domain at all, so the mask editor
  // lifts it to names/addrs on open and must keep those keys registered through the save.
  it('saves a legacy xdns mask in the names/addrs shape', async () => {
    const onConfirm = vi.fn();
    const queryClient = makeTestQueryClient();
    const outbound = {
      protocol: 'vless',
      tag: 'dns-tunnel',
      settings: {
        vnext: [
          {
            address: 'example.com',
            port: 53,
            users: [{ id: 'c9f0c2d0-0000-4000-8000-000000000000', encryption: 'none' }],
          },
        ],
      },
      streamSettings: {
        network: 'kcp',
        security: 'none',
        kcpSettings: { mtu: 130, tti: 50 },
        finalmask: {
          udp: [
            {
              type: 'xdns',
              settings: {
                domains: [{ name: 't.example.com', types: [16], edns0: 1232 }],
                resolvers: [{ type: 'udp', settings: { addr: '8.8.8.8:53' } }],
              },
            },
          ],
        },
      },
    };
    const tree = (open: boolean) => (
      <QueryClientProvider client={queryClient}>
        <ThemeProvider>
          <OutboundFormModal
            open={open}
            outbound={outbound}
            existingTags={[]}
            onClose={() => {}}
            onConfirm={onConfirm}
          />
        </ThemeProvider>
      </QueryClientProvider>
    );
    // The panel keeps the modal mounted and flips `open`; mounting it already open lets
    // the add-mode transport seed replace the edited outbound's kcp stream with tcp.
    const { rerender } = render(tree(false));
    rerender(tree(true));
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
    await act(async () => {
      fireEvent.click(ok);
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    expect(onConfirm).toHaveBeenCalledTimes(1);
    const payload = onConfirm.mock.calls[0][0] as {
      streamSettings: { finalmask: { udp: Array<{ type: string; settings: unknown }> } };
    };
    expect(payload.streamSettings.finalmask.udp).toEqual([
      {
        type: 'xdns',
        settings: {
          domains: [{ names: ['t.example.com'], types: [16], edns0: 1232 }],
          resolvers: [{ addrs: ['udp://8.8.8.8:53'] }],
        },
      },
    ]);
  });

  // xray-core 26.10.10 verifies TLS against its bundled roots, not the OS store, so an
  // outbound to a server signed by a privately installed CA needs useSystemCA set.
  it('saves useSystemCA from the TLS switch', async () => {
    const onConfirm = vi.fn();
    const queryClient = makeTestQueryClient();
    const outbound = {
      protocol: 'vless',
      tag: 'tls-out',
      settings: {
        vnext: [
          {
            address: 'example.com',
            port: 443,
            users: [{ id: 'c9f0c2d0-0000-4000-8000-000000000000', encryption: 'none' }],
          },
        ],
      },
      streamSettings: {
        network: 'tcp',
        security: 'tls',
        tlsSettings: { serverName: 'example.com' },
      },
    };
    const tree = (open: boolean) => (
      <QueryClientProvider client={queryClient}>
        <ThemeProvider>
          <OutboundFormModal
            open={open}
            outbound={outbound}
            existingTags={[]}
            onClose={() => {}}
            onConfirm={onConfirm}
          />
        </ThemeProvider>
      </QueryClientProvider>
    );
    const { rerender } = render(tree(false));
    rerender(tree(true));
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    toggleSwitch('Use system CA');
    const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
    await act(async () => {
      fireEvent.click(ok);
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    expect(onConfirm).toHaveBeenCalledTimes(1);
    const payload = onConfirm.mock.calls[0][0] as {
      streamSettings: { tlsSettings: { serverName?: string; useSystemCA?: boolean } };
    };
    expect(payload.streamSettings.tlsSettings).toMatchObject({
      serverName: 'example.com',
      useSystemCA: true,
    });
  });

  // xray-core reads Google Drive's three secrets by position (ClientID, ClientSecret,
  // RefreshToken); a misordered list fails OAuth only when the tunnel first dials.
  it('gates Flow and Mux on the flow the transport can actually carry', async () => {
    const vless = (encryption: string, streamSettings: Record<string, unknown>) => ({
      protocol: 'vless',
      tag: 'flow-out',
      settings: {
        address: 'example.com',
        port: 443,
        id: 'c9f0c2d0-0000-4000-8000-000000000000',
        flow: 'xtls-rprx-vision',
        encryption,
      },
      streamSettings,
    });
    const openModal = async (outbound: Record<string, unknown>) => {
      const queryClient = makeTestQueryClient();
      const tree = (open: boolean) => (
        <QueryClientProvider client={queryClient}>
          <ThemeProvider>
            <OutboundFormModal
              open={open}
              outbound={outbound}
              existingTags={[]}
              onClose={() => {}}
              onConfirm={() => {}}
            />
          </ThemeProvider>
        </QueryClientProvider>
      );
      const view = render(tree(false));
      view.rerender(tree(true));
      await act(async () => {
        await new Promise((r) => setTimeout(r, 0));
      });
      return view;
    };

    const ws = await openModal(
      vless('none', { network: 'ws', security: 'tls', wsSettings: { path: '/' } }),
    );
    expect(formItemByLabel('Transmission')?.textContent).toContain('WebSocket');
    expect(formItemByLabel('Flow')).toBeUndefined();
    expect(formItemByLabel('Mux')).toBeDefined();
    ws.unmount();

    await openModal(
      vless('mlkem768x25519plus.native.0rtt.key', {
        network: 'xhttp',
        security: 'none',
        xhttpSettings: { path: '/', mode: 'auto' },
      }),
    );
    expect(formItemByLabel('Flow')).toBeDefined();
  });

  it('saves an XDRIVE stream picked in the transmission selector', async () => {
    const onConfirm = vi.fn();
    const queryClient = makeTestQueryClient();
    const outbound = {
      protocol: 'vless',
      tag: 'drive-out',
      settings: {
        vnext: [
          {
            address: 'example.com',
            port: 443,
            users: [{ id: 'c9f0c2d0-0000-4000-8000-000000000000', encryption: 'none' }],
          },
        ],
      },
      streamSettings: { network: 'tcp', security: 'none' },
    };
    const tree = (open: boolean) => (
      <QueryClientProvider client={queryClient}>
        <ThemeProvider>
          <OutboundFormModal
            open={open}
            outbound={outbound}
            existingTags={[]}
            onClose={() => {}}
            onConfirm={onConfirm}
          />
        </ThemeProvider>
      </QueryClientProvider>
    );
    const { rerender } = render(tree(false));
    rerender(tree(true));
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    chooseOptionByLabel('Transmission', 'XDRIVE');
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
    for (const [label, value] of [
      ['Remote folder', 'xray-tunnel'],
      ['Client ID', 'id.apps.googleusercontent.com'],
      ['Client secret', 'client-secret'],
      ['Refresh token', 'refresh-token'],
    ]) {
      fireEvent.change(screen.getByLabelText(label), { target: { value } });
    }
    const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
    await act(async () => {
      fireEvent.click(ok);
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    expect(onConfirm).toHaveBeenCalledTimes(1);
    const payload = onConfirm.mock.calls[0][0] as { streamSettings: Record<string, unknown> };
    expect(payload.streamSettings).toMatchObject({
      network: 'xdrive',
      xdriveSettings: {
        service: 'Google Drive',
        remoteFolder: 'xray-tunnel',
        secrets: ['id.apps.googleusercontent.com', 'client-secret', 'refresh-token'],
      },
    });
  });
});

// xray-core 26.10.10 dials the XDRIVE storage API with the outbound's tlsSettings (SNI,
// uTLS fingerprint), so switching a TLS outbound to XDRIVE must keep them.
describe('OutboundFormModal XDRIVE TLS', () => {
  it('keeps TLS when a TLS outbound moves to XDRIVE', async () => {
    const onConfirm = vi.fn();
    const queryClient = makeTestQueryClient();
    const outbound = {
      protocol: 'vless',
      tag: 'drive-out',
      settings: {
        vnext: [
          {
            address: 'www.google.com',
            port: 443,
            users: [{ id: 'c9f0c2d0-0000-4000-8000-000000000000', encryption: 'none' }],
          },
        ],
      },
      streamSettings: {
        network: 'tcp',
        security: 'tls',
        tlsSettings: { serverName: 'www.google.com', fingerprint: 'chrome' },
      },
    };
    const tree = (open: boolean) => (
      <QueryClientProvider client={queryClient}>
        <ThemeProvider>
          <OutboundFormModal
            open={open}
            outbound={outbound}
            existingTags={[]}
            onClose={() => {}}
            onConfirm={onConfirm}
          />
        </ThemeProvider>
      </QueryClientProvider>
    );
    const { rerender } = render(tree(false));
    rerender(tree(true));
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    chooseOptionByLabel('Transmission', 'XDRIVE');
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
    expect((screen.getByRole('radio', { name: 'TLS' }) as HTMLInputElement).checked).toBe(true);
    for (const [label, value] of [
      ['Remote folder', 'xray-tunnel'],
      ['Client ID', 'id.apps.googleusercontent.com'],
      ['Client secret', 'client-secret'],
      ['Refresh token', 'refresh-token'],
    ]) {
      fireEvent.change(screen.getByLabelText(label), { target: { value } });
    }
    const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
    await act(async () => {
      fireEvent.click(ok);
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    expect(onConfirm).toHaveBeenCalledTimes(1);
    const payload = onConfirm.mock.calls[0][0] as { streamSettings: Record<string, unknown> };
    expect(payload.streamSettings).toMatchObject({
      network: 'xdrive',
      security: 'tls',
      tlsSettings: { serverName: 'www.google.com', fingerprint: 'chrome' },
    });
  });
});

describe('OutboundFormModal MASQUE', () => {
  // xray-core refuses user/pass next to warp, and builds the WARP client certificate
  // from privateKey; a save that kept stale credentials or dropped a key never connects.
  it('saves a WARP-over-MASQUE outbound without Basic-auth credentials', async () => {
    const onConfirm = vi.fn();
    const queryClient = makeTestQueryClient();
    const outbound = {
      protocol: 'masque',
      tag: 'warp-masque',
      settings: { address: '162.159.198.1', port: 443 },
      streamSettings: {
        network: 'masque',
        security: 'tls',
        tlsSettings: { serverName: '' },
        masqueSettings: { user: 'old-user', pass: 'old-pass' },
      },
    };
    const tree = (open: boolean) => (
      <QueryClientProvider client={queryClient}>
        <ThemeProvider>
          <OutboundFormModal
            open={open}
            outbound={outbound}
            existingTags={[]}
            onClose={() => {}}
            onConfirm={onConfirm}
          />
        </ThemeProvider>
      </QueryClientProvider>
    );
    const { rerender } = render(tree(false));
    rerender(tree(true));
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    toggleSwitch('WARP');
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
    fireEvent.change(screen.getByLabelText('Private key'), { target: { value: 'PRIVATE-PEM' } });
    fireEvent.change(screen.getByLabelText('Endpoint public key'), {
      target: { value: 'PUBLIC-PEM' },
    });
    const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
    await act(async () => {
      fireEvent.click(ok);
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    expect(onConfirm).toHaveBeenCalledTimes(1);
    const payload = onConfirm.mock.calls[0][0] as {
      protocol: string;
      settings: Record<string, unknown>;
      streamSettings: {
        network: string;
        security: string;
        masqueSettings: Record<string, unknown>;
      };
    };
    expect(payload.protocol).toBe('masque');
    expect(payload.settings).toMatchObject({ address: '162.159.198.1', port: 443 });
    expect(payload.streamSettings).toMatchObject({ network: 'masque', security: 'tls' });
    expect(payload.streamSettings.masqueSettings.warp).toMatchObject({
      privateKey: 'PRIVATE-PEM',
      publicKey: 'PUBLIC-PEM',
    });
    expect(payload.streamSettings.masqueSettings.user).toBeUndefined();
    expect(payload.streamSettings.masqueSettings.pass).toBeUndefined();
  });
});

describe('OutboundFormModal UDP hop', () => {
  // xray-core 26.9.9 ignores quicParams.udpHop; a client hops only through the udphop
  // mask, so the editor must lift the dead key and keep every mask field on save.
  it('saves a legacy quicParams.udpHop hysteria outbound as a udphop mask', async () => {
    const onConfirm = vi.fn();
    const queryClient = makeTestQueryClient();
    const outbound = {
      protocol: 'hysteria',
      tag: 'hy-out',
      settings: { address: 'hy.example.com', port: 443, version: 2 },
      streamSettings: {
        network: 'hysteria',
        security: 'tls',
        hysteriaSettings: { version: 2, auth: 'a', udpIdleTimeout: 60 },
        tlsSettings: { serverName: 'hy.example.com', alpn: ['h3'] },
        finalmask: {
          udp: [{ type: 'salamander', settings: { password: 'p' } }],
          quicParams: { congestion: 'bbr', udpHop: { ports: '20000-30000', interval: '5-10' } },
        },
      },
    };
    const tree = (open: boolean) => (
      <QueryClientProvider client={queryClient}>
        <ThemeProvider>
          <OutboundFormModal
            open={open}
            outbound={outbound}
            existingTags={[]}
            onClose={() => {}}
            onConfirm={onConfirm}
          />
        </ThemeProvider>
      </QueryClientProvider>
    );
    const { rerender } = render(tree(false));
    rerender(tree(true));
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
    await act(async () => {
      fireEvent.click(ok);
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    expect(onConfirm).toHaveBeenCalledTimes(1);
    const payload = onConfirm.mock.calls[0][0] as {
      streamSettings: { finalmask: { udp: unknown[]; quicParams?: Record<string, unknown> } };
    };
    expect(payload.streamSettings.finalmask.udp).toEqual([
      { type: 'salamander', settings: { password: 'p' } },
      {
        type: 'udphop',
        settings: { mode: 'intervalRemote', remotePorts: '20000-30000', interval: '5-10' },
      },
    ]);
    expect(payload.streamSettings.finalmask.quicParams?.udpHop).toBeUndefined();
    expect(payload.streamSettings.finalmask.quicParams?.congestion).toBe('bbr');
  });
});

describe('OutboundFormModal finalmask variables', () => {
  function xhttpOutbound(finalmask: Record<string, unknown>) {
    return {
      protocol: 'vless',
      tag: 'masked',
      settings: {
        vnext: [
          {
            address: 'example.com',
            port: 443,
            users: [{ id: 'c9f0c2d0-0000-4000-8000-000000000000', encryption: 'none' }],
          },
        ],
      },
      streamSettings: {
        network: 'xhttp',
        security: 'none',
        xhttpSettings: { path: '/', mode: 'auto' },
        finalmask,
      },
    };
  }

  async function saveFinalmask(
    finalmask: Record<string, unknown>,
    edit: () => void = () => {},
  ): Promise<Record<string, unknown>> {
    const onConfirm = vi.fn();
    const queryClient = makeTestQueryClient();
    const tree = (open: boolean) => (
      <QueryClientProvider client={queryClient}>
        <ThemeProvider>
          <OutboundFormModal
            open={open}
            outbound={xhttpOutbound(finalmask)}
            existingTags={[]}
            onClose={() => {}}
            onConfirm={onConfirm}
          />
        </ThemeProvider>
      </QueryClientProvider>
    );
    const { rerender } = render(tree(false));
    rerender(tree(true));
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
    edit();
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
    await act(async () => {
      fireEvent.click(ok);
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
    expect(onConfirm).toHaveBeenCalledTimes(1);
    return (
      onConfirm.mock.calls[0][0] as { streamSettings: { finalmask: Record<string, unknown> } }
    ).streamSettings.finalmask;
  }

  function typeInto(label: string, value: string) {
    const input = formItemByLabel(label)?.querySelector('input, textarea');
    if (!input) throw new Error(`${label} input not found`);
    fireEvent.change(input, { target: { value } });
  }

  // xray-core 26.6 header-custom variables, error replies and standalone UDP headers, plus
  // the sudoku UDP mask; the same fixture is built by Go's TestGoldenStreamFixturesBuildInXray.
  it('keeps every header-custom variable, sudoku UDP mask and noise reset range on save', async () => {
    const fixture = (await import('./golden/fixtures/finalmask/header-custom-variables.json'))
      .default as Record<string, unknown>;

    expect(await saveFinalmask(structuredClone(fixture))).toEqual(fixture);
  });

  // The core rejects an item carrying two kinds ("exactly one item kind must be set").
  it('drops the bytes of an item switched to reuse a variable', async () => {
    const finalmask = await saveFinalmask(
      {
        udp: [
          {
            type: 'header-custom',
            settings: { client: [{ type: 'array', rand: 4, randRange: '0-255' }], server: [] },
          },
        ],
      },
      () => {
        chooseOptionByLabel('Kind', 'Reuse variable');
        typeInto('Reuse', 'nonce');
      },
    );

    expect(finalmask.udp).toEqual([
      { type: 'header-custom', settings: { client: [{ reuse: 'nonce' }], server: [] } },
    ]);
  });

  // A string transform fails the whole core config, so half-typed JSON must never be saved.
  it.each([
    ['{"op": "be16", "args": [{"u64": 7}]}', [{ transform: { op: 'be16', args: [{ u64: 7 }] } }]],
    ['{"op": "be16"', [{}]],
  ])('saves the typed transform %s only once it parses', async (typed, client) => {
    const finalmask = await saveFinalmask(
      {
        udp: [
          { type: 'header-custom', settings: { client: [{ type: 'array', rand: 4 }], server: [] } },
        ],
      },
      () => {
        chooseOptionByLabel('Kind', 'Transform');
        typeInto('Transform', typed);
      },
    );

    expect(finalmask.udp).toEqual([{ type: 'header-custom', settings: { client, server: [] } }]);
  });

  it('saves a noise reset typed as a range', async () => {
    const finalmask = await saveFinalmask(
      { udp: [{ type: 'noise', settings: { reset: 0, noise: [] } }] },
      () => typeInto('Reset', '60-120'),
    );

    expect(finalmask.udp).toEqual([{ type: 'noise', settings: { reset: '60-120', noise: [] } }]);
  });
});
