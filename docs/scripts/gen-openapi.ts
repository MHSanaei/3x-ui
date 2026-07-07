import { readFileSync } from 'node:fs';
import { basename } from 'node:path';
import { generateFiles } from 'fumadocs-openapi';
import { openapi } from '../lib/openapi.ts';

// Map a slugified tag name back to its clean display name, so generated page
// titles read "API Tokens" instead of the auto-split "A P I Tokens".
const spec = JSON.parse(readFileSync('./public/openapi.json', 'utf8')) as {
  tags?: { name: string }[];
};
const titleBySlug = new Map(
  (spec.tags ?? []).map((t) => [t.name.toLowerCase().replace(/\s+/g, '-'), t.name]),
);

// Generate one MDX page per tag into the English reference/api folder.
// Other locales fall back to English for the API reference.
await generateFiles({
  input: openapi,
  output: './content/docs/en/reference/api',
  per: 'tag',
  beforeWrite(files) {
    for (const file of files) {
      const slug = basename(file.path).replace(/\.mdx$/, '');
      const title = titleBySlug.get(slug);
      if (title) {
        file.content = file.content.replace(/^title:.*$/m, `title: ${title}`);
      }
    }
  },
});

console.log('Generated API reference pages.');
