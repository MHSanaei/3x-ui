import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { z } from 'zod';
import enValidation from 'zod/v4/locales/en.js';

const translationDir = resolve(process.cwd(), '../internal/web/translation');
const readCatalog = (name: string) =>
  JSON.parse(readFileSync(resolve(translationDir, name), 'utf8')) as Record<string, unknown>;

function flatten(obj: Record<string, unknown>, prefix = ''): Record<string, string> {
  const result: Record<string, string> = {};
  for (const [key, value] of Object.entries(obj)) {
    const path = prefix ? `${prefix}.${key}` : key;
    if (typeof value === 'string') result[path] = value;
    else Object.assign(result, flatten(value as Record<string, unknown>, path));
  }
  return result;
}

const tokens = (text: string, pattern: RegExp) =>
  [...text.matchAll(pattern)].map((m) => m[0]).sort();

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  z.config(enValidation());
});

describe('Slovak catalog', () => {
  it('preserves every key, substitution, HTML tag and message line break', () => {
    const english = flatten(readCatalog('en-US.json'));
    const slovak = flatten(readCatalog('sk-SK.json'));
    expect(Object.keys(slovak)).toEqual(Object.keys(english));
    for (const [key, value] of Object.entries(english)) {
      expect(slovak[key].trim(), key).not.toBe('');
      for (const pattern of [
        /\{\{[^{}]*\}\}|\{[^{}]*\}|#[A-Za-z][A-Za-z0-9_]*#/g,
        /<\/?[A-Za-z][^>]*>/g,
        /\r\n|\n/g,
      ]) {
        expect(tokens(slovak[key], pattern), key).toEqual(tokens(value, pattern));
      }
    }
  });

  it.each(['sk', 'sk-SK'])(
    'detects %s and loads Slovak without using the English fallback',
    async (browserLanguage) => {
      vi.resetModules();
      const utils = await import('@/utils');
      const cookies = new Map<string, string>();
      vi.spyOn(utils.CookieManager, 'getCookie').mockImplementation(
        (name) => cookies.get(name) ?? '',
      );
      vi.spyOn(utils.CookieManager, 'setCookie').mockImplementation((name, value) => {
        cookies.set(name, value);
      });
      const reload = vi.fn();
      vi.stubGlobal('window', { navigator: { language: browserLanguage }, location: { reload } });
      const { readyI18n } = await import('@/i18n/react');
      const i18n = await readyI18n();
      expect(cookies.get('lang')).toBe('sk-SK');
      expect(reload).not.toHaveBeenCalled();
      expect(i18n.resolvedLanguage).toBe('sk-SK');
      expect(i18n.t('menu.dashboard')).toBe('Prehľad');
      expect(utils.SizeFormatter.sizeFormat(1536)).toBe('1,50 KB');
      expect(utils.CPUFormatter.cpuCoreFormat(2)).toBe('Jadrá: 2');
      expect(utils.NumberFormatter.toFixed(1.234, 2)).toBe(1.23);
      expect(i18n.t('pages.clients.totalGB')).toBe('Dátový limit (GB)');
      expect(i18n.t('pages.clients.subLinksCopiedAll', { count: 4 })).toBe('Skopírované odkazy: 4');
      const { z: localizedZod } = await import('zod');
      const invalid = localizedZod.number().min(0).safeParse(-1);
      expect(invalid.success).toBe(false);
      if (!invalid.success) expect(invalid.error.issues[0].message).not.toMatch(/Too small/);
    },
  );

  it('keeps the panel language independent of a Slovak subscription', async () => {
    vi.resetModules();
    const utils = await import('@/utils');
    const cookies = new Map<string, string>([
      ['lang', 'en-US'],
      ['subLang', 'sk-SK'],
    ]);
    vi.spyOn(utils.CookieManager, 'getCookie').mockImplementation(
      (name) => cookies.get(name) ?? '',
    );
    vi.spyOn(utils.CookieManager, 'setCookie').mockImplementation((name, value) => {
      cookies.set(name, value);
    });
    vi.stubGlobal('window', { navigator: { language: 'en-US' }, location: { reload: vi.fn() } });
    const { readyI18n } = await import('@/i18n/react');
    const i18n = await readyI18n('subscription');
    expect(i18n.t('subscription.copyAllConfigs')).toBe('Skopírovať všetky konfigurácie');
    expect(cookies.get('lang')).toBe('en-US');
    const date = utils.IntlUtil.formatDate(Date.UTC(2026, 9, 21, 12), 'gregorian', 'sk-SK');
    expect(date).toMatch(/21\.\s*10\.\s*2026/);
    expect(date).not.toMatch(/AM|PM/);
    await readyI18n('panel');
    expect(i18n.t('menu.dashboard')).toBe('Overview');
  });
});
