import { describe, expect, it } from 'vitest';

import { withHostPort } from '@/lib/hosts/host-port';

describe('withHostPort', () => {
  it('replaces the port of every address, including ones that already carry another', () => {
    expect(
      withHostPort(['a.example.com:8443', 'b.example.com:2096', 'c.example.com'], 9443),
    ).toEqual(['a.example.com:9443', 'b.example.com:9443', 'c.example.com:9443']);
  });

  it('brackets bare IPv6 and keeps bracketed IPv6 and CIDR ranges intact', () => {
    expect(withHostPort(['2001:db8::1', '[2001:db8::2]:443', '10.0.0.0/24:443'], 9443)).toEqual([
      '[2001:db8::1]:9443',
      '[2001:db8::2]:9443',
      '10.0.0.0/24:9443',
    ]);
  });

  it('keeps a port-only entry port-only', () => {
    expect(withHostPort([':8443'], 9443)).toEqual([':9443']);
  });

  it('strips the ports when the new port is 0 (inherit the inbound port)', () => {
    expect(withHostPort(['a.example.com:8443', '[2001:db8::1]:443', ':8443'], 0)).toEqual([
      'a.example.com',
      '2001:db8::1',
      '',
    ]);
  });
});
