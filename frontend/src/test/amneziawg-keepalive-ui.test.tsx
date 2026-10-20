import { describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { buildAmneziaWGClientConfig, isAmneziaWGClient } from '@/pages/clients/amneziawgConfig';
import { genAmneziaWGConfig } from '@/lib/xray/inbound-link';
import { buildWireguardClientConfig, isWireguardClient } from '@/pages/clients/wireguardConfig';
import ClientFormModal from '@/pages/clients/ClientFormModal';
import { ClientRecordSchema } from '@/schemas/client';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';
import type { AmneziawgInboundSettings } from '@/schemas/protocols/inbound/amneziawg';
import {
  OptionalAmneziaWGKeepAliveInputSchema,
  OptionalNumericKeepAliveInputSchema,
  keepAliveInputValue,
} from '@/schemas/primitives/keepalive';
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
    expect(ClientRecordSchema.parse({ email: 'peer-1', keepAlive: '15-25' }).keepAlive).toBe(
      '15-25',
    );
    expect(ClientRecordSchema.parse({ email: 'legacy', keepAlive: 70000 }).keepAlive).toBe(70000);
    expect(ClientRecordSchema.safeParse({ email: 'bad', keepAlive: '15-65536' }).success).toBe(
      false,
    );
    expect(ClientRecordSchema.safeParse({ email: 'bad', keepAlive: '015-25' }).success).toBe(false);
    expect(ClientRecordSchema.safeParse({ email: 'bad', keepAlive: '25-15' }).success).toBe(false);
    expect(ClientRecordSchema.parse({ email: 'peer-1', keepAlive: '0-65535' }).keepAlive).toBe(
      '0-65535',
    );
    expect(OptionalNumericKeepAliveInputSchema.safeParse('15-25').success).toBe(false);
    expect(OptionalAmneziaWGKeepAliveInputSchema.parse(keepAliveInputValue('0'))).toBe(0);
    expect(OptionalAmneziaWGKeepAliveInputSchema.parse(undefined)).toBeUndefined();
  });

  it('writes the exact range to a native AmneziaWG profile', () => {
    const conf = buildAmneziaWGClientConfig(
      { ...baseClient, keepAlive: '15-25' } as unknown as ClientRecord,
      awgInbound,
      'awg.example.test',
    );
    expect(conf).toContain('PersistentKeepalive = 15-25');
  });

  it('writes the exact range to an AmneziaWG share profile', () => {
    const settings = {
      server: { publicKey: 'serverPubKey==', jc: 4, jmin: 40, jmax: 100, s1: 30, s2: 90 },
      clients: [
        {
          email: 'peer-1',
          privateKey: 'clientPrivKey==',
          allowedIPs: ['10.8.1.2/32'],
          keepAlive: '15-25',
        },
      ],
    } as unknown as AmneziawgInboundSettings;
    const conf = genAmneziaWGConfig({
      settings,
      address: 'awg.example.test',
      port: 51820,
      remark: 'peer-1',
      peerIndex: 0,
    });
    expect(conf).toContain('PersistentKeepalive = 15-25');
  });

  it('never exports a range into a plain WireGuard profile', () => {
    const conf = buildWireguardClientConfig(
      { ...baseClient, keepAlive: '15-25' } as unknown as ClientRecord,
      { id: 18, tag: 'wg-18', protocol: 'wireguard', port: 51821 } as unknown as InboundOption,
      'wg.example.test',
    );
    expect(conf).toContain('PrivateKey = clientPrivKey==');
    expect(conf).toContain('Endpoint = wg.example.test:51821');
    expect(conf).not.toContain('PersistentKeepalive');
    const rangeOnlyClient = { email: 'peer-1', keepAlive: '15-25' } as unknown as ClientRecord;
    expect(isWireguardClient(rangeOnlyClient)).toBe(false);
    expect(isAmneziaWGClient(rangeOnlyClient)).toBe(true);
  });

  it('keeps the shared WireGuard and AWG form numeric-only', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    renderWithProviders(
      <QueryClientProvider client={queryClient}>
        <ClientFormModal
          open
          mode="edit"
          client={baseClient}
          inbounds={[
            awgInbound,
            {
              id: 18,
              tag: 'wg-18',
              protocol: 'wireguard',
              port: 51821,
            } as unknown as InboundOption,
          ]}
          attachedIds={[17, 18]}
          save={vi.fn().mockResolvedValue({ success: true })}
          onOpenChange={() => {}}
        />
      </QueryClientProvider>,
    );
    const credentialsTab = Array.from(document.querySelectorAll('.ant-tabs-tab')).find(
      (tab) => tab.textContent?.trim() === 'Credentials',
    );
    if (!credentialsTab) throw new Error('Credentials tab not found');
    fireEvent.click(credentialsTab);

    const keepAliveLabel = Array.from(
      document.querySelectorAll<HTMLLabelElement>('.ant-form-item-label label'),
    ).find((label) => label.textContent?.trim() === 'Keepalive (seconds)');
    if (!keepAliveLabel?.htmlFor) throw new Error('Keepalive field label not found');
    const keepAlive = await screen.findByRole('spinbutton', { name: 'Keepalive (seconds)' });
    expect(document.getElementById(keepAliveLabel.htmlFor)).toBe(keepAlive);
    expect(screen.queryByPlaceholderText('25')).toBeNull();
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

    const keepAlive = await screen.findByPlaceholderText('25');
    fireEvent.change(keepAlive, { target: { value: '15-25' } });
    fireEvent.click(screen.getByRole('button', { name: /save/i }));

    await waitFor(() => expect(save).toHaveBeenCalled());
    expect((save.mock.calls[0][0] as Record<string, unknown>).keepAlive).toBe('15-25');
  });

  it('preserves an unchanged legacy numeric value outside the new range', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const save = vi.fn().mockResolvedValue({ success: true });
    renderWithProviders(
      <QueryClientProvider client={queryClient}>
        <ClientFormModal
          open
          mode="edit"
          client={{ ...baseClient, keepAlive: 70000 } as unknown as ClientRecord}
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
    await screen.findByPlaceholderText('25');
    fireEvent.click(screen.getByRole('button', { name: /save/i }));

    await waitFor(() => expect(save).toHaveBeenCalled());
    expect(Object.hasOwn(save.mock.calls[0][0] as object, 'keepAlive')).toBe(false);
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

    const keepAlive = await screen.findByPlaceholderText('25');
    fireEvent.change(keepAlive, { target: { value: '15-65536' } });
    fireEvent.click(screen.getByRole('button', { name: /save/i }));

    await waitFor(() =>
      expect(document.querySelector('.ant-form-item-explain-error')?.textContent).toContain(
        'Enter a whole number or range from 0 to 65535.',
      ),
    );
    expect(save).not.toHaveBeenCalled();
  });

  it('omits a cleared AWG value so editing another field preserves the stored value', async () => {
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
    const keepAlive = await screen.findByPlaceholderText('25');
    fireEvent.change(keepAlive, { target: { value: '' } });
    fireEvent.click(screen.getByRole('button', { name: /save/i }));

    await waitFor(() => expect(save).toHaveBeenCalled());
    expect(Object.hasOwn(save.mock.calls[0][0] as object, 'keepAlive')).toBe(false);
  });
});
