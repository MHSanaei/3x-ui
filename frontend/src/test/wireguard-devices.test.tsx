import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import ClientWireguardDevices from '@/pages/clients/ClientWireguardDevices';
import { buildWireguardDeviceConfigs } from '@/pages/clients/wireguardConfig';
import type { InboundOption } from '@/hooks/useClients';

vi.mock('react-i18next', () => ({ useTranslation: () => ({ t: (key: string) => key }) }));

function deviceState() {
  return {
    data: {
      limit: 2,
      inbounds: [
        {
          inboundId: 90,
          devices: [
            {
              id: 'primary',
              name: '',
              enable: true,
              privateKey: 'key',
              publicKey: 'pub',
              preSharedKey: '',
              allowedIPs: ['10.0.0.2/32'],
            },
            {
              id: 'phone',
              name: 'Phone',
              enable: false,
              privateKey: 'key2',
              publicKey: 'pub2',
              preSharedKey: '',
              allowedIPs: ['10.0.0.3/32'],
            },
          ],
        },
      ],
    },
    loading: false,
    error: null,
    pending: false,
    mutationError: null,
    create: vi.fn(),
    update: vi.fn(),
    remove: vi.fn(),
    setLimit: vi.fn(),
  };
}

function showDevices(state = deviceState()) {
  const configs = buildWireguardDeviceConfigs(
    { email: 'alexander' },
    [{ id: 90, protocol: 'wireguard', port: 51820, tag: 'WG' } as InboundOption],
    state.data,
    undefined,
    [],
    'localhost',
    '',
    true,
  );
  render(
    <ClientWireguardDevices state={state} subLink="" clientEmail="alexander" configs={configs} />,
  );
}

describe('WireGuard device rows', () => {
  it('allows reenabling an additional device from its row menu', async () => {
    const state = deviceState();
    showDevices(state);
    fireEvent.click(screen.getByRole('button', { name: 'Phone more' }));
    fireEvent.click(
      await screen.findByRole('menuitem', { name: 'pages.clients.wgDevices.enabled' }),
    );
    expect(state.update).toHaveBeenCalledWith('phone', { enable: true });
  });

  it('keeps the limit editor out of the card and disables connections for disabled devices', () => {
    showDevices();
    expect(screen.queryByRole('spinbutton')).toBeNull();
    expect(
      screen.getByRole('button', { name: /pages.clients.wgDevices.add/ }).hasAttribute('disabled'),
    ).toBe(true);
    expect(
      screen
        .getByRole('button', { name: 'pages.clients.wgDevices.primary copy' })
        .hasAttribute('disabled'),
    ).toBe(false);
    expect(screen.getByRole('button', { name: 'Phone copy' }).hasAttribute('disabled')).toBe(true);
  });

  it('creates a named device through the platform prompt dialog', async () => {
    const state = deviceState();
    state.data.limit = 3;
    showDevices(state);
    fireEvent.click(screen.getByRole('button', { name: /pages.clients.wgDevices.add/ }));
    fireEvent.change(await screen.findByRole('textbox', { name: 'pages.clients.wgDevices.name' }), {
      target: { value: 'Laptop' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'confirm' }));
    expect(state.create).toHaveBeenCalledWith('Laptop');
  });
});
