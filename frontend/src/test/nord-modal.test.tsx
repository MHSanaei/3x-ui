import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';

import NordModal from '@/pages/xray/overrides/NordModal';
import { HttpUtil, Msg } from '@/utils';
import { renderWithProviders } from './test-utils';

const NORD_DATA = { token: 'nord-token', private_key: 'current-private-key' };
const COUNTRIES = [{ id: 228, name: 'United States', code: 'US' }];
const SERVER_DATA = {
  locations: [
    { id: 10, country: { city: { id: 100, name: 'New York' } } },
    { id: 20, country: { city: { id: 200, name: 'Los Angeles' } } },
  ],
  servers: [
    {
      id: 1,
      name: 'United States #1',
      hostname: 'us1.nordvpn.com',
      station: '198.51.100.10',
      load: 12,
      location_ids: [10],
      technologies: [{ id: 35, metadata: [{ name: 'public_key', value: 'public-one' }] }],
    },
    {
      id: 2,
      name: 'United States #2',
      hostname: 'us2.nordvpn.com',
      station: '198.51.100.20',
      load: 24,
      location_ids: [20],
      technologies: [{ id: 35, metadata: [{ name: 'public_key', value: 'public-two' }] }],
    },
  ],
};

function nordApiPost(url: string) {
  if (url === '/panel/api/xray/nord/data') {
    return new Msg(true, '', JSON.stringify(NORD_DATA));
  }
  if (url === '/panel/api/xray/nord/countries') {
    return new Msg(true, '', JSON.stringify(COUNTRIES));
  }
  if (url === '/panel/api/xray/nord/servers') {
    return new Msg(true, '', JSON.stringify(SERVER_DATA));
  }
  if (url === '/panel/api/xray/nord/del') return new Msg(true, '', '');
  return new Msg(false, `Unexpected POST ${url}`, null);
}

function mockNordApi() {
  vi.mocked(HttpUtil.post).mockImplementation(async (url: string) => nordApiPost(url));
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
  fireEvent.mouseDown(select.querySelector('.ant-select-selector') ?? select);
  await waitFor(() => expect(visibleOptions().length).toBeGreaterThan(0));
  const option = visibleOptions().find((item) =>
    `${item.getAttribute('title') ?? ''} ${item.textContent ?? ''}`.includes(labelPart),
  );
  if (!option) throw new Error(`Missing option containing ${labelPart}`);
  fireEvent.click(option);
}

async function clickAddOutbound() {
  const button = await waitFor(() => {
    const candidate = screen.getByRole('button', { name: /Add outbound/ });
    if ((candidate as HTMLButtonElement).disabled) throw new Error('Add outbound still disabled');
    return candidate;
  });
  fireEvent.click(button);
}

function NordHarness({
  initial = [],
  onAdded,
}: {
  initial?: Record<string, unknown>[];
  onAdded?: (outbound: Record<string, unknown>) => void;
}) {
  const [outbounds, setOutbounds] = useState(initial);
  return (
    <>
      <output data-testid="outbound-state">{JSON.stringify(outbounds)}</output>
      <NordModal
        open
        templateSettings={{ outbounds }}
        onClose={vi.fn()}
        onAddOutbound={(outbound) => {
          onAdded?.(outbound);
          setOutbounds((previous) => [...previous, outbound]);
        }}
        onResetOutbound={({ index, outbound }) => {
          setOutbounds((previous) =>
            previous.map((existing, current) => (current === index ? outbound : existing)),
          );
        }}
      />
    </>
  );
}

