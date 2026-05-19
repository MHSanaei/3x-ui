import { createI18n } from 'vue-i18n';

import { LanguageManager } from '@/utils';
import enUS from '../../../web/translation/en-US.json';

const FALLBACK = 'en-US';
const lazyModules = import.meta.glob([
  '../../../web/translation/*.json',
  '!../../../web/translation/en-US.json',
]);

function moduleKeyFor(code) {
  return `../../../web/translation/${code}.json`;
}

let active = LanguageManager.getLanguage();
if (active !== FALLBACK && !Object.prototype.hasOwnProperty.call(lazyModules, moduleKeyFor(active))) {
  active = FALLBACK;
}

export const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: active,
  fallbackLocale: FALLBACK,
  messages: { [FALLBACK]: enUS },
  warnHtmlMessage: false,
  missingWarn: false,
  fallbackWarn: false,
});

export function t(key, params) {
  return i18n.global.t(key, params || {});
}

export async function loadLocale(code) {
  if (code === FALLBACK) {
    i18n.global.locale.value = FALLBACK;
    return true;
  }
  const loader = lazyModules[moduleKeyFor(code)];
  if (!loader) return false;
  const mod = await loader();
  i18n.global.setLocaleMessage(code, mod.default || mod);
  i18n.global.locale.value = code;
  return true;
}

export async function readyI18n() {
  if (active !== FALLBACK) {
    await loadLocale(active);
  }
  return i18n;
}
