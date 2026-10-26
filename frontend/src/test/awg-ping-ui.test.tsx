import { describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import ClientFormModal from '@/pages/clients/ClientFormModal';
import { ClientRecordSchema } from '@/schemas/client';
import type { InboundOption } from '@/hooks/useClients';
import { renderWithProviders } from './test-utils';
import { server } from './msw/server';

function modal() {
  server.use(
    http.get('*/panel/api/inbounds/amneziawgPingReadiness', () =>
      HttpResponse.json({ success: true, obj: { available: true } }),
    ),
  );
  const save = vi.fn().mockResolvedValue({ success: true });
  const close = vi.fn();
  renderWithProviders(
    <ClientFormModal
      open
      mode="edit"
      client={ClientRecordSchema.parse({
        email: 'synthetic',
        subId: 'synthetic-sub',
        enable: true,
        allowedIPs: '192.0.2.2/32',
        keepAlive: 25,
      })}
      inbounds={[{ id: 17, protocol: 'amneziawg', port: 51820 } as InboundOption]}
      attachedIds={[17]}
      save={save}
      onOpenChange={close}
    />,
  );
  const credentials = Array.from(document.querySelectorAll('.ant-tabs-tab')).find(
    (tab) => tab.textContent?.trim() === 'Credentials',
  );
  if (!credentials) throw new Error('Credentials tab missing');
  fireEvent.click(credentials);
  return { save, close };
}

describe('AWG ping editor', () => {
  it('defaults to deny and saves explicit independent allow and deny lists', async () => {
    const { save } = modal();
    const enabled = await screen.findByRole('switch', { name: 'Allow ping' });
    expect(enabled.getAttribute('aria-checked')).toBe('false');
    expect(document.body.textContent).toContain('Separate from TCP/UDP routing');
    fireEvent.click(enabled);
    fireEvent.change(screen.getByLabelText('Allowed IPv4 addresses / CIDRs'), {
      target: { value: '198.51.100.0/24' },
    });
    fireEvent.change(screen.getByLabelText('Denied IPv4 addresses / CIDRs'), {
      target: { value: '198.51.100.8' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^save$/i }));
    await waitFor(() => expect(save).toHaveBeenCalled());
    expect(save.mock.calls[0][0].awgPing).toEqual({
      enabled: true,
      allow: ['198.51.100.0/24'],
      deny: ['198.51.100.8'],
    });
  });
  it('rejects malformed ranges before calling save', async () => {
    const { save } = modal();
    fireEvent.change(await screen.findByLabelText('Allowed IPv4 addresses / CIDRs'), {
      target: { value: '::/0' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^save$/i }));
    await screen.findByText('Use at most 256 IPv4 addresses or CIDRs per list.');
    expect(save).not.toHaveBeenCalled();
  });
  it('cancels without a save', async () => {
    const { save, close } = modal();
    fireEvent.click(await screen.findByRole('switch', { name: 'Allow ping' }));
    fireEvent.click(screen.getByRole('button', { name: /^cancel$/i }));
    expect(close).toHaveBeenCalledWith(false);
    expect(save).not.toHaveBeenCalled();
  });
});
