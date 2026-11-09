import { describe, expect, it } from 'vitest';
import { AWGPingListInputSchema, AWGPingPolicySchema } from '@/schemas/primitives/awg-ping';
import { ClientRecordSchema } from '@/schemas/client';
import { buildAmneziaWGClientConfig } from '@/pages/clients/amneziawgConfig';
import type { InboundOption } from '@/hooks/useClients';

describe('independent AWG ping permissions', () => {
  it('accepts IPv4 and CIDR lists but rejects arbitrary destinations and unbounded lists', () => {
    expect(AWGPingListInputSchema.parse('192.0.2.1, 198.51.100.0/24\n203.0.113.1')).toEqual([
      '192.0.2.1',
      '198.51.100.0/24',
      '203.0.113.1',
    ]);
    for (const bad of [
      '::/0',
      'example.test',
      '192.0.2.01',
      '192.0.2.1/33',
      '256.0.0.1',
      '192.0.2.1/01',
    ])
      expect(AWGPingListInputSchema.safeParse(bad).success).toBe(false);
    expect(
      AWGPingPolicySchema.safeParse({ enabled: true, allow: Array(257).fill('192.0.2.1') }).success,
    ).toBe(false);
  });
  it('keeps server-side ping rights out of the QR profile byte for byte', () => {
    const client = ClientRecordSchema.parse({
      email: 'synthetic',
      privateKey: 'synthetic-private',
      allowedIPs: '192.0.2.2/32',
      keepAlive: 25,
    });
    const inbound = {
      id: 1,
      protocol: 'amneziawg',
      port: 51820,
      awgServer: { publicKey: 'synthetic-public', primaryDns: '192.0.2.53' },
    } as InboundOption;
    const before = buildAmneziaWGClientConfig(client, inbound, 'vpn.example.test');
    const after = buildAmneziaWGClientConfig(
      { ...client, awgPing: { enabled: true, allow: ['198.51.100.0/24'], deny: ['198.51.100.8'] } },
      inbound,
      'vpn.example.test',
    );
    expect(after).toBe(before);
  });
});
