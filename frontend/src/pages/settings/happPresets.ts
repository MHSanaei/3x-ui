// Encode UTF-8 string to standard Base64 safe for browser environments.
export function toBase64Utf8(str: string): string {
  return btoa(
    encodeURIComponent(str).replace(/%([0-9A-F]{2})/g, (_, p1) =>
      String.fromCharCode(Number(`0x${p1}`)),
    ),
  );
}

// Splits multiline or comma-separated string into clean unique token arrays.
export function parseList(input: string): string[] {
  return input
    .split(/[\n,]+/)
    .map((s) => s.trim())
    .filter(Boolean);
}

// Build standard Happ routing deeplink or special state for curated presets.
export function buildHappPresetDeeplink(preset: string, includeAdblock = false): string {
  // Ad blocking is opt-in for each generated profile, independent of the base routing rules.
  const blockSites = includeAdblock ? ['geosite:category-ads-all'] : [];

  switch (preset) {
    case 'off':
      return 'happ://routing/off';
    case 'iran-bypass':
      return (
        'happ://routing/onadd/' +
        toBase64Utf8(
          JSON.stringify({
            Name: 'Iran Bypass',
            GlobalProxy: 'true',
            DirectSites: ['domain:ir', 'regexp:.*\\.ir$'],
            DirectIp: ['geoip:ir', '10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16'],
            BlockSites: blockSites,
            BlockIp: [],
            ProxySites: [],
            ProxyIp: [],
            DomainStrategy: 'IPIfNonMatch',
          }),
        )
      );
    case 'china-direct':
      return (
        'happ://routing/onadd/' +
        toBase64Utf8(
          JSON.stringify({
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
            BlockSites: blockSites,
            BlockIp: [],
            DomainStrategy: 'IPIfNonMatch',
            FakeDNS: 'false',
            UseChunkFiles: 'true',
          }),
        )
      );
    case 'global':
      return (
        'happ://routing/onadd/' +
        toBase64Utf8(
          JSON.stringify({
            Name: 'Global Proxy',
            GlobalProxy: 'true',
            DirectSites: [],
            DirectIp: [],
            BlockSites: blockSites,
            BlockIp: [],
            ProxySites: [],
            ProxyIp: [],
            DomainStrategy: 'AsIs',
          }),
        )
      );
    default:
      return '';
  }
}
