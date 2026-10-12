import { afterEach, describe, expect, test } from 'vitest';

import {
  SPONSOR_DISMISS_MS,
  dismissSponsor,
  isSponsorDismissed,
  pickLocale,
  placementStatus,
} from '@/lib/sponsors';

afterEach(() => {
  localStorage.clear();
});

describe('pickLocale', () => {
  const map = { en: 'Hello', fa: 'سلام', 'zh-TW': '哈囉' };

  test.each([
    ['fa-IR', 'سلام'],
    ['zh-TW', '哈囉'],
    ['zh-CN', 'Hello'],
    ['ru-RU', 'Hello'],
  ])('%s resolves to %s', (lang, want) => {
    expect(pickLocale(map, lang)).toBe(want);
  });

  test('missing map yields empty string', () => {
    expect(pickLocale(undefined, 'en-US')).toBe('');
  });
});

describe('sponsor dismissal', () => {
  const t0 = 1_800_000_000_000;

  test('hides only the dismissed slot until the window elapses', () => {
    dismissSponsor('acme', 'sidebar', t0);
    expect(isSponsorDismissed('acme', 'sidebar', t0 + SPONSOR_DISMISS_MS - 1)).toBe(true);
    expect(isSponsorDismissed('acme', 'dashboard', t0)).toBe(false);
    expect(isSponsorDismissed('acme', 'sidebar', t0 + SPONSOR_DISMISS_MS)).toBe(false);
  });

  test('prunes expired entries on the next dismiss', () => {
    dismissSponsor('old', 'login', t0);
    dismissSponsor('new', 'login', t0 + SPONSOR_DISMISS_MS);
    const stored = JSON.parse(localStorage.getItem('xui.sponsor.dismissed') || '{}');
    expect(Object.keys(stored)).toEqual(['new:login']);
  });

  test('corrupt storage is treated as nothing dismissed', () => {
    localStorage.setItem('xui.sponsor.dismissed', 'not json');
    expect(isSponsorDismissed('acme', 'sidebar', t0)).toBe(false);
  });
});

describe('placementStatus', () => {
  const sp = (id: string, slots: string[], until: string) => ({
    id,
    name: id,
    slots,
    until,
    title: {},
    text: {},
    link: 'https://x.example/',
  });

  test('exclusive slot is taken until the latest booking ends', () => {
    const sponsors = [
      sp('a', ['dashboard'], '2026-11-01T00:00:00Z'),
      sp('b', ['dashboard'], '2026-12-15T00:00:00Z'),
    ];
    expect(placementStatus(sponsors, 'dashboard')).toEqual({
      count: 2,
      capacity: 1,
      takenUntil: '2026-12-15T00:00:00Z',
    });
  });

  test('sidebar is full at three and frees up when the earliest ends', () => {
    const sponsors = [
      sp('a', ['sidebar'], '2026-12-01T00:00:00Z'),
      sp('b', ['sidebar'], '2026-10-20T00:00:00Z'),
      sp('c', ['sidebar'], '2026-11-10T00:00:00Z'),
    ];
    expect(placementStatus(sponsors, 'sidebar')).toEqual({
      count: 3,
      capacity: 3,
      takenUntil: '2026-10-20T00:00:00Z',
    });
    expect(placementStatus(sponsors.slice(0, 2), 'sidebar')).toEqual({ count: 2, capacity: 3 });
  });

  test('uncapped page slot reports only a count', () => {
    expect(placementStatus([sp('a', ['page'], '2026-12-01T00:00:00Z')], 'page')).toEqual({
      count: 1,
      capacity: undefined,
    });
  });
});
