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
    expect(parsed.BlockSites).toContain('geosite:category-ads-all');
  });

  it('generates valid base64 payload for china-direct preset', () => {
    const link = buildHappPresetDeeplink('china-direct');
    expect(link.startsWith('happ://routing/onadd/')).toBe(true);

    const b64 = link.replace('happ://routing/onadd/', '');
    const jsonStr = atob(b64);
    const parsed = JSON.parse(jsonStr);

    expect(parsed.Name).toBe('China Direct');
    expect(parsed.DirectSites).toContain('geosite:cn');
    expect(parsed.DirectIp).toContain('geoip:cn');
  });

  it('generates valid base64 payload for adblock preset', () => {
    const link = buildHappPresetDeeplink('adblock');
    const b64 = link.replace('happ://routing/onadd/', '');
    const jsonStr = atob(b64);
    const parsed = JSON.parse(jsonStr);

    expect(parsed.Name).toBe('AdBlock');
    expect(parsed.BlockSites).toContain('geosite:category-ads-all');
  });

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
