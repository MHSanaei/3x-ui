// vue-i18n setup. Locale files live in web/translation/*.json — the same
// directory the Go binary embeds, so SPA + Telegram bot + subscription
// page all read from a single source.
//
// Usage in a component:
//   import { useI18n } from 'vue-i18n';
//   const { t } = useI18n();
//   ...
//   <span>{{ t('pages.inbounds.email') }}</span>
//
// Or via the global helper exposed on the app:
//   <span>{{ $t('pages.inbounds.email') }}</span>
//
// The locale follows the `lang` cookie that LanguageManager already
// reads/writes — switching language anywhere in the app continues to
// trigger a full page reload (matches legacy ergonomics), so we don't
// need a runtime locale switcher here.

import { createI18n } from 'vue-i18n';

import { LanguageManager } from '@/utils';

// Lazy-loaded locales — Vite splits each one into its own chunk. We
// eager-load only the active language plus the en-US fallback so the
// initial page payload stays small (the inbounds bundle was sitting
// at ~700kB gzipped with all 13 locales eager; now ~480kB).
//
// LanguageManager.setLanguage() does a full reload on change, so
// "lazy" here effectively means "load only what this page needs for
// its lifetime."
const FALLBACK = 'en-US';
const lazyModules = import.meta.glob('../../../web/translation/*.json');
const eagerModules = import.meta.glob('../../../web/translation/*.json', { eager: true });

function moduleKeyFor(code) {
  return `../../../web/translation/${code}.json`;
}

// Resolve the active locale via LanguageManager so the cookie set on
// the legacy panel keeps working after a user upgrades. Falls back
// to en-US when the cookie names a language we don't have.
let active = LanguageManager.getLanguage();
if (!Object.prototype.hasOwnProperty.call(lazyModules, moduleKeyFor(active))) {
  active = FALLBACK;
}

const messages = {};
// Eagerly include the active locale + the fallback (when distinct)
// so the very first render has strings ready. Vite still emits these
// as their own chunks so the user pays for at most two locales.
for (const code of new Set([active, FALLBACK])) {
  const mod = eagerModules[moduleKeyFor(code)];
  if (mod) messages[code] = mod.default || mod;
}

export const i18n = createI18n({
  legacy: false,
  // `composition` mode (legacy: false) so `useI18n()` works in
  // <script setup> blocks.
  globalInjection: true,
  locale: active,
  fallbackLocale: FALLBACK,
  // Locale JSON is nested by namespace ({pages: {inbounds: {email: ...}}})
  // so vue-i18n's default `.`-delimited lookups walk straight into it.
  messages,
  // The Go side sometimes interpolates `#variable#` into translated
  // strings (e.g. xraySwitchVersionDialogDesc). vue-i18n's default
  // expects `{var}` — disable warnings about strings that look like
  // they don't use the new syntax.
  warnHtmlMessage: false,
  missingWarn: false,
  fallbackWarn: false,
});

// Convenience export for non-component contexts (HTTP error toasts,
// stores, etc.) that need to look up a translation outside a setup
// scope.
export function t(key, params) {
  return i18n.global.t(key, params || {});
}

// loadLocale fetches a locale module on demand and registers it with
// vue-i18n. Pages that switch language at runtime (rather than via
// LanguageManager's reload) can call this to swap strings live.
export async function loadLocale(code) {
  const key = moduleKeyFor(code);
  const loader = lazyModules[key];
  if (!loader) return false;
  const mod = await loader();
  i18n.global.setLocaleMessage(code, mod.default || mod);
  i18n.global.locale.value = code;
  return true;
}
