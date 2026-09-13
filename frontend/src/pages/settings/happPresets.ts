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
export function buildHappPresetDeeplink(preset: string): string {
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
            BlockSites: ['geosite:category-ads-all'],
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
            Name: 'China Direct',
            GlobalProxy: 'true',
            DirectSites: ['geosite:cn', 'geosite:geolocation-cn'],
            DirectIp: ['geoip:cn', '10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16'],
            BlockSites: ['geosite:category-ads-all'],
            BlockIp: [],
            ProxySites: [],
            ProxyIp: [],
            DomainStrategy: 'IPIfNonMatch',
          }),
        )
      );
    case 'adblock':
      return (
        'happ://routing/onadd/' +
        toBase64Utf8(
          JSON.stringify({
            Name: 'AdBlock',
            GlobalProxy: 'true',
            DirectSites: [],
            DirectIp: ['10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16'],
            BlockSites: ['geosite:category-ads-all'],
            BlockIp: [],
            ProxySites: [],
            ProxyIp: [],
            DomainStrategy: 'IPIfNonMatch',
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
            BlockSites: [],
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
