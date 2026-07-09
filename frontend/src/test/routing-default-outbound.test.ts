import { describe, expect, it } from 'vitest';

import type { XraySettingsValue } from '@/hooks/useXraySetting';
import { getDefaultOutboundTag, setDefaultOutboundTag } from '@/pages/xray/basics/helpers';

function tpl(
  outbounds: Array<{ tag?: string; protocol?: string; settings?: unknown }>,
  rules: Array<{ type: string; outboundTag?: string; ip?: string[]; protocol?: string[] }> = [],
): XraySettingsValue {
  return { outbounds, routing: { rules } } as XraySettingsValue;
}

describe('routing default outbound', () => {
  it('reads first outbound tag', () => {
    expect(getDefaultOutboundTag(tpl([{ tag: 'warp', protocol: 'socks' }, { tag: 'direct', protocol: 'freedom' }]))).toBe('warp');
    expect(getDefaultOutboundTag(tpl([]))).toBe('direct');
  });

  it('moves existing outbound to first position', () => {
    const tt = tpl([
      { tag: 'direct', protocol: 'freedom' },
      { tag: 'warp', protocol: 'socks' },
      { tag: 'blocked', protocol: 'blackhole' },
    ]);
    setDefaultOutboundTag(tt, 'warp');
    expect(tt.outbounds!.map((o) => o?.tag)).toEqual(['warp', 'direct', 'blocked']);
  });

  it('creates blocked outbound when missing', () => {
    const tt = tpl([{ tag: 'direct', protocol: 'freedom' }]);
    setDefaultOutboundTag(tt, 'blocked');
    expect(tt.outbounds![0]?.tag).toBe('blocked');
    expect(tt.outbounds![0]?.protocol).toBe('blackhole');
  });

  it('does not prune direct when only blocked rules reference an outbound', () => {
    const tt = tpl(
      [
        { tag: 'direct', protocol: 'freedom', settings: { domainStrategy: 'AsIs' } },
        { tag: 'blocked', protocol: 'blackhole' },
        { tag: 'warp', protocol: 'socks' },
      ],
      [
        { type: 'field', ip: ['geoip:private'], outboundTag: 'blocked' },
        { type: 'field', protocol: ['bittorrent'], outboundTag: 'blocked' },
      ],
    );
    setDefaultOutboundTag(tt, 'warp');
    expect(tt.outbounds!.map((o) => o?.tag)).toEqual(['warp', 'direct', 'blocked']);
  });
});
