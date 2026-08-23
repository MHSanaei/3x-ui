import { describe, expect, it } from 'vitest';

import { generateAwgObfuscation } from '@/lib/xray/amneziawg-obfuscation';
import { AmneziawgServerSchema } from '@/schemas/protocols/inbound/amneziawg';
import { ServerSettingsSchema } from '@/generated/zod';

/*
 * Parses "lo-hi" and asserts min <= lo <= hi <= max; mirrors the bounds the
 * Go generator's own test pins (internal/amneziawg/params_test.go), so the
 * two generators cannot drift apart silently.
 */
function expectRangeWithin(value: string, min: number, max: number): [number, number] {
  const m = /^(\d+)-(\d+)$/.exec(value);
  expect(m, `${value} is not a lo-hi range`).not.toBeNull();
  const lo = Number(m![1]);
  const hi = Number(m![2]);
  expect(lo).toBeGreaterThanOrEqual(min);
  expect(hi).toBeLessThanOrEqual(max);
  expect(lo).toBeLessThanOrEqual(hi);
  return [lo, hi];
}

describe('generateAwgObfuscation', () => {
  it('stays inside the Go generator ranges and invariants', () => {
    for (let i = 0; i < 200; i++) {
      const o = generateAwgObfuscation();

      expect(o.jc).toBeGreaterThanOrEqual(3);
      expect(o.jc).toBeLessThanOrEqual(6);
      expect(o.jmin).toBeGreaterThanOrEqual(40);
      expect(o.jmin).toBeLessThanOrEqual(89);
      expect(o.jmax - o.jmin).toBeGreaterThanOrEqual(50);
      expect(o.jmax - o.jmin).toBeLessThanOrEqual(250);
      expect(o.s1 + 56).not.toBe(o.s2);
      expect(o.s3).toBeGreaterThanOrEqual(12);
      expect(o.s3).toBeLessThanOrEqual(55);
      expect(o.s4).toBeGreaterThanOrEqual(12);
      expect(o.s4).toBeLessThanOrEqual(27);

      const hBounds = [o.h1, o.h2, o.h3, o.h4].map((h) => expectRangeWithin(h, 5, 2147483647));
      for (let j = 1; j < 4; j++) {
        expect(hBounds[j][0], 'H ranges must not overlap').toBeGreaterThan(hBounds[j - 1][1]);
      }

      expect(o.i1).toMatch(/^<r \d+>$/);
      expect(o.i2).toBe('');
      expect(o.i5).toBe('');

      const key = atob(o.headerProtectionKey);
      expect(key.length, 'headerProtectionKey must decode to 32 bytes').toBe(32);

      expectRangeWithin(o.contentPaddingAddition, 8, 64);
      const [, rekeyHi] = expectRangeWithin(o.rekeyAfterTime, 100, 160);
      const [rejectLo] = expectRangeWithin(o.rejectAfterTime, 130, 310);
      expect(
        rejectLo,
        'reject window must start >= 30s above the rekey window',
      ).toBeGreaterThanOrEqual(rekeyHi + 30);
      expectRangeWithin(o.rekeyTimeout, 3, 10);
      expectRangeWithin(o.keepaliveTimeout, 8, 20);
      expectRangeWithin(o.maxHandshakeAttempts, 15, 50);

      expect(o.randomTrailers).toBe(true);
      expect(o.disableCookies).toBe(true);
    }
  });

  it('produces values the hand-written schema accepts unchanged', () => {
    const parsed = AmneziawgServerSchema.parse({
      ...generateAwgObfuscation(),
      privateKey: 'p',
      publicKey: 'P',
    });
    expect(parsed.headerProtectionKey).not.toBe('');
  });
});

/*
 * Drift guard for the three-way mirror: the hand-written AmneziawgServerSchema,
 * the Go ServerSettings struct, and the openapigen output must agree on the
 * field set. Comparing hand-written vs generated keys catches a field added on
 * one side but forgotten on the other before it silently drops from configs.
 */
describe('AmneziawgServerSchema parity with generated ServerSettings', () => {
  it('declares exactly the generated key set', () => {
    const handwritten = Object.keys(AmneziawgServerSchema.shape).sort();
    const generated = Object.keys(ServerSettingsSchema.shape).sort();
    expect(handwritten).toEqual(generated);
  });
});
