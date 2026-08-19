import { source } from '@/lib/source';
import { createFromSource } from 'fumadocs-core/search/server';

// Required for `output: 'export'` — the search index is fully static.
export const revalidate = false;
export const dynamic = 'force-static';

// Every locale still serves English fallback content, so all map to zbsearch's
// English tokenizer (its SUPPORTED_LANGUAGES has no Persian or Chinese anyway).
export const { staticGET: GET } = createFromSource(source, {
  localeMap: {
    en: 'english',
    fa: 'english',
    ru: 'english',
    zh: 'english',
  },
});
