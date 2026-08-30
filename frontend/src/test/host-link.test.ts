/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import { hostToExternalProxyEntry, withMtprotoHostEndpoints } from '@/lib/hosts/host-link';
import { inboundFromDb } from '@/lib/xray/inbound-from-db';

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
    verifyPeerCertByName: 'verify.example.com',
    echConfigList: 'ECH',
    overrideSniFromAddress: false,
    keepSniBlank: false,
    vlessRoute: '',
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
    expect(ep.verifyPeerCertByName).toBe('verify.example.com');
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

  it('carries a single vlessRoute value through to the entry', () => {
    expect(hostToExternalProxyEntry({ ...base, vlessRoute: '443' }).vlessRoute).toBe('443');
    expect(hostToExternalProxyEntry({ ...base, vlessRoute: '' }).vlessRoute).toBeUndefined();
  });
});

describe('withMtprotoHostEndpoints', () => {
  const inbound = inboundFromDb({
    protocol: 'mtproto',
    port: 4060,
    listen: '127.0.0.1',
    settings: { clients: [] },
    streamSettings: {},
    sniffing: {},
  });

  it('projects enabled raw Hosts onto MTProto share endpoints', () => {
    const got = withMtprotoHostEndpoints(
      inbound,
      7,
      [
        {
          groupId: 'public',
          inboundIds: [7],
          hosts: ['proxy.example.com:443', '[2001:db8::1]'],
          port: 443,
          remark: 'public',
        },
      ],
      '',
      'panel.example.com',
    );
    expect(got.streamSettings?.externalProxy).toEqual([
      { forceTls: 'same', dest: 'proxy.example.com', port: 443, remark: 'public' },
      { forceTls: 'same', dest: '2001:db8::1', port: 4060, remark: 'public' },
    ]);
  });

  it('inherits the inbound address for a port-only Host', () => {
    const got = withMtprotoHostEndpoints(
      inbound,
      7,
      [{ groupId: 'port-only', inboundIds: [7], hosts: [':8443'], port: 8443 }],
      '',
      'panel.example.com',
    );
    expect(got.streamSettings?.externalProxy).toEqual([
      { forceTls: 'same', dest: 'panel.example.com', port: 8443, remark: '' },
    ]);
  });

  it('ignores disabled, excluded and unrelated Hosts', () => {
    const got = withMtprotoHostEndpoints(
      inbound,
      7,
      [
        { groupId: 'disabled', inboundIds: [7], hosts: ['a.example.com:443'], isDisabled: true },
        {
          groupId: 'excluded',
          inboundIds: [7],
          hosts: ['b.example.com:443'],
          excludeFromSubTypes: ['raw'],
        },
        { groupId: 'other', inboundIds: [8], hosts: ['c.example.com:443'] },
      ],
      '',
      'panel.example.com',
    );
    expect(got).toBe(inbound);
  });
});
