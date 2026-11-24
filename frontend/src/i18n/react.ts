import i18next from 'i18next';
import type { InterpolationOptions } from 'i18next';
import { initReactI18next } from 'react-i18next';

import { LanguageManager } from '@/utils';
import type { LanguageScope } from '@/utils';
import { loadAntdLocale } from './AntdRootConfig';
import enUS from '../../../internal/web/translation/en-US.json';

const FALLBACK = 'en-US';

// No locale uses $t(...) nesting, and untrusted text (node-reported names, remarks) must not
// reach it; i18next 27 has no off switch, so the prefix is a NUL that no text contains.
export const INTERPOLATION: InterpolationOptions = {
  escapeValue: false,
  prefix: '{',
  suffix: '}',
  nestingPrefix: '\u0000',
};

const lazyModules = import.meta.glob([
  '../../../internal/web/translation/*.json',
  '!../../../internal/web/translation/en-US.json',
]);

function moduleKeyFor(code: string): string {
  return `../../../internal/web/translation/${code}.json`;
}

export async function readyI18n(scope: LanguageScope = 'panel') {
  let active = LanguageManager.getLanguage(scope);
  if (
    active !== FALLBACK &&
    !Object.prototype.hasOwnProperty.call(lazyModules, moduleKeyFor(active))
  ) {
    active = FALLBACK;
  }

  await i18next.use(initReactI18next).init({
    lng: active,
    fallbackLng: FALLBACK,
    resources: { [FALLBACK]: { translation: enUS } },
    interpolation: INTERPOLATION,
    returnNull: false,
  });
  if (active !== FALLBACK) {
    const loader = lazyModules[moduleKeyFor(active)] as
      | (() => Promise<{ default: Record<string, unknown> }>)
      | undefined;
    if (loader) {
      const mod = await loader();
      const messages = (mod.default ?? mod) as Record<string, unknown>;
      i18next.addResourceBundle(active, 'translation', messages, true, true);
      await i18next.changeLanguage(active);
    }
  }
  // The html files ship lang="en"; screen readers pick their voice from it (s-ui#1293).
  document.documentElement.lang = i18next.language;
  await loadAntdLocale(i18next.language);
  return i18next;
}

export { i18next as i18n };
