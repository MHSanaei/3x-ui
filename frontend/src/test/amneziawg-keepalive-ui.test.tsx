import { describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { buildAmneziaWGClientConfig } from '@/pages/clients/amneziawgConfig';
import { buildWireguardClientConfig } from '@/pages/clients/wireguardConfig';
import ClientFormModal from '@/pages/clients/ClientFormModal';
import { ClientRecordSchema } from '@/schemas/client';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';
import { renderWithProviders } from './test-utils';

const awgInbound = {
  id: 17,
  tag: 'awg-17',
  protocol: 'amneziawg',
  port: 51820,
  awgServer: { publicKey: 'serverPubKey==', jc: 4, jmin: 40, jmax: 100, s1: 30, s2: 90 },
} as unknown as InboundOption;

const baseClient = {
  email: 'peer-1',
  privateKey: 'clientPrivKey==',
  allowedIPs: '10.8.1.2/32',
  keepAlive: 25,
} as unknown as ClientRecord;

describe('AWG keepalive range UI contract', () => {
  it('accepts range-valued API records while preserving legacy numeric values', () => {
    expect(ClientRecordSchema.safeParse({ email: 'peer-1', keepAlive: '15-25' }).success).toBe(
      true,
    );
    expect(ClientRecordSchema.safeParse({ email: 'legacy', keepAlive: 70000 }).success).toBe(true);
  });

  it('writes the exact range to a native AmneziaWG profile', () => {
    const conf = buildAmneziaWGClientConfig(
      { ...baseClient, keepAlive: '15-25' } as unknown as ClientRecord,
      awgInbound,
      'awg.example.test',
    );
    expect(conf).toContain('PersistentKeepalive = 15-25');
  });

  it('never exports a range into a plain WireGuard profile', () => {
    const conf = buildWireguardClientConfig(
      { ...baseClient, keepAlive: '15-25' } as unknown as ClientRecord,
      { id: 18, tag: 'wg-18', protocol: 'wireguard', port: 51821 } as unknown as InboundOption,
      'wg.example.test',
    );
    expect(conf).not.toContain('PersistentKeepalive');
  });

  it('submits an AWG range from the client form', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const save = vi.fn().mockResolvedValue({ success: true });
    renderWithProviders(
      <QueryClientProvider client={queryClient}>
        <ClientFormModal
          open
          mode="edit"
          client={baseClient}
          inbounds={[awgInbound]}
          attachedIds={[17]}
          save={save}
          onOpenChange={() => {}}
        />
      </QueryClientProvider>,
    );
    const credentialsTab = Array.from(document.querySelectorAll('.ant-tabs-tab')).find(
      (tab) => tab.textContent?.trim() === 'Credentials',
    );
    if (!credentialsTab) throw new Error('Credentials tab not found');
    fireEvent.click(credentialsTab);

    const keepAlive = await screen.findByPlaceholderText('25 / 15-25');
    fireEvent.change(keepAlive, { target: { value: '15-25' } });
    fireEvent.click(screen.getByRole('button', { name: /save/i }));

    await waitFor(() => expect(save).toHaveBeenCalled());
    expect((save.mock.calls[0][0] as Record<string, unknown>).keepAlive).toBe('15-25');
  });

  it('rejects out-of-bounds ranges before saving', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const save = vi.fn().mockResolvedValue({ success: true });
    renderWithProviders(
      <QueryClientProvider client={queryClient}>
        <ClientFormModal
          open
          mode="edit"
          client={baseClient}
          inbounds={[awgInbound]}
          attachedIds={[17]}
          save={save}
          onOpenChange={() => {}}
        />
      </QueryClientProvider>,
    );
    const credentialsTab = Array.from(document.querySelectorAll('.ant-tabs-tab')).find(
      (tab) => tab.textContent?.trim() === 'Credentials',
    );
    if (!credentialsTab) throw new Error('Credentials tab not found');
    fireEvent.click(credentialsTab);

    const keepAlive = await screen.findByPlaceholderText('25 / 15-25');
    fireEvent.change(keepAlive, { target: { value: '15-65536' } });
    fireEvent.click(screen.getByRole('button', { name: /save/i }));

    await waitFor(() => expect(save).not.toHaveBeenCalled());
  });
});
