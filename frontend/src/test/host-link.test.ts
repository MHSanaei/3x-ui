/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import {
  hostEndpointsFor,
  hostToExternalProxyEntry,
  withHostEndpoints,
} from '@/lib/hosts/host-link';
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
    cipherSuites: 'TLS_AES_128_GCM_SHA256',
    overrideSniFromAddress: false,
    keepSniBlank: false,
    vlessRoute: '',
    allowInsecure: false,
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
    expect(ep.cipherSuites).toBe('TLS_AES_128_GCM_SHA256');
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

  it('carries allowInsecure through to the entry', () => {
    expect(hostToExternalProxyEntry({ ...base, allowInsecure: true }).allowInsecure).toBe(true);
    expect(
      hostToExternalProxyEntry({ ...base, allowInsecure: false }).allowInsecure,
    ).toBeUndefined();
  });
});

describe('withHostEndpoints', () => {
  const inbound = inboundFromDb({
    protocol: 'mtproto',
    port: 4060,
    listen: '127.0.0.1',
    settings: { clients: [] },
    streamSettings: {},
    sniffing: {},
  });

  it('projects enabled raw Hosts onto MTProto share endpoints', () => {
    const got = withHostEndpoints(
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
    const got = withHostEndpoints(
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
    const got = withHostEndpoints(
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

describe('hostEndpointsFor - multi SNI', () => {
  it('applies the same random SNI to all endpoints when sniSharedRandom is true', () => {
    const snis = ['sni1.example.com', 'sni2.example.com'];
    const makeGroup = () =>
      hostEndpointsFor(
        [
          {
            groupId: 'shared',
            inboundIds: [1],
            hosts: ['h1.example.com', 'h2.example.com', 'h3.example.com'],
            port: 443,
            snis,
            sniSharedRandom: true,
          },
        ],
        1,
        443,
        'default.com',
      );

    const eps = makeGroup();
    expect(eps).toHaveLength(3);
    const chosenSni = eps[0]!.sni;
    expect(snis).toContain(chosenSni);
    for (const ep of eps) {
      expect(ep.sni).toBe(chosenSni);
    }

    // Verify that across repeated calls, selection is randomized across the pool
    const picks = new Set(Array.from({ length: 30 }, () => makeGroup()[0]!.sni));
    expect(picks.size).toBeGreaterThan(1);
  });

  it('randomizes SNI per endpoint for a multi-SNI pool', () => {
    const snis = ['sni1.example.com', 'sni2.example.com'];
    const hosts = Array.from({ length: 30 }, (_, i) => `h${i}.example.com`);
    const eps = hostEndpointsFor(
      [
        {
          groupId: 'rand',
          inboundIds: [1],
          hosts,
          port: 443,
          snis,
        },
      ],
      1,
      443,
      'default.com',
    );
    expect(eps).toHaveLength(30);
    const seen = new Set<string>();
    for (const ep of eps) {
      expect(snis).toContain(ep.sni);
      if (ep.sni) seen.add(ep.sni);
    }
    expect(seen.size).toBeGreaterThan(1);
  });

  it('gives precedence to overrideSniFromAddress and keepSniBlank over snis', () => {
    const snis = ['sni1.example.com', 'sni2.example.com'];
    const eps = hostEndpointsFor(
      [
        {
          groupId: 'override',
          inboundIds: [1],
          hosts: ['dest.example.com'],
          port: 443,
          snis,
          overrideSniFromAddress: true,
        },
        {
          groupId: 'blank',
          inboundIds: [1],
          hosts: ['blank.example.com'],
          port: 443,
          snis,
          keepSniBlank: true,
        },
      ],
      1,
      443,
      'default.com',
    );
    expect(eps).toHaveLength(2);
    expect(eps[0]!.sni).toBe('dest.example.com');
    expect(eps[1]!.sni).toBeUndefined();
  });
});
