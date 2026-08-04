import { describe, expect, it } from 'vitest';

import { buildAmneziaWGClientConfig } from '@/pages/clients/amneziawgConfig';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';

const client: ClientRecord = {
  email: 'alice',
  privateKey: 'QGVlb2dXc1ZTWGw0ZXBzZndsWmtMaUM5MUlNYjBHWFdYbz0=',
  allowedIPs: '10.8.1.2/32',
  inboundIds: [91],
};

const inbound: InboundOption = {
  id: 91,
  tag: 'in-51821-udp',
  remark: 'awg-mc',
  protocol: 'amneziawg',
  port: 51821,
  awgServer: {
    publicKey: 'DGSYIcEKAUkA7HhzGSjxLZuV67BR3LeyU0BMLJzNVHQ=',
    mtu: 1420,
    jc: 5,
    jmin: 10,
    jmax: 50,
    s1: 30,
    s2: 45,
  },
};

describe('buildAmneziaWGClientConfig — AmneziaWG 3.0 fields', () => {
  it('omits HeaderProtectionKey/ContentPaddingAddition when the server has neither set', () => {
    const cfg = buildAmneziaWGClientConfig(client, inbound, 'example.com', '');
    expect(cfg).not.toContain('HeaderProtectionKey');
    expect(cfg).not.toContain('ContentPaddingAddition');
  });

  it('emits HeaderProtectionKey when the server has it set', () => {
    const withHp: InboundOption = {
      ...inbound,
      awgServer: { ...inbound.awgServer, headerProtectionKey: 'some-header-protection-key==' },
    };
    const cfg = buildAmneziaWGClientConfig(client, withHp, 'example.com', '');
    expect(cfg).toContain('HeaderProtectionKey = some-header-protection-key==');
  });

  it('emits ContentPaddingAddition when the server has it set', () => {
    const withCpa: InboundOption = {
      ...inbound,
      awgServer: { ...inbound.awgServer, contentPaddingAddition: '20-40' },
    };
    const cfg = buildAmneziaWGClientConfig(client, withCpa, 'example.com', '');
    expect(cfg).toContain('ContentPaddingAddition = 20-40');
  });
});
