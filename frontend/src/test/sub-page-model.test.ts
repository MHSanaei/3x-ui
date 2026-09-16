import { describe, expect, it } from 'vitest';

import {
  buildSubApps,
  daysUntil,
  detectPlatform,
  resolveSubStatus,
  usagePercent,
} from '@/pages/sub/subPageModel';

const DAY = 86_400_000;
const NOW = Date.UTC(2026, 8, 15, 12, 0, 0);

describe('resolveSubStatus', () => {
  const base = { enabled: true, usedByte: 10, totalByte: 100, expireMs: NOW + DAY };

  it.each([
    [
      'disabled wins over expiry and quota',
      { ...base, enabled: false, expireMs: NOW - DAY },
      'disabled',
    ],
    ['expired at the expiry instant', { ...base, expireMs: NOW }, 'expired'],
    ['expired beats depleted', { ...base, usedByte: 100, expireMs: NOW - DAY }, 'expired'],
    ['depleted once usage reaches the quota', { ...base, usedByte: 100 }, 'depleted'],
    [
      'unlimited with neither quota nor expiry',
      { ...base, totalByte: 0, expireMs: 0 },
      'unlimited',
    ],
    ['active without quota but with a future expiry', { ...base, totalByte: 0 }, 'active'],
    ['active inside quota and before expiry', base, 'active'],
  ] as const)('%s', (_name, input, want) => {
    expect(resolveSubStatus(input, NOW)).toBe(want);
  });
});

describe('daysUntil', () => {
  it('is null for a subscription that never expires', () => {
    expect(daysUntil(0, NOW)).toBeNull();
  });

  it('rounds the last partial day up to 1', () => {
    expect(daysUntil(NOW + 3 * 3_600_000, NOW)).toBe(1);
  });

  it('counts whole days', () => {
    expect(daysUntil(NOW + 23 * DAY, NOW)).toBe(23);
  });

  it('stays at 0 after expiry', () => {
    expect(daysUntil(NOW - DAY, NOW)).toBe(0);
  });
});

describe('usagePercent', () => {
  it('is 0 without a quota instead of NaN or Infinity', () => {
    expect(usagePercent(5_000, 0)).toBe(0);
  });

  it('clamps an over-quota client to 100', () => {
    expect(usagePercent(150, 100)).toBe(100);
  });
});

describe('detectPlatform', () => {
  it.each([
    [
      'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 Chrome/128.0 Mobile Safari/537.36',
      'android',
    ],
    [
      'Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148',
      'ios',
    ],
    [
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Version/17.5 Safari/605.1.15',
      'ios',
    ],
    [
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/128.0 Safari/537.36',
      'android',
    ],
    ['Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/128.0 Safari/537.36', 'android'],
  ])('%s -> %s', (ua, want) => {
    expect(detectPlatform(ua)).toBe(want);
  });
});

describe('buildSubApps', () => {
  const subUrl = 'https://sub.example.com/sub/abc';
  const encSub = encodeURIComponent(subUrl);
  const sub = { subUrl, sId: 'abc', subTitle: 'Nova Net' };

  it('offers Android and iOS app lists only', () => {
    expect(Object.keys(buildSubApps(sub))).toEqual(['android', 'ios']);
  });

  it('gives every Android app a one-tap import link', () => {
    expect(buildSubApps(sub).android).toEqual([
      { name: 'V2Box', url: `v2box://install-sub?url=${encSub}&name=abc` },
      { name: 'V2RayNG', url: `v2rayng://install-config?url=${encSub}` },
      { name: 'Sing-box', url: `sing-box://import-remote-profile?url=${encSub}#Nova%20Net` },
      { name: 'V2RayTun', url: `v2raytun://import/${subUrl}` },
      { name: 'Happ', url: `happ://add/${subUrl}` },
      { name: 'Incy', url: `incy://add/${subUrl}` },
    ]);
  });

  it('gives every iOS app a one-tap import link', () => {
    const rocket = Buffer.from(`${subUrl}?flag=shadowrocket`).toString('base64');
    expect(buildSubApps(sub).ios).toEqual([
      { name: 'Shadowrocket', url: `shadowrocket://add/sub://${rocket}?remark=Nova%20Net` },
      { name: 'V2Box', url: `v2box://install-sub?url=${encSub}&name=abc` },
      { name: 'Streisand', url: `streisand://import/${encSub}` },
      { name: 'V2RayTun', url: `v2raytun://import/${subUrl}` },
      { name: 'Happ', url: `happ://add/${subUrl}` },
      { name: 'Incy', url: `incy://add/${subUrl}` },
    ]);
  });

  it('names the sing-box profile after the subscription id when there is no title', () => {
    expect(buildSubApps({ ...sub, subTitle: '' }).android[2].url).toBe(
      `sing-box://import-remote-profile?url=${encSub}#abc`,
    );
  });

  it('appends flag=shadowrocket with & when the subscription URL already has a query', () => {
    const withQuery = { ...sub, subUrl: `${subUrl}?token=1` };
    const rocket = Buffer.from(`${subUrl}?token=1&flag=shadowrocket`).toString('base64');
    expect(buildSubApps(withQuery).ios[0].url).toBe(
      `shadowrocket://add/sub://${rocket}?remark=Nova%20Net`,
    );
  });
});
