import { describe, expect, it } from 'vitest';

import { mergeDetected, whiteDnsLink } from '@/pages/dns-tunnel/links';

// WhiteDNS refuses a link whose scheme, schema, version or server fields differ from what
// its importer (WhiteDnsProfileLinks.kt) reads; this decodes the link the way it does.
function decodeLikeWhiteDns(link: string) {
  const scheme = link.slice(0, link.indexOf('://'));
  const payload = link.slice(link.indexOf('://') + 3);
  const padded = payload
    .replace(/-/g, '+')
    .replace(/_/g, '/')
    .padEnd(Math.ceil(payload.length / 4) * 4, '=');
  const bin = atob(padded);
  const bytes = Uint8Array.from(bin, (c) => c.charCodeAt(0));
  return { scheme, json: JSON.parse(new TextDecoder().decode(bytes)) };
}

describe('whiteDnsLink', () => {
  it('builds a stormdns:// profile WhiteDNS can import, non-ASCII names included', () => {
    const link = whiteDnsLink({
      name: 'ایران',
      engine: 'stormdns',
      domain: 'st.t.example.com',
      key: '93d3404c6701dc9dea2d1ff3282ab88b',
      method: 1,
    });
    expect(link.slice('stormdns://'.length)).not.toMatch(/[+/=]/);
    expect(decodeLikeWhiteDns(link)).toEqual({
      scheme: 'stormdns',
      json: {
        schema: 'whitedns.profile',
        version: 1,
        profile: {
          name: 'ایران',
          server: {
            domain: 'st.t.example.com',
            encryption_key: '93d3404c6701dc9dea2d1ff3282ab88b',
            encryption_method: 1,
          },
        },
      },
    });
  });
});

describe('mergeDetected', () => {
  it('refreshes the key of a server already listed, keeps its name, and adds new domains', () => {
    const listed = [
      {
        name: 'Iran',
        engine: 'stormdns' as const,
        domain: 'st.a.com',
        key: 'old-key-0123456789',
        method: 0,
      },
      {
        name: 'dnstt',
        engine: 'dnstt' as const,
        domain: 'b.com',
        key: 'dnstt-pubkey-0123456789',
        method: 0,
      },
    ];
    const detected = [
      {
        name: 'StormDNS',
        engine: 'stormdns' as const,
        domain: 'st.a.com',
        key: 'new-key-0123456789',
        method: 1,
      },
      {
        name: 'StormDNS 2',
        engine: 'stormdns' as const,
        domain: 'st.c.com',
        key: 'new-key-0123456789',
        method: 1,
      },
    ];
    expect(mergeDetected(listed, detected)).toEqual([
      {
        name: 'Iran',
        engine: 'stormdns',
        domain: 'st.a.com',
        key: 'new-key-0123456789',
        method: 1,
      },
      listed[1],
      detected[1],
    ]);
  });
});
