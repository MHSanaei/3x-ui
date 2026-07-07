import type { MetadataRoute } from 'next';
import { source } from '@/lib/source';
import { i18n } from '@/lib/i18n';
import { siteUrl } from '@/lib/shared';

// Required for `output: 'export'`.
export const dynamic = 'force-static';

// Locale home pages + the canonical (English) docs pages. Other locales
// currently fall back to English content, so we don't list them separately
// to avoid duplicate-content entries until real translations exist.
export default function sitemap(): MetadataRoute.Sitemap {
  const entries: MetadataRoute.Sitemap = [];

  for (const lang of i18n.languages) {
    const prefix = lang === 'en' ? '' : `/${lang}`;
    entries.push({
      url: `${siteUrl}${prefix}` || siteUrl,
      changeFrequency: 'weekly',
      priority: 1,
    });
  }

  for (const page of source.getPages('en')) {
    entries.push({
      url: `${siteUrl}${page.url}`,
      changeFrequency: 'weekly',
      priority: 0.8,
    });
  }

  return entries;
}
