import { createI18nMiddleware } from 'fumadocs-core/i18n/middleware';
import { i18n } from '@/lib/i18n';

// Next 16 "proxy" (middleware). The i18n middleware detects the locale and
// rewrites `/docs/...` -> `/en/docs/...` internally (en is the hidden default),
// while `/fa/...`, `/ru/...`, `/zh/...` keep their prefix.
export default createI18nMiddleware(i18n);

export const config = {
  // Run on everything except API routes, Next internals, and files with an
  // extension (og images, llms.txt, sitemap.xml, favicon, content.md, ...).
  matcher: ['/((?!api|_next|.*\\..*).*)'],
};
