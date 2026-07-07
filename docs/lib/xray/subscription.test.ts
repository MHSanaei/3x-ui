import { describe, it, expect } from 'vitest';
import {
  buildSubscriptionUrls,
  buildShareLinks,
  buildBase64Subscription,
  buildJsonSubscription,
  type SubClient,
  type SubUrlInput,
} from './subscription';
import { base64ToText } from './base64';
import { parseLink } from './links';

describe('buildSubscriptionUrls', () => {
  const base: SubUrlInput = {
    scheme: 'http',
    host: 'sub.example.com',
    port: 2096,
    subPath: '/sub/',
    jsonPath: '/json/',
    subId: 'ABC',
  };

  it('builds the sub and json URLs with the port', () => {
    const u = buildSubscriptionUrls(base);
    expect(u.base64).toBe('http://sub.example.com:2096/sub/ABC');
    expect(u.json).toBe('http://sub.example.com:2096/json/ABC');
  });

  it('omits the port behind a reverse proxy', () => {
    const u = buildSubscriptionUrls({
      ...base,
      scheme: 'https',
      host: 'example.com',
      subPath: '/sub-xxx/',
      behindProxy: true,
    });
    expect(u.base64).toBe('https://example.com/sub-xxx/ABC');
    expect(u.json).toBe('https://example.com/json/ABC');
  });

  it('normalizes a path missing its slashes', () => {
    const u = buildSubscriptionUrls({ ...base, subPath: 'sub' });
    expect(u.base64).toBe('http://sub.example.com:2096/sub/ABC');
  });

  it('returns empty strings for an empty subId', () => {
    const u = buildSubscriptionUrls({ ...base, subId: '' });
    expect(u.base64).toBe('');
    expect(u.json).toBe('');
  });
});

const vlessClient: SubClient = {
  protocol: 'vless',
  remark: 'HK-01',
  address: 'a.example.com',
  port: 443,
  id: '11111111-2222-3333-4444-555555555555',
  flow: 'xtls-rprx-vision',
  network: 'tcp',
  security: 'reality',
  sni: 'www.microsoft.com',
  publicKey: 'PBK',
  shortId: 'ab',
  fingerprint: 'chrome',
};

describe('buildShareLinks', () => {
  it('builds a vless link that round-trips through parseLink', () => {
    const [link] = buildShareLinks([vlessClient]);
    const parsed = parseLink(link);
    expect(parsed.protocol).toBe('vless');
    expect(parsed.address).toBe('a.example.com');
    expect(parsed.port).toBe(443);
    expect(parsed.credential).toBe('11111111-2222-3333-4444-555555555555');
    expect(parsed.params.security).toBe('reality');
    expect(parsed.name).toBe('HK-01');
  });
});

describe('buildBase64Subscription', () => {
  it('decodes back to the newline-joined links', () => {
    const trojan: SubClient = {
      protocol: 'trojan',
      remark: 'T',
      address: 'b.example.com',
      port: 443,
      password: 'pw',
      network: 'ws',
      security: 'tls',
      path: '/x',
      sni: 'b.example.com',
    };
    const body = buildBase64Subscription([vlessClient, trojan]);
    const decoded = base64ToText(body);
    expect(decoded.split('\n')).toEqual(buildShareLinks([vlessClient, trojan]));
  });

  it('returns empty for no clients', () => {
    expect(buildBase64Subscription([])).toBe('');
  });
});

describe('buildJsonSubscription', () => {
  it('emits a single object for one client with a FLAT proxy outbound (settings.id, no vnext)', () => {
    const cfg = JSON.parse(buildJsonSubscription([vlessClient]));
    expect(Array.isArray(cfg)).toBe(false);
    const proxy = cfg.outbounds[0];
    expect(proxy.protocol).toBe('vless');
    expect(proxy.tag).toBe('proxy');
    expect(proxy.settings.id).toBe(vlessClient.id);
    expect('vnext' in proxy.settings).toBe(false);
    expect(proxy.settings.level).toBe(8);
    expect(proxy.settings.flow).toBe('xtls-rprx-vision');
    // the default.json skeleton outbounds are preserved after the proxy
    expect(cfg.outbounds.some((o: { tag: string }) => o.tag === 'direct')).toBe(true);
    expect(cfg.outbounds.some((o: { tag: string }) => o.tag === 'block')).toBe(true);
    // sockopt is stripped from JSON-sub streamSettings
    expect(JSON.stringify(proxy.streamSettings)).not.toContain('sockopt');
    expect(cfg.remarks).toBe('HK-01');
  });

  it('trojan uses servers[] with a password and no method', () => {
    const trojan: SubClient = {
      protocol: 'trojan',
      remark: 'T',
      address: 'b',
      port: 443,
      password: 'pw',
      network: 'tcp',
      security: 'tls',
    };
    const cfg = JSON.parse(buildJsonSubscription([trojan]));
    const server = cfg.outbounds[0].settings.servers[0];
    expect(server.password).toBe('pw');
    expect('method' in server).toBe(false);
  });

  it('shadowsocks servers[] include a method', () => {
    const ss: SubClient = {
      protocol: 'ss',
      remark: 'S',
      address: 'c',
      port: 8388,
      password: 'pw',
      method: 'aes-256-gcm',
    };
    const cfg = JSON.parse(buildJsonSubscription([ss]));
    expect(cfg.outbounds[0].protocol).toBe('shadowsocks');
    expect(cfg.outbounds[0].settings.servers[0].method).toBe('aes-256-gcm');
  });

  it('emits an array for multiple clients', () => {
    const json = buildJsonSubscription([vlessClient, { ...vlessClient, remark: 'HK-02' }]);
    expect(Array.isArray(JSON.parse(json))).toBe(true);
  });

  it('returns empty for no clients', () => {
    expect(buildJsonSubscription([])).toBe('');
  });
});
