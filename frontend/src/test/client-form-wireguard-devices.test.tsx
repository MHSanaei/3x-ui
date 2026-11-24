import { describe, it, expect, vi } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';

import ClientFormModal from '@/pages/clients/ClientFormModal';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';
import { renderWithProviders } from './test-utils';

const WIREGUARD_INBOUND = {
  id: 7,
  port: 51820,
  protocol: 'wireguard',
  tag: 'wireguard-test',
  enable: true,
} as InboundOption;

const CLIENT = {
  email: 'wireguard-user',
  uuid: '11111111-1111-1111-1111-111111111111',
  subId: 'wireguard-sub',
  enable: true,
} as ClientRecord;

function renderForm(client: ClientRecord | null, inbound = WIREGUARD_INBOUND) {
  const save = vi.fn().mockResolvedValue({ success: true });
  renderWithProviders(
    <ClientFormModal
      open
      mode={client ? 'edit' : 'add'}
      client={client}
      inbounds={[inbound]}
      attachedIds={client ? [inbound.id] : []}
      save={save}
      onOpenChange={() => {}}
    />,
  );
  return save;
}

describe('ClientFormModal WireGuard device limit', () => {
  it('saves the chosen limit through the normal client update', async () => {
    const save = renderForm({ ...CLIENT, wgDeviceLimit: 8 });
    const input = await screen.findByRole('spinbutton', { name: 'WireGuard · Device limit' });
    expect((input as HTMLInputElement).value).toBe('8');
    fireEvent.change(input, { target: { value: '12' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(save).toHaveBeenCalled());
    expect(save.mock.calls[0][0]).toHaveProperty('wgDeviceLimit', 12);
  });

  it('defaults legacy clients to five devices', async () => {
    const save = renderForm(CLIENT);
    const input = await screen.findByRole('spinbutton', { name: 'WireGuard · Device limit' });
    expect((input as HTMLInputElement).value).toBe('5');
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(save).toHaveBeenCalled());
    expect(save.mock.calls[0][0]).toHaveProperty('wgDeviceLimit', 5);
  });

  it('includes the limit when creating a WireGuard client', async () => {
    const save = renderForm(null);
    fireEvent.click(screen.getByRole('button', { name: 'Select all' }));
    const input = await screen.findByRole('spinbutton', { name: 'WireGuard · Device limit' });
    expect((input as HTMLInputElement).value).toBe('5');
    fireEvent.change(input, { target: { value: '4' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create' }));
    await waitFor(() => expect(save).toHaveBeenCalled());
    expect(save.mock.calls[0][0]).toHaveProperty('client.wgDeviceLimit', 4);
  });

  it('keeps the limit above the existing device count', async () => {
    renderForm({ ...CLIENT, wgDeviceLimit: 6, wireguardDevices: [{}, {}] });
    const input = await screen.findByRole('spinbutton', { name: 'WireGuard · Device limit' });
    expect(input.getAttribute('aria-valuemin')).toBe('3');
    expect(input.getAttribute('aria-valuemax')).toBe('32');
  });

  it('requires a limit before saving', async () => {
    const save = renderForm({ ...CLIENT, wgDeviceLimit: 6, wireguardDevices: [{}, {}] });
    const input = await screen.findByRole('spinbutton', { name: 'WireGuard · Device limit' });
    fireEvent.change(input, { target: { value: '' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await screen.findByText('Device limit (3–32)');
    expect(save).not.toHaveBeenCalled();
  });

  it('hides the setting for clients attached only to other protocols', async () => {
    const save = renderForm(CLIENT, { ...WIREGUARD_INBOUND, protocol: 'vless' });
    expect(screen.queryByRole('spinbutton', { name: 'WireGuard · Device limit' })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(save).toHaveBeenCalled());
    expect(save.mock.calls[0][0]).not.toHaveProperty('wgDeviceLimit');
  });
});
