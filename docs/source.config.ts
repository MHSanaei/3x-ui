import { defineConfig, defineDocs } from 'fumadocs-mdx/config';
import { metaSchema, pageSchema } from 'fumadocs-core/source/schema';
import { z } from 'zod';

// `title` is already required by `pageSchema`. We additionally require a
// non-empty `description` on every page (used for SEO, search, and OG images),
// so the build fails fast if a page is missing it.
// See https://fumadocs.dev/docs/mdx/collections
export const docs = defineDocs({
  dir: 'content/docs',
  docs: {
    schema: pageSchema.extend({
      description: z
        .string({ message: 'Every doc page needs a frontmatter `description`.' })
        .min(1, 'Frontmatter `description` cannot be empty.'),
    }),
    postprocess: {
      includeProcessedMarkdown: true,
    },
  },
  meta: {
    schema: metaSchema,
  },
});

export default defineConfig({
  mdxOptions: {
    // MDX options
  },
});
