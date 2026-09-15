import { describe, expect, it } from 'vitest';

import { buildHappPresetDeeplink, parseList, toBase64Utf8 } from '@/pages/settings/happPresets';

describe('Happ presets and helpers', () => {
  it('correctly parses comma and newline separated lists', () => {
    const raw = 'domain:ir\nregexp:.*\\.ir$\n, example.com, , google.com';
    const parsed = parseList(raw);
    expect(parsed).toEqual(['domain:ir', 'regexp:.*\\.ir$', 'example.com', 'google.com']);
  });

  it('generates valid happ://routing/off for off preset', () => {
    const link = buildHappPresetDeeplink('off');
    expect(link).toBe('happ://routing/off');
  });

  it('generates valid base64 payload for iran-bypass preset', () => {
    const link = buildHappPresetDeeplink('iran-bypass');
    expect(link.startsWith('happ://routing/onadd/')).toBe(true);

    const b64 = link.replace('happ://routing/onadd/', '');
    const jsonStr = atob(b64);
    const parsed = JSON.parse(jsonStr);

    expect(parsed.Name).toBe('Iran Bypass');
    expect(parsed.GlobalProxy).toBe('true');
    expect(parsed.DirectSites).toContain('domain:ir');
    expect(parsed.DirectIp).toContain('geoip:ir');
    expect(parsed.BlockSites).toEqual([]);
  });

  it('generates valid base64 payload for china-direct preset', () => {
    const link = buildHappPresetDeeplink('china-direct');
    expect(link.startsWith('happ://routing/onadd/')).toBe(true);

    const b64 = link.replace('happ://routing/onadd/', '');
    const jsonStr = atob(b64);
    const parsed = JSON.parse(jsonStr);

    expect(parsed).toEqual({
      Name: 'Bypass-CN',
      GlobalProxy: 'true',
      RouteOrder: 'block-proxy-direct',
      RemoteDNSType: 'DoH',
      RemoteDNSDomain: 'https://cloudflare-dns.com/dns-query',
      RemoteDNSIP: '1.1.1.1',
      DomesticDNSType: 'DoH',
      DomesticDNSDomain: 'https://dns.alidns.com/dns-query',
      DomesticDNSIP: '223.5.5.5',
      Geoipurl: '',
      Geositeurl: '',
      LastUpdated: '1787658176',
      DnsHosts: {
        'cloudflare-dns.com': '1.1.1.1',
        'dns.alidns.com': '223.5.5.5',
      },
      DirectSites: ['geosite:private', 'geosite:cn', 'geosite:geolocation-cn'],
      DirectIp: [
        'geoip:cn',
        '127.0.0.0/8',
        '10.0.0.0/8',
        '172.16.0.0/12',
        '192.168.0.0/16',
        '169.254.0.0/16',
        '224.0.0.0/4',
        '255.255.255.255',
      ],
      ProxySites: [],
      ProxyIp: [],
      BlockSites: [],
      BlockIp: [],
      DomainStrategy: 'IPIfNonMatch',
      FakeDNS: 'false',
      UseChunkFiles: 'true',
    });
  });

  it.each(['iran-bypass', 'china-direct', 'global'])(
    'adds ad blocking only when opted in for %s without changing its routing',
    (preset) => {
      const decode = (link: string) => JSON.parse(atob(link.replace('happ://routing/onadd/', '')));
      const base = decode(buildHappPresetDeeplink(preset));
      const withAds = decode(buildHappPresetDeeplink(preset, true));
      const withoutAds = decode(buildHappPresetDeeplink(preset, false));

      expect(base.BlockSites).toEqual([]);
      expect(withAds.BlockSites).toEqual(['geosite:category-ads-all']);
      expect({ ...withAds, BlockSites: [] }).toEqual(base);
      expect(withoutAds).toEqual(base);
    },
  );

  it('keeps routing disabled even when ad blocking is selected', () => {
    expect(buildHappPresetDeeplink('off', true)).toBe('happ://routing/off');
  });

  it.each(['adblock', 'unknown'])(
    'does not generate a profile for unsupported preset %s',
    (preset) => {
      expect(buildHappPresetDeeplink(preset)).toBe('');
    },
  );

  it('generates valid base64 payload for global preset', () => {
    const link = buildHappPresetDeeplink('global');
    const b64 = link.replace('happ://routing/onadd/', '');
    const jsonStr = atob(b64);
    const parsed = JSON.parse(jsonStr);

    expect(parsed.Name).toBe('Global Proxy');
    expect(parsed.DomainStrategy).toBe('AsIs');
  });

  it('encodes unicode properly via toBase64Utf8', () => {
    const text = 'فیلترشکن و روتینگ';
    const b64 = toBase64Utf8(text);
    const decoded = decodeURIComponent(
      Array.prototype.map
        .call(atob(b64), (c: string) => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2))
        .join(''),
    );
    expect(decoded).toBe(text);
  });
});
