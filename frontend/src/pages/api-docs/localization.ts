import type { TFunction } from 'i18next';

import enUS from '../../../../internal/web/translation/en-US.json';

export type ApiDocumentValue =
  | null
  | boolean
  | number
  | string
  | ApiDocumentValue[]
  | { [key: string]: ApiDocumentValue };

const referenceKeys = new Map(
  Object.entries(enUS.apiDocs.reference).map(([key, text]) => [text, key]),
);
const payloadKeys = new Set(['example', 'examples', 'default', 'enum', 'const']);
const proseKeys = new Set(['title', 'summary', 'description']);

export function translateApiText(text: string, t: TFunction): string {
  const key = referenceKeys.get(text);
  return key ? t('apiDocs.reference.' + key, { defaultValue: text }) : text;
}

export function localizeApiDocument(value: ApiDocumentValue, t: TFunction): ApiDocumentValue {
  if (Array.isArray(value)) return value.map((child) => localizeApiDocument(child, t));
  if (!value || typeof value !== 'object') return value;
  return Object.fromEntries(
    Object.entries(value).map(([key, child]) => {
      if (payloadKeys.has(key)) return [key, child];
      if (proseKeys.has(key) && typeof child === 'string') {
        return [key, translateApiText(child, t)];
      }
      if (key === 'tags' && Array.isArray(child)) {
        return [
          key,
          child.map((tag) => {
            if (typeof tag === 'string') return translateApiText(tag, t);
            const localized = localizeApiDocument(tag, t);
            if (
              localized &&
              typeof localized === 'object' &&
              !Array.isArray(localized) &&
              typeof localized.name === 'string'
            ) {
              return { ...localized, name: translateApiText(localized.name, t) };
            }
            return localized;
          }),
        ];
      }
      return [key, localizeApiDocument(child, t)];
    }),
  );
}

export function apiReferencePlugin(t: TFunction) {
  return {
    statePlugins: {
      spec: {
        wrapActions: {
          updateJsonSpec:
            (original: (spec: ApiDocumentValue) => unknown) => (spec: ApiDocumentValue) =>
              original(localizeApiDocument(spec, t)),
        },
      },
    },
  };
}
