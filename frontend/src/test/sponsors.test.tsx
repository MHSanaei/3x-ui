import { afterEach, describe, expect, test } from 'vitest';

import { SPONSOR_DISMISS_MS, dismissSponsor, isSponsorDismissed, pickLocale } from '@/lib/sponsors';

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
    dismissSponsor('acme', 'login', t0);
    expect(isSponsorDismissed('acme', 'login', t0 + SPONSOR_DISMISS_MS - 1)).toBe(true);
    expect(isSponsorDismissed('acme', 'dashboard', t0)).toBe(false);
    expect(isSponsorDismissed('acme', 'login', t0 + SPONSOR_DISMISS_MS)).toBe(false);
  });

  test('prunes expired entries on the next dismiss', () => {
    dismissSponsor('old', 'login', t0);
    dismissSponsor('new', 'login', t0 + SPONSOR_DISMISS_MS);
    const stored = JSON.parse(localStorage.getItem('xui.sponsor.dismissed') || '{}');
    expect(Object.keys(stored)).toEqual(['new:login']);
  });

  test('corrupt storage is treated as nothing dismissed', () => {
    localStorage.setItem('xui.sponsor.dismissed', 'not json');
    expect(isSponsorDismissed('acme', 'login', t0)).toBe(false);
  });
});
