import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';

import PiaModal from '@/pages/xray/overrides/PiaModal';
import { HttpUtil, Msg } from '@/utils';
import { renderWithProviders } from './test-utils';

const ACCOUNT = { username: 'p1234567', accountHint: 'p*****67' };
const COUNTRIES = [{ code: 'US' }, { code: 'DE' }, { code: 'AL' }];
const SERVERS = {
  regions: [
    { id: 'us-east', name: 'US East' },
    { id: 'us-west', name: 'US West' },
    { id: 'al', name: 'Albania' },
  ],
  servers: [
    { hostname: 'useast1', ip: '198.51.100.10', regionId: 'us-east', regionName: 'US East' },
    { hostname: 'uswest1', ip: '198.51.100.30', regionId: 'us-west', regionName: 'US West' },
    {
      hostname: 'Server-12406-1a',
      ip: '198.51.100.40',
      regionId: 'al',
      regionName: 'Albania',
    },
  ],
};

function piaApiPost(url: string, data?: unknown) {
  if (url === '/panel/api/xray/pia/data') return new Msg(true, '', ACCOUNT);
  if (url === '/panel/api/xray/pia/countries') return new Msg(true, '', COUNTRIES);
  if (url === '/panel/api/xray/pia/servers') {
    const code = (data as { countryCode?: string } | undefined)?.countryCode?.toUpperCase();
    if (code === 'AL') {
      return new Msg(true, '', {
        regions: [SERVERS.regions[2]],
        servers: [SERVERS.servers[2]],
      });
    }
    if (code === 'US') {
      return new Msg(true, '', {
        regions: SERVERS.regions.slice(0, 2),
        servers: SERVERS.servers.slice(0, 2),
      });
    }
    return new Msg(true, '', { regions: [], servers: [] });
  }
  if (url === '/panel/api/xray/pia/addKey') {
    const hostname = (data as { hostname?: string } | undefined)?.hostname;
    if (hostname === 'uswest1') {
      return new Msg(true, '', {
        tag: 'pia-us-west-uswest1',
        hostname: 'uswest1',
        secretKey: 'secret-west',
        address: '10.8.0.2/32',
        publicKey: 'pubkey-west',
        endpoint: '198.51.100.30:1337',
      });
    }
    if (hostname === 'Server-12406-1a' || hostname === 'pia-al-server-12406-1a') {
      return new Msg(true, '', {
        tag: 'pia-al-server-12406-1a',
        hostname: 'Server-12406-1a',
        secretKey: 'secret-al',
        address: '10.8.0.3/32',
        publicKey: 'pubkey-al',
        endpoint: '198.51.100.40:1337',
      });
    }
    if (hostname === 'useast1' || hostname === 'pia-us-east-useast1') {
      return new Msg(true, '', {
        tag: 'pia-us-east-useast1',
        hostname: 'useast1',
        secretKey: 'secret',
        address: '10.8.0.1/32',
        publicKey: 'pubkey',
        endpoint: '198.51.100.10:1337',
      });
    }
    return new Msg(false, `Unexpected addKey hostname ${hostname}`, null);
  }
  return new Msg(false, `Unexpected POST ${url}`, null);
}

function mockPiaApi() {
  vi.mocked(HttpUtil.post).mockImplementation(async (url: string, data?: unknown) =>
    piaApiPost(url, data),
  );
}

function visibleOptions(): HTMLElement[] {
  return Array.from(
    document.querySelectorAll<HTMLElement>(
      '.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option',
    ),
  );
}

async function chooseOption(testId: string, labelPart: string) {
  const node = screen.getByTestId(testId);
  const select = node.closest('.ant-select') ?? node;
  const selector = select.querySelector('.ant-select-selector') ?? select;
  fireEvent.mouseDown(selector);
  await waitFor(() => expect(visibleOptions().length).toBeGreaterThan(0));
  const option = visibleOptions().find((item) =>
    (item.getAttribute('title') ?? item.textContent ?? '').includes(labelPart),
  );
  if (!option) throw new Error(`Missing option containing ${labelPart}`);
  fireEvent.click(option);
}

async function clickAddOutbound() {
  const addButton = await waitFor(() => {
    const btn = screen.getByRole('button', { name: /Add outbound/ });
    if ((btn as HTMLButtonElement).disabled) throw new Error('Add outbound still disabled');
    return btn;
  });
  fireEvent.click(addButton);
}

function expectPiaOutbound(
  outbound: Record<string, unknown>,
  want: {
    tag: string;
    hostname: string;
    secretKey: string;
    address: string;
    publicKey: string;
    endpoint: string;
  },
) {
  expect(outbound).toMatchObject({
    tag: want.tag,
    piaHostname: want.hostname,
    protocol: 'wireguard',
    settings: {
      secretKey: want.secretKey,
      address: [want.address],
      mtu: 1420,
      noKernelTun: true,
      peers: [
        {
          publicKey: want.publicKey,
          endpoint: want.endpoint,
          allowedIPs: ['0.0.0.0/0'],
          keepAlive: 25,
        },
      ],
    },
  });
}

