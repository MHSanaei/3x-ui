import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import ClientWireguardDevices from '@/pages/clients/ClientWireguardDevices';

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
              name: 'Primary',
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

describe('WireGuard devices', () => {
  it('shows addresses and allows reenabling an additional device', () => {
    const state = deviceState();
    render(<ClientWireguardDevices state={state} subLink="" onConfig={vi.fn()} />);
    expect(screen.getByText('10.0.0.2/32')).toBeTruthy();
    expect(screen.getByText('10.0.0.3/32')).toBeTruthy();
    expect(
      screen
        .getByRole('switch', { name: 'Primary pages.clients.wgDevices.enabled' })
        .hasAttribute('disabled'),
    ).toBe(true);
    fireEvent.click(screen.getByRole('switch', { name: 'Phone pages.clients.wgDevices.enabled' }));
    expect(state.update).toHaveBeenCalledWith('phone', { enable: true });
  });

  it('blocks adding devices at the limit and hides configuration for disabled devices', () => {
    render(<ClientWireguardDevices state={deviceState()} subLink="" onConfig={vi.fn()} />);
    expect(
      screen.getByRole('button', { name: /pages.clients.wgDevices.add/ }).hasAttribute('disabled'),
    ).toBe(true);
    const buttons = screen.getAllByRole('button', { name: 'pages.clients.wireguardConfig' });
    expect(buttons[0].hasAttribute('disabled')).toBe(false);
    expect(buttons[1].hasAttribute('disabled')).toBe(true);
  });

  it('creates a named device when there is room', () => {
    const state = deviceState();
    state.data.limit = 3;
    render(<ClientWireguardDevices state={state} subLink="" onConfig={vi.fn()} />);
    fireEvent.change(screen.getByRole('textbox', { name: 'pages.clients.wgDevices.name' }), {
      target: { value: 'Laptop' },
    });
    fireEvent.click(screen.getByRole('button', { name: /pages.clients.wgDevices.add/ }));
    expect(state.create).toHaveBeenCalledWith('Laptop');
  });
});
