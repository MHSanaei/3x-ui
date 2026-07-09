import { describe, it, expect } from 'vitest';
import {
  buildOutbound,
  buildOutboundJson,
  buildStreamSettings,
  type OutboundInput,
  type StreamInput,
} from './outbounds';

describe('buildOutbound — freedom & blackhole', () => {
  it('freedom has empty settings by default and no streamSettings', () => {
    const ob = buildOutbound({ kind: 'freedom', tag: 'direct' });
    expect(ob.protocol).toBe('freedom');
    expect(ob.tag).toBe('direct');
    expect(ob.settings).toEqual({});
    expect('streamSettings' in ob).toBe(false);
  });

  it('freedom carries domainStrategy when set', () => {
    const ob = buildOutbound({ kind: 'freedom', tag: 'direct', domainStrategy: 'UseIP' });
    expect(ob.settings).toEqual({ domainStrategy: 'UseIP' });
  });

  it('blackhole has empty settings', () => {
    const ob = buildOutbound({ kind: 'blackhole', tag: 'block' });
    expect(ob.protocol).toBe('blackhole');
    expect(ob.settings).toEqual({});
  });
});

describe('buildOutbound — proxy protocols', () => {
  const base = { address: 'example.com', port: 443 };

  it('vless uses the FLAT settings form (address/port/id/flow/encryption), not vnext', () => {
    const ob = buildOutbound({
      kind: 'vless',
      tag: 'v',
      server: { ...base, id: 'uuid-1', flow: 'xtls-rprx-vision' },
    });
    expect(ob.protocol).toBe('vless');
    const s = ob.settings as Record<string, unknown>;
    expect(s.address).toBe('example.com');
    expect(s.port).toBe(443);
    expect(s.id).toBe('uuid-1');
    expect(s.flow).toBe('xtls-rprx-vision');
    expect(s.encryption).toBe('none');
    expect('vnext' in s).toBe(false);
  });

  it('vmess uses the vnext settings form', () => {
    const ob = buildOutbound({ kind: 'vmess', tag: 'm', server: { ...base, id: 'uuid-2' } });
    const vnext = (ob.settings as { vnext: Array<Record<string, unknown>> }).vnext;
    expect(vnext[0].address).toBe('example.com');
    expect(vnext[0].port).toBe(443);
    const users = vnext[0].users as Array<Record<string, unknown>>;
    expect(users[0].id).toBe('uuid-2');
    expect(users[0].security).toBe('auto');
  });

  it('trojan uses servers[] with a password', () => {
    const ob = buildOutbound({ kind: 'trojan', tag: 't', server: { ...base, password: 'pw' } });
    const servers = (ob.settings as { servers: Array<Record<string, unknown>> }).servers;
    expect(servers[0].address).toBe('example.com');
    expect(servers[0].password).toBe('pw');
  });

  it('shadowsocks servers[] include a method', () => {
    const ob = buildOutbound({
      kind: 'shadowsocks',
      tag: 's',
      server: { ...base, password: 'pw', method: 'aes-256-gcm' },
    });
    const servers = (ob.settings as { servers: Array<Record<string, unknown>> }).servers;
    expect(servers[0].method).toBe('aes-256-gcm');
    expect(servers[0].password).toBe('pw');
  });

  it('coerces a string port to a number', () => {
    const ob = buildOutbound({
      kind: 'vless',
      tag: 'v',
      server: { address: 'a', port: '8443' as unknown as number, id: 'x' },
    });
    expect((ob.settings as Record<string, unknown>).port).toBe(8443);
  });
});

