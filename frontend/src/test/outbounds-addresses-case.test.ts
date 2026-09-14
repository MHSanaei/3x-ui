import { describe, expect, it } from 'vitest';

import { outboundAddresses } from '@/pages/xray/outbounds/outbounds-tab-helpers';
import type { OutboundRow } from '@/pages/xray/outbounds/outbounds-tab-types';

// The core resolves "VMess" to vmess, so a row whose server settings are
// populated must still show its address; the switch reads the id verbatim.
const row = (protocol: string, settings: Record<string, unknown>): OutboundRow => ({
  key: 0,
  tag: 'p',
  protocol,
  settings,
});

const vnext = { vnext: [{ address: 'a.example.com', port: 443 }] };

describe('outboundAddresses', () => {
  it('reads a capitalised vmess id', () => {
    expect(outboundAddresses(row('VMess', vnext))).toEqual(['a.example.com:443']);
  });

  it('reads a capitalised trojan id', () => {
    expect(
      outboundAddresses(row('Trojan', { servers: [{ address: 'b.example.com', port: 8443 }] })),
    ).toEqual(['b.example.com:8443']);
  });

  it('reads a capitalised wireguard id', () => {
    expect(
      outboundAddresses(row('WireGuard', { peers: [{ endpoint: 'c.example.com:51820' }] })),
    ).toEqual(['c.example.com:51820']);
  });

  it('leaves a canonical id unchanged', () => {
    expect(outboundAddresses(row('vmess', vnext))).toEqual(['a.example.com:443']);
  });

  it('still returns nothing for a protocol that carries no address', () => {
    expect(outboundAddresses(row('freedom', {}))).toEqual([]);
  });
});
