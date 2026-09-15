import { describe, expect, it } from 'vitest';

import { outboundAddresses } from '@/pages/xray/outbounds/outbounds-tab-helpers';
import type { OutboundRow } from '@/pages/xray/outbounds/outbounds-tab-types';

// The core lowercases a protocol id before resolving the handler, so a row
// spelled "VMess" must still show the address its settings carry.
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

  it('reads a capitalised dns id', () => {
    expect(outboundAddresses(row('DNS', { rewriteAddress: '1.1.1.1', rewritePort: 53 }))).toEqual([
      '1.1.1.1:53',
    ]);
  });

  it('reads the flat server of a capitalised vless id', () => {
    expect(outboundAddresses(row('VLESS', { address: 'd.example.com', port: 443 }))).toEqual([
      'd.example.com:443',
    ]);
  });

  it('leaves a canonical id unchanged', () => {
    expect(outboundAddresses(row('vmess', vnext))).toEqual(['a.example.com:443']);
  });

  it('still returns nothing for a protocol that carries no address', () => {
    expect(outboundAddresses(row('freedom', {}))).toEqual([]);
  });

  it('reads the vnext server of a vless row', () => {
    expect(outboundAddresses(row('VLESS', vnext))).toEqual(['a.example.com:443']);
  });

  it('returns no bare separator for a vless row that carries no server', () => {
    expect(outboundAddresses(row('VLESS', {}))).toEqual([]);
  });

  it('reads the flat server of a hysteria id', () => {
    expect(outboundAddresses(row('hysteria', { address: 'e.example.com', port: 443 }))).toEqual([
      'e.example.com:443',
    ]);
  });

  it('reads the peer endpoint of an amneziawg id', () => {
    expect(
      outboundAddresses(row('amneziawg', { peers: [{ endpoint: 'f.example.com:51820' }] })),
    ).toEqual(['f.example.com:51820']);
  });
});
