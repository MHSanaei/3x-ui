import { describe, expect, it } from 'vitest';

import {
  buildHappRoutingDeeplink,
  loadHappRouting,
  parseHappRoutingJson,
  parseHappRoutingList,
} from '@/pages/settings/happRoutingEditor';

const profile = {
  Name: '大陆 · فارسی 🛣',
  GlobalProxy: 'false',
  RouteOrder: 'direct-proxy-block',
  DomainStrategy: 'AsIs',
  RemoteDNSDomain: 'https://cloudflare-dns.com/dns-query',
  DnsHosts: { 'dns.alidns.com': '223.5.5.5' },
  DirectSites: ['geosite:cn', 'regexp:^host[0-9]{1,3}\\.example$'],
  DirectIp: ['geoip:cn'],
  BlockSites: ['geosite:category-ads-all'],
  FutureSetting: { enabled: true, values: ['保留', 7, null] },
};

describe('Happ routing editor profile loading', () => {
  it.each(['onadd', 'add'] as const)('loads %s with UTF-8 and preserves every field', (mode) => {
    const encoded = Buffer.from(JSON.stringify(profile), 'utf8').toString('base64');
    expect(loadHappRouting(`  happ://routing/${mode}/${encoded}\n`)).toEqual({
      success: true,
      profile,
      mode,
      isNew: false,
    });
  });

  it.each(['base64', 'base64url'] as const)(
    'accepts padded and unpadded %s payloads',
    (encoding) => {
      const encoded = Buffer.from(JSON.stringify(profile), 'utf8').toString(encoding);
      const unpadded = encoded.replace(/=+$/, '');
      const padded = unpadded.padEnd(Math.ceil(unpadded.length / 4) * 4, '=');
      for (const payload of [padded, unpadded]) {
        const loaded = loadHappRouting(`happ://routing/onadd/${payload}`);
        expect(loaded).toMatchObject({ success: true, profile });
      }
    },
  );

  it('loads raw JSON without filling absent fields or discarding extensions', () => {
    const input = '{"Name":"existing","GlobalProxy":"false","Future":{"nested":[1,null]}}';
    expect(loadHappRouting(input)).toEqual({
      success: true,
      profile: { Name: 'existing', GlobalProxy: 'false', Future: { nested: [1, null] } },
      mode: 'onadd',
      isNew: false,
    });
  });

  it('retains the existing new-profile flow only for an empty source', () => {
    expect(loadHappRouting(' \n ')).toEqual({
      success: true,
      profile: {
        Name: 'Custom Rules',
        GlobalProxy: 'true',
        DirectSites: [],
        DirectIp: [],
        ProxySites: [],
        ProxyIp: [],
        BlockSites: [],
        BlockIp: [],
        DomainStrategy: 'IPIfNonMatch',
      },
      mode: 'onadd',
      isNew: true,
    });
  });

  it.each([
    ['happ://routing/off', 'off'],
    ['https://example.com/rules', 'remote'],
    ['http://example.com/rules', 'remote'],
    ['happ://routing/onadd/', 'invalid'],
    ['happ://routing/onadd/%invalid', 'invalid'],
    ['happ://routing/onadd/e3\n0=', 'invalid'],
    ['happ://routing/add//w==', 'invalid'],
    ['happ://routing/onadd/W10=', 'invalid'],
    ['happ://unknown/e30=', 'invalid'],
    ['{"DirectSites":"geosite:cn"}', 'invalid'],
    ['{"DirectIp":[3]}', 'invalid'],
    ['{"Name":', 'invalid'],
  ])('does not replace an unloadable source %s with a new profile', (source, error) => {
    expect(loadHappRouting(source)).toEqual({ success: false, error });
  });
});

describe('Happ routing editor JSON and output', () => {
  it('preserves all fields through a valid JSON edit', () => {
    expect(parseHappRoutingJson(JSON.stringify(profile))).toEqual(profile);
  });

  it('preserves extension keys as JSON data without changing object prototypes', () => {
    const source = '{"Name":"extensions","__proto__":{"clientOption":true},"DirectSites":[]}';
    const parsed = parseHappRoutingJson(source);
    expect(parsed).not.toBeNull();
    if (!parsed) throw new Error('Expected a valid profile');

    const rebuilt = JSON.parse(atob(buildHappRoutingDeeplink(parsed).split('/onadd/')[1]));
    expect(rebuilt).toEqual(JSON.parse(source));
    expect(Object.getPrototypeOf(parsed)).toBe(Object.prototype);
  });

  it.each(['', '{', '[]', 'null', 'true', '42', '{"BlockIp":null}', '{"ProxySites":[{}]}'])(
    'rejects an invalid profile %s',
    (source) => {
      expect(parseHappRoutingJson(source)).toBeNull();
    },
  );

  it('generates a standard UTF-8 link while retaining add semantics and unknown fields', () => {
    const encoded = Buffer.from(JSON.stringify(profile), 'utf8').toString('base64');
    expect(buildHappRoutingDeeplink(profile, 'add')).toBe(`happ://routing/add/${encoded}`);
    expect(buildHappRoutingDeeplink(profile)).toBe(`happ://routing/onadd/${encoded}`);
  });

  it('keeps regexp commas intact when editing one rule per line', () => {
    expect(
      parseHappRoutingList('  geosite:cn\r\nregexp:^host[0-9]{1,3}\\.example$\n\n domain:local '),
    ).toEqual(['geosite:cn', 'regexp:^host[0-9]{1,3}\\.example$', 'domain:local']);
  });
});
