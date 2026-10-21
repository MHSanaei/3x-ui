import { describe, expect, it } from 'vitest';
import { createInstance } from 'i18next';

import { buildSpec } from '../../scripts/build-openapi.mjs';
import enUS from '../../../internal/web/translation/en-US.json';
import skSK from '../../../internal/web/translation/sk-SK.json';
import { localizeApiDocument, translateApiText } from '@/pages/api-docs/localization';
import type { ApiDocumentValue } from '@/pages/api-docs/localization';

const examples = new Set(['example', 'examples', 'default', 'enum', 'const']);
const prose = new Set(['title', 'summary', 'description']);

function proseValues(value: unknown): string[] {
  if (Array.isArray(value)) return value.flatMap(proseValues);
  if (!value || typeof value !== 'object') return [];
  return Object.entries(value).flatMap(([key, child]) => {
    if (examples.has(key)) return [];
    if (prose.has(key) && typeof child === 'string') return child ? [child] : [];
    if (key === 'tags' && Array.isArray(child)) {
      return child.flatMap((tag) =>
        typeof tag === 'string' ? [tag] : [tag.name, ...proseValues(tag)],
      );
    }
    return proseValues(child);
  });
}

function technicalContract(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(technicalContract);
  if (!value || typeof value !== 'object') return value;
  return Object.fromEntries(
    Object.entries(value).flatMap(([key, child]) => {
      if (examples.has(key)) return [[key, child]];
      if (prose.has(key) && typeof child === 'string') return [];
      if (key === 'tags' && Array.isArray(child)) {
        return [
          [
            key,
            child.map((tag) =>
              typeof tag === 'string' ? null : technicalContract({ ...tag, name: null }),
            ),
          ],
        ];
      }
      return [[key, technicalContract(child)]];
    }),
  );
}

async function translator(language: string) {
  const i18n = createInstance();
  await i18n.init({
    lng: language,
    resources: { 'en-US': { translation: enUS }, 'sk-SK': { translation: skSK } },
    interpolation: { prefix: '{', suffix: '}', escapeValue: false },
  });
  return i18n.t;
}

describe('Slovak API reference', () => {
  it('covers every generated explanation and notices new or changed upstream prose', async () => {
    const t = await translator('sk-SK');
    const texts = [...new Set(proseValues(buildSpec()))];
    expect(new Set(Object.values(enUS.apiDocs.reference))).toEqual(new Set(texts));
    expect(texts.map((text) => translateApiText(text, t))).toContain('API panela 3X-UI');
    expect(translateApiText('Client password', t)).toBe('Heslo klienta');
  });

  it('keeps inline code, placeholders and technical error examples verbatim', async () => {
    const t = await translator('sk-SK');
    const tokens = (text: string) =>
      [...text.matchAll(/`([^`]*)`|\{[^{}]*\}/g)]
        .map((match) => match[0].replace(/\s+/g, ' '))
        .sort();
    for (const text of Object.values(enUS.apiDocs.reference)) {
      expect(tokens(translateApiText(text, t)), text).toEqual(tokens(text));
    }
  });

  it('preserves all paths, payload examples, schema rules, references and auth while localizing prose', async () => {
    const spec = buildSpec() as unknown as ApiDocumentValue;
    const original = JSON.stringify(spec);
    const localized = localizeApiDocument(spec, await translator('sk-SK'));
    expect(technicalContract(localized)).toEqual(technicalContract(spec));
    expect(JSON.stringify(spec)).toBe(original);
    expect(localized).toHaveProperty('info.title', 'API panela 3X-UI');
    expect(localized).toHaveProperty(
      'paths./login.post.summary',
      'Overí meno a heslo a vráti cookie relácie. Vyžaduje sa pred každým volaním API založeným na cookie.',
    );
    expect(localizeApiDocument(spec, await translator('en-US'))).toEqual(spec);
  });
});
