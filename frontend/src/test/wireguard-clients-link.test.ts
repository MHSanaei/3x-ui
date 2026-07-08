import { describe, expect, it } from 'vitest';

import { genWireguardConfigs, genWireguardLinks } from '@/lib/xray/inbound-link';
import { InboundSchema } from '@/schemas/api/inbound';

// Multi-client WireGuard renders one link/config per entry in settings.clients
// (the canonical store), not settings.peers. Each client carries its own
// privateKey + allowedIPs; the server public key is derived from secretKey.
function wgInbound() {
  return InboundSchema.parse({
    id: 90,
    remark: 'wg-mc',
    port: 51820,
    protocol: 'wireguard',
    settings: {
      mtu: 1420,
      secretKey: 'iJ2cBkrSGqRwIfYIDIxk7hr5RXfdR93MfJUL7yqkkH8=',
      peers: [],
      clients: [
        {
          email: 'alice',
          privateKey: 'QGVlb2dXc1ZTWGw0ZXBzZndsWmtMaUM5MUlNYjBHWFdYbz0=',
          publicKey: 'DGSYIcEKAUkA7HhzGSjxLZuV67BR3LeyU0BMLJzNVHQ=',
          allowedIPs: ['10.0.0.2/32'],
          keepAlive: 25,
        },
        {
          email: 'bob',
          privateKey: 'aGVsbG8td29ybGQtdGVzdC1wcml2YXRlLWtleS1ub3chIQ==',
          publicKey: 'b3RoZXItcHVibGljLWtleS1mb3ItYm9iLXRlc3QtdmFsISE=',
          allowedIPs: ['10.0.0.3/32'],
        },
      ],
    },
  });
}

describe('wireguard multi-client link/config fan-out', () => {
  it('emits one link per client from settings.clients', () => {
    const out = genWireguardLinks({
      inbound: wgInbound(),
      remark: 'wg-mc',
      fallbackHostname: 'wg.example.test',
    });
    const links = out.split('\r\n').filter(Boolean);
    expect(links).toHaveLength(2);
    expect(links[0]).toContain('wireguard://');
    expect(links[0]).toContain('address=10.0.0.2%2F32');
    expect(links[1]).toContain('address=10.0.0.3%2F32');
  });

  it('emits one .conf per client with its own address', () => {
    const out = genWireguardConfigs({
      inbound: wgInbound(),
      remark: 'wg-mc',
      fallbackHostname: 'wg.example.test',
    });
    const configs = out.split('\r\n[Interface]').length;
    expect(out).toContain('Address = 10.0.0.2/32');
    expect(out).toContain('Address = 10.0.0.3/32');
    expect(configs).toBe(2);
  });

  it('falls back to settings.peers for legacy single-config inbounds', () => {
    const legacy = InboundSchema.parse({
      id: 91,
      remark: 'wg-legacy',
      port: 51820,
      protocol: 'wireguard',
      settings: {
        secretKey: 'iJ2cBkrSGqRwIfYIDIxk7hr5RXfdR93MfJUL7yqkkH8=',
        peers: [
          {
            privateKey: 'QGVlb2dXc1ZTWGw0ZXBzZndsWmtMaUM5MUlNYjBHWFdYbz0=',
            publicKey: 'DGSYIcEKAUkA7HhzGSjxLZuV67BR3LeyU0BMLJzNVHQ=',
            allowedIPs: ['10.0.0.9/32'],
          },
        ],
      },
    });
    const out = genWireguardLinks({ inbound: legacy, remark: 'wg-legacy', fallbackHostname: 'wg.example.test' });
    const links = out.split('\r\n').filter(Boolean);
    expect(links).toHaveLength(1);
    expect(links[0]).toContain('address=10.0.0.9%2F32');
  });
});
