import { afterEach, describe, expect, it, vi } from 'vitest';

// Screen readers choose their voice from <html lang>; the shipped html says "en"
// for every locale, so NVDA read Persian panels with an English voice (s-ui#1293).
describe('readyI18n page language', () => {
  afterEach(() => {
    document.cookie = 'lang=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/';
    document.cookie = 'subLang=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/';
    document.documentElement.lang = 'en';
  });

  it.each([
    ['panel', 'lang', 'fa-IR'],
    ['subscription', 'subLang', 'zh-TW'],
  ] as const)('sets <html lang> to the active %s locale', async (scope, cookie, locale) => {
    vi.resetModules();
    document.cookie = `${cookie}=${locale}; path=/`;
    const { readyI18n } = await import('@/i18n/react');

    await readyI18n(scope);

    expect(document.documentElement.lang).toBe(locale);
  });

  it('falls back to en-US when the cookie names an unknown locale', async () => {
    vi.resetModules();
    document.cookie = 'lang=xx-XX; path=/';
    const { readyI18n } = await import('@/i18n/react');

    await readyI18n('panel');

    expect(document.documentElement.lang).toBe('en-US');
  });
});
