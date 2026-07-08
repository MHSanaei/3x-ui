import { defineI18n } from 'fumadocs-core/i18n';

// Directory-based i18n: content lives in `content/docs/{locale}/...`.
// `fallbackLanguage` defaults to `defaultLanguage` ('en'), so untranslated
// pages transparently serve English instead of 404ing.
export const i18n = defineI18n({
  defaultLanguage: 'en',
  languages: ['en', 'fa', 'ru', 'zh'],
  parser: 'dir',
  // English keeps canonical `/docs/...`; other locales are prefixed `/fa/...`.
  hideLocale: 'default-locale',
});

export type Locale = (typeof i18n.languages)[number];

// Display names for the language switcher (in their own script).
export const locales: { locale: Locale; name: string }[] = [
  { locale: 'en', name: 'English' },
  { locale: 'fa', name: 'فارسی' },
  { locale: 'ru', name: 'Русский' },
  { locale: 'zh', name: '中文' },
];

// Right-to-left locales (Persian). Drives `<html dir>` and `rtl:` variants.
const rtlLocales = new Set<string>(['fa']);

export function localeDirection(locale: string): 'rtl' | 'ltr' {
  return rtlLocales.has(locale) ? 'rtl' : 'ltr';
}