describe('NordVPN modal', () => {
  it('shows access-token and private-key entry while signed out', async () => {
    vi.mocked(HttpUtil.post).mockImplementation(async (url: string) => {
      if (url === '/panel/api/xray/nord/data') return new Msg(true, '', '');
      return new Msg(false, `Unexpected POST ${url}`, null);
    });

    renderWithProviders(
      <NordModal
        open
        templateSettings={{ outbounds: [] }}
        onClose={vi.fn()}
        onAddOutbound={vi.fn()}
        onResetOutbound={vi.fn()}
      />,
    );

    await waitFor(() => expect(screen.getByPlaceholderText('Access token')).toBeTruthy());
    fireEvent.click(screen.getByRole('tab', { name: 'Private key' }));
    expect(await screen.findByPlaceholderText('Private key')).toBeTruthy();
  });

  it('adds multiple different NordLynx outbounds without closing the modal', async () => {
    mockNordApi();
    const added: Record<string, unknown>[] = [];
    renderWithProviders(<NordHarness onAdded={(outbound) => added.push(outbound)} />);

    await waitFor(() => expect(screen.getByText('nord-token')).toBeTruthy());
    await chooseOption('nord-country-select', 'United States');
    await waitFor(() => expect(screen.getByTestId('nord-server-select')).toBeTruthy());
    await clickAddOutbound();

    await waitFor(() => expect(screen.getByTestId('nord-added-table')).toBeTruthy());
    expect(screen.getByText('nord-us1.nordvpn.com')).toBeTruthy();
    expect(
      screen.getByTestId('nord-added-table').querySelector('.nord-added-server-endpoint')
        ?.textContent,
    ).toBe('198.51.100.10:51820');
    expect(screen.getByRole('dialog', { name: 'NordVPN NordLynx' })).toBeTruthy();

    await chooseOption('nord-server-select', 'United States #2');
    await clickAddOutbound();
    await waitFor(() => expect(screen.getByText('nord-us2.nordvpn.com')).toBeTruthy());

    expect(added).toHaveLength(2);
    expect(added[0]).toMatchObject({
      tag: 'nord-us1.nordvpn.com',
      protocol: 'wireguard',
      settings: {
        secretKey: 'current-private-key',
        address: ['10.5.0.2/32'],
        peers: [{ publicKey: 'public-one', endpoint: '198.51.100.10:51820' }],
        noKernelTun: true,
      },
    });
    expect(added[1]).toMatchObject({
      tag: 'nord-us2.nordvpn.com',
      settings: {
        peers: [{ publicKey: 'public-two', endpoint: '198.51.100.20:51820' }],
      },
    });
  });

  it('shows concise server details and load in the server picker', async () => {
    mockNordApi();
    renderWithProviders(<NordHarness />);

    await waitFor(() => expect(screen.getByText('nord-token')).toBeTruthy());
    await chooseOption('nord-country-select', 'United States');
    await waitFor(() => expect(screen.getByTestId('nord-server-select')).toBeTruthy());

    const node = screen.getByTestId('nord-server-select');
    const select = node.closest('.ant-select') ?? node;
    fireEvent.mouseDown(select.querySelector('.ant-select-selector') ?? select);

    await waitFor(() =>
      expect(
        document.querySelectorAll<HTMLElement>('.nord-server-popup .ant-select-item-option'),
      ).toHaveLength(2),
    );
    const options = Array.from(
      document.querySelectorAll<HTMLElement>('.nord-server-popup .ant-select-item-option'),
    );
    expect(options[0].querySelector('.nord-server-option-name')?.textContent).toBe(
      'United States #1',
    );
    expect(options[0].querySelector('.nord-server-option-hostname')?.textContent).toBe(
      'us1.nordvpn.com',
    );
    expect(options[0].querySelector('.nord-server-option-address')?.textContent).toBe(
      '198.51.100.10:51820',
    );
    expect(options[0].querySelector('.nord-server-load-value')?.textContent).toBe('12%');
    expect(options[1].querySelector('.nord-server-load-value')?.textContent).toBe('24%');
  });

  it('shows the country flag and selects All Cities after loading servers', async () => {
    mockNordApi();
    renderWithProviders(<NordHarness />);

    await waitFor(() => expect(screen.getByText('nord-token')).toBeTruthy());
    await chooseOption('nord-country-select', 'United States');

    const countrySelect = screen.getByTestId('nord-country-select').closest('.ant-select');
    expect(countrySelect?.textContent).toContain('🇺🇸 United States (US)');

    const citySelect = await waitFor(() => {
      const select = screen.getByTestId('nord-city-select').closest('.ant-select');
      if (!select?.textContent?.includes('All Cities')) {
        throw new Error('All Cities is not selected');
      }
      return select;
    });
    expect(citySelect.textContent).toContain('All Cities');

    const serverNode = screen.getByTestId('nord-server-select');
    const serverSelect = serverNode.closest('.ant-select') ?? serverNode;
    fireEvent.mouseDown(serverSelect.querySelector('.ant-select-selector') ?? serverSelect);
    await waitFor(() =>
      expect(
        document.querySelectorAll<HTMLElement>('.nord-server-popup .ant-select-item-option'),
      ).toHaveLength(2),
    );
  });

  it('disables Add when the selected server is already present', async () => {
    mockNordApi();
    renderWithProviders(
      <NordHarness initial={[{ tag: 'nord-us1.nordvpn.com', protocol: 'wireguard' }]} />,
    );

    await waitFor(() => expect(screen.getByText('nord-token')).toBeTruthy());
    await chooseOption('nord-country-select', 'United States');
    await waitFor(() => {
      const button = screen.getByRole('button', { name: /Add outbound/ });
      expect((button as HTMLButtonElement).disabled).toBe(true);
    });
    expect(screen.getByText(/already in the outbound list/i)).toBeTruthy();
  });

  it('refreshes only the selected existing outbound private key', async () => {
    mockNordApi();
    const onResetOutbound = vi.fn();
    const nordOutbound = {
      tag: 'nord-us9.nordvpn.com',
      protocol: 'wireguard',
      sendThrough: '192.0.2.8',
      settings: {
        secretKey: 'old-private-key',
        address: ['10.5.0.2/32'],
        noKernelTun: true,
        customOption: 'preserve-me',
        peers: [{ publicKey: 'old-public', endpoint: '198.51.100.90:51820' }],
      },
    };
    renderWithProviders(
      <NordModal
        open
        templateSettings={{
          outbounds: [{ tag: 'direct', protocol: 'freedom' }, nordOutbound],
        }}
        onClose={vi.fn()}
        onAddOutbound={vi.fn()}
        onResetOutbound={onResetOutbound}
      />,
    );

    const reset = await waitFor(() => screen.getByTestId('nord-reset-1'));
    fireEvent.click(reset);
    await waitFor(() => expect(onResetOutbound).toHaveBeenCalledTimes(1));
    expect(onResetOutbound.mock.calls[0][0]).toEqual({
      index: 1,
      outbound: {
        ...nordOutbound,
        settings: { ...nordOutbound.settings, secretKey: 'current-private-key' },
      },
      oldTag: 'nord-us9.nordvpn.com',
      newTag: 'nord-us9.nordvpn.com',
    });
  });

  it('shows malformed Nord rows but disables their Reset action', async () => {
    mockNordApi();
    renderWithProviders(
      <NordModal
        open
        templateSettings={{
          outbounds: [{ tag: 'nord-broken', protocol: 'wireguard', settings: {} }],
        }}
        onClose={vi.fn()}
        onAddOutbound={vi.fn()}
        onResetOutbound={vi.fn()}
      />,
    );

    const reset = await waitFor(() => screen.getByTestId('nord-reset-0'));
    expect((reset as HTMLButtonElement).disabled).toBe(true);
    expect(screen.getByText('nord-broken')).toBeTruthy();
  });

  it('clears credentials on logout without removing configured outbounds', async () => {
    mockNordApi();
    renderWithProviders(
      <NordHarness
        initial={[
          {
            tag: 'nord-us1.nordvpn.com',
            protocol: 'wireguard',
            settings: { secretKey: 'embedded-private-key' },
          },
        ]}
      />,
    );

    await waitFor(() => expect(screen.getByText('nord-token')).toBeTruthy());
    fireEvent.click(screen.getByRole('button', { name: 'Log Out' }));
    await waitFor(() => expect(screen.getByPlaceholderText('Access token')).toBeTruthy());
    expect(screen.getByTestId('outbound-state').textContent).toContain('nord-us1.nordvpn.com');
    expect(vi.mocked(HttpUtil.post)).toHaveBeenCalledWith('/panel/api/xray/nord/del');
  });

  it('does not add a server that omits its NordLynx public key', async () => {
    const onAddOutbound = vi.fn();
    vi.mocked(HttpUtil.post).mockImplementation(async (url: string) => {
      if (url === '/panel/api/xray/nord/data') {
        return new Msg(true, '', JSON.stringify(NORD_DATA));
      }
      if (url === '/panel/api/xray/nord/countries') {
        return new Msg(true, '', JSON.stringify(COUNTRIES));
      }
      if (url === '/panel/api/xray/nord/servers') {
        return new Msg(
          true,
          '',
          JSON.stringify({
            ...SERVER_DATA,
            servers: [{ ...SERVER_DATA.servers[0], technologies: [{ id: 35, metadata: [] }] }],
          }),
        );
      }
      return new Msg(false, `Unexpected POST ${url}`, null);
    });
    renderWithProviders(
      <NordModal
        open
        templateSettings={{ outbounds: [] }}
        onClose={vi.fn()}
        onAddOutbound={onAddOutbound}
        onResetOutbound={vi.fn()}
      />,
    );

    await waitFor(() => expect(screen.getByText('nord-token')).toBeTruthy());
    await chooseOption('nord-country-select', 'United States');
    await clickAddOutbound();
    await waitFor(() =>
      expect(
        screen.getByText('Selected server does not advertise a NordLynx public key.'),
      ).toBeTruthy(),
    );
    expect(onAddOutbound).not.toHaveBeenCalled();
  });
});
