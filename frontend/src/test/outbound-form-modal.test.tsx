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
