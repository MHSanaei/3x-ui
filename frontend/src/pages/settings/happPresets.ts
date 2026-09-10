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
            rules: [
              {
                type: 'field',
                outboundTag: 'direct',
                domain: ['domain:ir', 'regexp:.*\\.ir$'],
                ip: ['geoip:ir', 'geoip:private'],
              },
              {
                type: 'field',
                outboundTag: 'block',
                domain: ['geosite:category-ads-all'],
              },
              {
                type: 'field',
                outboundTag: 'proxy',
                network: 'tcp,udp',
              },
            ],
          }),
        )
      );
    case 'china-direct':
      return (
        'happ://routing/onadd/' +
        toBase64Utf8(
          JSON.stringify({
            rules: [
              {
                type: 'field',
                outboundTag: 'direct',
                domain: ['domain:cn', 'geosite:cn'],
                ip: ['geoip:cn', 'geoip:private'],
              },
              {
                type: 'field',
                outboundTag: 'block',
                domain: ['geosite:category-ads-all'],
              },
              {
                type: 'field',
                outboundTag: 'proxy',
                network: 'tcp,udp',
              },
            ],
          }),
        )
      );
    case 'adblock':
      return (
        'happ://routing/onadd/' +
        toBase64Utf8(
          JSON.stringify({
            rules: [
              {
                type: 'field',
                outboundTag: 'block',
                domain: ['geosite:category-ads-all'],
              },
              {
                type: 'field',
                outboundTag: 'direct',
                ip: ['geoip:private'],
              },
              {
                type: 'field',
                outboundTag: 'proxy',
                network: 'tcp,udp',
              },
            ],
          }),
        )
      );
    case 'global':
      return (
        'happ://routing/onadd/' +
        toBase64Utf8(
          JSON.stringify({
            rules: [
              {
                type: 'field',
                outboundTag: 'proxy',
                network: 'tcp,udp',
              },
            ],
          }),
        )
      );
    default:
      return '';
  }
}