function PiaHarness({ onAdded }: { onAdded?: (outbound: Record<string, unknown>) => void }) {
  const [outbounds, setOutbounds] = useState<Record<string, unknown>[]>([]);
  return (
    <PiaModal
      open
      templateSettings={{ outbounds }}
      onClose={vi.fn()}
      onAddOutbound={(outbound) => {
        onAdded?.(outbound);
        setOutbounds((prev) => [...prev, outbound]);
      }}
      onResetOutbound={vi.fn()}
    />
  );
}

describe('PIA modal', () => {
  it('shows username and password when not signed in', async () => {
    vi.mocked(HttpUtil.post).mockImplementation(async (url: string) => {
      if (url === '/panel/api/xray/pia/data') return new Msg(true, '', null);
      return new Msg(false, `Unexpected POST ${url}`, null);
    });

    renderWithProviders(
      <PiaModal
        open
        templateSettings={{ outbounds: [] }}
        onClose={vi.fn()}
        onAddOutbound={vi.fn()}
        onResetOutbound={vi.fn()}
      />,
    );

    await waitFor(() => expect(screen.getByPlaceholderText('PIA username')).toBeTruthy());
    expect(screen.getByPlaceholderText('PIA password')).toBeTruthy();
    expect(screen.getByRole('dialog', { name: 'Private Internet Access WireGuard' })).toBeTruthy();
    expect(screen.getByRole('button', { name: /Log In/ })).toBeTruthy();
    expect(screen.queryByTestId('pia-country-select')).toBeNull();
  });

  it('adds two WireGuard outbounds for different servers', async () => {
    mockPiaApi();
    const added: Record<string, unknown>[] = [];
    renderWithProviders(<PiaHarness onAdded={(outbound) => added.push(outbound)} />);

    await waitFor(() => expect(screen.getByText('p*****67')).toBeTruthy());
    await chooseOption('pia-country-select', 'US');
    await waitFor(() => expect(screen.getByTestId('pia-server-select')).toBeTruthy());
    await clickAddOutbound();

    await waitFor(() => expect(screen.getByTestId('pia-added-table')).toBeTruthy());
    expect(screen.getByText('pia-us-east-useast1')).toBeTruthy();
    expect(screen.getByRole('button', { name: /Add outbound/ })).toBeTruthy();

    await chooseOption('pia-server-select', 'uswest1');
    await clickAddOutbound();
    await waitFor(() => expect(screen.getByText('pia-us-west-uswest1')).toBeTruthy());
    expect(added).toHaveLength(2);
    expectPiaOutbound(added[0], {
      tag: 'pia-us-east-useast1',
      hostname: 'useast1',
      secretKey: 'secret',
      address: '10.8.0.1/32',
      publicKey: 'pubkey',
      endpoint: '198.51.100.10:1337',
    });
    expectPiaOutbound(added[1], {
      tag: 'pia-us-west-uswest1',
      hostname: 'uswest1',
      secretKey: 'secret-west',
      address: '10.8.0.2/32',
      publicKey: 'pubkey-west',
      endpoint: '198.51.100.30:1337',
    });
  });

  it('disables Add when the selected server is already in the list', async () => {
    mockPiaApi();
    renderWithProviders(
      <PiaModal
        open
        templateSettings={{ outbounds: [{ tag: 'pia-us-east-useast1', piaHostname: 'useast1' }] }}
        onClose={vi.fn()}
        onAddOutbound={vi.fn()}
        onResetOutbound={vi.fn()}
      />,
    );

    await waitFor(() => expect(screen.getByText('p*****67')).toBeTruthy());
    await chooseOption('pia-country-select', 'US');
    await waitFor(() => expect(screen.getByTestId('pia-server-select')).toBeTruthy());
    await waitFor(() => {
      const btn = screen.getByRole('button', { name: /Add outbound/ });
      expect((btn as HTMLButtonElement).disabled).toBe(true);
    });
    expect(screen.getByText(/Use Reset to renew its key/)).toBeTruthy();
  });

  it('resets an existing PIA outbound in place', async () => {
    const onResetOutbound = vi.fn();
    mockPiaApi();
    renderWithProviders(
      <PiaModal
        open
        templateSettings={{ outbounds: [{ tag: 'pia-us-east-useast1', piaHostname: 'useast1' }] }}
        onClose={vi.fn()}
        onAddOutbound={vi.fn()}
        onResetOutbound={onResetOutbound}
      />,
    );

    await waitFor(() => expect(screen.getByTestId('pia-reset-0')).toBeTruthy());
    fireEvent.click(screen.getByTestId('pia-reset-0'));
    await waitFor(() => expect(onResetOutbound).toHaveBeenCalledTimes(1));
    const payload = onResetOutbound.mock.calls[0][0] as {
      index: number;
      outbound: { tag: string; piaHostname: string; settings: { secretKey: string } };
      oldTag?: string;
      newTag: string;
    };
    expect(payload.index).toBe(0);
    expect(payload.oldTag).toBe('pia-us-east-useast1');
    expect(payload.newTag).toBe('pia-us-east-useast1');
    expectPiaOutbound(payload.outbound as Record<string, unknown>, {
      tag: 'pia-us-east-useast1',
      hostname: 'useast1',
      secretKey: 'secret',
      address: '10.8.0.1/32',
      publicKey: 'pubkey',
      endpoint: '198.51.100.10:1337',
    });
  });

  it('disables Add for a hyphenated cn when only the tag remains', async () => {
    mockPiaApi();
    renderWithProviders(
      <PiaModal
        open
        templateSettings={{ outbounds: [{ tag: 'pia-al-server-12406-1a' }] }}
        onClose={vi.fn()}
        onAddOutbound={vi.fn()}
        onResetOutbound={vi.fn()}
      />,
    );

    await waitFor(() => expect(screen.getByText('p*****67')).toBeTruthy());
    await chooseOption('pia-country-select', 'AL');
    await waitFor(() => expect(screen.getByTestId('pia-server-select')).toBeTruthy());
    await waitFor(() => {
      const btn = screen.getByRole('button', { name: /Add outbound/ });
      expect((btn as HTMLButtonElement).disabled).toBe(true);
    });
  });

  it('disables Add when only the outbound tag remains', async () => {
    mockPiaApi();
    renderWithProviders(
      <PiaModal
        open
        templateSettings={{ outbounds: [{ tag: 'pia-us-east-useast1' }] }}
        onClose={vi.fn()}
        onAddOutbound={vi.fn()}
        onResetOutbound={vi.fn()}
      />,
    );

    await waitFor(() => expect(screen.getByText('p*****67')).toBeTruthy());
    await chooseOption('pia-country-select', 'US');
    await waitFor(() => expect(screen.getByTestId('pia-server-select')).toBeTruthy());
    await waitFor(() => {
      const btn = screen.getByRole('button', { name: /Add outbound/ });
      expect((btn as HTMLButtonElement).disabled).toBe(true);
    });
  });

  it('resets from the outbound tag when piaHostname was stripped', async () => {
    const onResetOutbound = vi.fn();
    const posts: unknown[] = [];
    mockPiaApi();
    vi.mocked(HttpUtil.post).mockImplementation(async (url: string, data?: unknown) => {
      if (url === '/panel/api/xray/pia/addKey') posts.push(data);
      return piaApiPost(url, data);
    });
    renderWithProviders(
      <PiaModal
        open
        templateSettings={{ outbounds: [{ tag: 'pia-al-server-12406-1a' }] }}
        onClose={vi.fn()}
        onAddOutbound={vi.fn()}
        onResetOutbound={onResetOutbound}
      />,
    );

    const reset = await waitFor(() => screen.getByTestId('pia-reset-0'));
    expect((reset as HTMLButtonElement).disabled).toBe(false);
    fireEvent.click(reset);
    await waitFor(() => expect(onResetOutbound).toHaveBeenCalledTimes(1));
    expect(posts).toEqual([{ hostname: 'pia-al-server-12406-1a' }]);
    expectPiaOutbound(onResetOutbound.mock.calls[0][0].outbound as Record<string, unknown>, {
      tag: 'pia-al-server-12406-1a',
      hostname: 'Server-12406-1a',
      secretKey: 'secret-al',
      address: '10.8.0.3/32',
      publicKey: 'pubkey-al',
      endpoint: '198.51.100.40:1337',
    });
  });

  it('does not add an outbound when addKey omits WireGuard fields', async () => {
    const onAddOutbound = vi.fn();
    vi.mocked(HttpUtil.post).mockImplementation(async (url: string) => {
      if (url === '/panel/api/xray/pia/data') return new Msg(true, '', ACCOUNT);
      if (url === '/panel/api/xray/pia/countries') return new Msg(true, '', COUNTRIES);
      if (url === '/panel/api/xray/pia/servers') return new Msg(true, '', SERVERS);
      if (url === '/panel/api/xray/pia/addKey') {
        return new Msg(true, '', { tag: 'pia-us-east-useast1', hostname: 'useast1' });
      }
      return new Msg(false, `Unexpected POST ${url}`, null);
    });
    renderWithProviders(
      <PiaModal
        open
        templateSettings={{ outbounds: [] }}
        onClose={vi.fn()}
        onAddOutbound={onAddOutbound}
        onResetOutbound={vi.fn()}
      />,
    );

    await waitFor(() => expect(screen.getByText('p*****67')).toBeTruthy());
    await chooseOption('pia-country-select', 'US');
    await waitFor(() => expect(screen.getByTestId('pia-server-select')).toBeTruthy());
    await clickAddOutbound();
    await waitFor(() =>
      expect(screen.getByText('Could not build the PIA outbound. Try again.')).toBeTruthy(),
    );
    expect(onAddOutbound).not.toHaveBeenCalled();
  });
});
