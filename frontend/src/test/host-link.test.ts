/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import { hostToExternalProxyEntry } from '@/lib/hosts/host-link';

describe('hostToExternalProxyEntry', () => {
  const base = {
    security: 'tls' as const,
    address: 'cdn.example.com',
    port: 8443,
    remark: 'R',
    sni: 'sni.example.com',
    alpn: ['h2'] as ('h2' | 'h3' | 'http/1.1')[],
    fingerprint: 'chrome' as const,
    pinnedPeerCertSha256: ['AAAA'],
    echConfigList: 'ECH',
    overrideSniFromAddress: false,
    keepSniBlank: false,
  };

  it('maps the overlapping fields onto an external-proxy entry', () => {
    const ep = hostToExternalProxyEntry(base);
    expect(ep.forceTls).toBe('tls');
    expect(ep.dest).toBe('cdn.example.com');
    expect(ep.port).toBe(8443);
    expect(ep.remark).toBe('R');
    expect(ep.sni).toBe('sni.example.com');
    expect(ep.alpn).toEqual(['h2']);
    expect(ep.fingerprint).toBe('chrome');
    expect(ep.pinnedPeerCertSha256).toEqual(['AAAA']);
    expect(ep.echConfigList).toBe('ECH');
  });

  it('maps reality/same security to forceTls "same"', () => {
    expect(hostToExternalProxyEntry({ ...base, security: 'reality' }).forceTls).toBe('same');
    expect(hostToExternalProxyEntry({ ...base, security: 'same' }).forceTls).toBe('same');
    expect(hostToExternalProxyEntry({ ...base, security: 'none' }).forceTls).toBe('none');
  });

  it('uses the address as sni when overrideSniFromAddress is set', () => {
    const ep = hostToExternalProxyEntry({ ...base, overrideSniFromAddress: true });
    expect(ep.sni).toBe('cdn.example.com');
  });

  it('omits sni when keepSniBlank is set', () => {
    const ep = hostToExternalProxyEntry({ ...base, keepSniBlank: true });
    expect(ep.sni).toBeUndefined();
  });

  it('falls back to port 443 when the host port is 0 (inherit)', () => {
    const ep = hostToExternalProxyEntry({ ...base, port: 0 });
    expect(ep.port).toBe(443);
  });
});
