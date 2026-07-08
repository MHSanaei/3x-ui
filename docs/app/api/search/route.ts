import { source } from '@/lib/source';
import { createFromSource } from 'fumadocs-core/search/server';

// Required for `output: 'export'` — the search index is fully static.
export const revalidate = false;
export const dynamic = 'force-static';

// Static search index: works under both SSR/Vercel and static export
// (`output: 'export'`). The client loads this prebuilt index and searches
// in-browser (see the `type: 'static'` search option in app/[lang]/layout.tsx).
// All locales currently hold English (fallback) content, and Orama has no
// Persian tokenizer, so map every locale to the English tokenizer. When real
// translations land, switch ru -> 'russian', zh -> 'mandarin' (with
// @orama/tokenizers), etc. See https://docs.orama.com/open-source/supported-languages
export const { staticGET: GET } = createFromSource(source, {
  localeMap: {
    en: 'english',
    fa: 'english',
    ru: 'english',
    zh: 'english',
  },
});
