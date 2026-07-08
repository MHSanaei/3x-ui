import { defineI18nUI } from 'fumadocs-ui/i18n';
import { i18n } from './i18n';

// UI-side i18n config: provides the language-switcher display names and the
// `provider(locale)` props consumed by <RootProvider i18n={...}>.
export const { provider } = defineI18nUI(i18n, {
  en: { displayName: 'English' },
  fa: { displayName: 'فارسی' },
  ru: { displayName: 'Русский' },
  zh: { displayName: '中文' },
});