describe('buildStreamSettings', () => {
  const baseStream: StreamInput = { network: 'ws', security: 'none' };

  it('ws emits wsSettings.path and omits an empty host', () => {
    const st = buildStreamSettings({ ...baseStream, path: '/ray' });
    expect((st.wsSettings as Record<string, unknown>).path).toBe('/ray');
    expect('host' in (st.wsSettings as Record<string, unknown>)).toBe(false);
    expect('tlsSettings' in st).toBe(false);
  });

  it('reality emits realitySettings and no tlsSettings', () => {
    const st = buildStreamSettings({
      network: 'tcp',
      security: 'reality',
      publicKey: 'PBK',
      shortId: 'ab',
      sni: 'www.microsoft.com',
    });
    const r = st.realitySettings as Record<string, unknown>;
    expect(r.publicKey).toBe('PBK');
    expect(r.shortId).toBe('ab');
    expect(r.serverName).toBe('www.microsoft.com');
    expect(r.fingerprint).toBe('chrome');
    expect('tlsSettings' in st).toBe(false);
  });

  it('tls emits tlsSettings.serverName from sni', () => {
    const st = buildStreamSettings({ network: 'ws', security: 'tls', sni: 'a.com', path: '/' });
    expect((st.tlsSettings as Record<string, unknown>).serverName).toBe('a.com');
  });

  it('security none emits no security sub-object', () => {
    const st = buildStreamSettings({ network: 'grpc', security: 'none', serviceName: 'svc' });
    expect('tlsSettings' in st).toBe(false);
    expect('realitySettings' in st).toBe(false);
    expect((st.grpcSettings as Record<string, unknown>).serviceName).toBe('svc');
  });

  it('kcp emits the version-correct defaults (no header/seed)', () => {
    const st = buildStreamSettings({ network: 'kcp', security: 'none' });
    const k = st.kcpSettings as Record<string, unknown>;
    expect(k.tti).toBe(20);
    expect(k.cwndMultiplier).toBe(1);
    expect(k.maxSendingWindow).toBe(2097152);
    expect('header' in k).toBe(false);
    expect('seed' in k).toBe(false);
  });
});

describe('buildOutbound — stream attachment', () => {
  it('attaches streamSettings for vless when a stream is provided', () => {
    const ob = buildOutbound({
      kind: 'vless',
      tag: 'v',
      server: { address: 'a', port: 443, id: 'x' },
      stream: { network: 'ws', security: 'tls', path: '/p', sni: 's' },
    });
    expect((ob.streamSettings as Record<string, unknown>).network).toBe('ws');
  });

  it('does not attach streamSettings to freedom even if a stream is provided', () => {
    const ob = buildOutbound({
      kind: 'freedom',
      tag: 'd',
      stream: { network: 'ws', security: 'none' },
    });
    expect('streamSettings' in ob).toBe(false);
  });
});

describe('buildOutbound — wireguard & warp', () => {
  it('wireguard peers carry publicKey, endpoint, and default allowedIPs', () => {
    const ob = buildOutbound({
      kind: 'wireguard',
      tag: 'wg',
      wireguard: { secretKey: 'sk', address: ['10.0.0.2/32'], publicKey: 'pk', endpoint: 'host:51820' },
    });
    const s = ob.settings as Record<string, unknown>;
    expect(s.secretKey).toBe('sk');
    expect(s.mtu).toBe(1420);
    const peers = s.peers as Array<Record<string, unknown>>;
    expect(peers[0].publicKey).toBe('pk');
    expect(peers[0].endpoint).toBe('host:51820');
    expect(peers[0].allowedIPs).toEqual(['0.0.0.0/0', '::/0']);
  });

  it('warp uses the wireguard protocol, forces tag=warp, and the Cloudflare endpoint', () => {
    const ob = buildOutbound({
      kind: 'warp',
      tag: 'ignored',
      wireguard: { secretKey: 'sk', address: [], publicKey: 'pk', endpoint: '' },
    });
    expect(ob.protocol).toBe('wireguard');
    expect(ob.tag).toBe('warp');
    const peers = (ob.settings as Record<string, unknown>).peers as Array<Record<string, unknown>>;
    expect(peers[0].endpoint).toBe('engage.cloudflareclient.com:2408');
  });
});

describe('buildOutboundJson', () => {
  it('round-trips and is 2-space indented', () => {
    const input: OutboundInput = {
      kind: 'vless',
      tag: 'v',
      server: { address: 'a', port: 443, id: 'x' },
    };
    const json = buildOutboundJson(input);
    expect(json).toContain('\n  "');
    expect(JSON.parse(json)).toEqual(buildOutbound(input));
  });
});
