import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { Inter, Vazirmatn } from 'next/font/google';
import './global.css';
import { appName, appTagline, siteUrl } from '@/lib/shared';
import { i18n, localeDirection } from '@/lib/i18n';

const inter = Inter({ subsets: ['latin'], display: 'swap' });
// Persian UI font; covers Arabic + Latin glyphs so mixed content renders well.
const vazirmatn = Vazirmatn({ subsets: ['arabic'], display: 'swap' });

// Global SEO defaults and document shell. Locale-aware html attributes are
// computed from route params so RTL locales get a correct base direction.
export const metadata: Metadata = {
  metadataBase: new URL(siteUrl),
  title: {
    default: `${appName} — ${appTagline}`,
    template: `%s — ${appName}`,
  },
  description: appTagline,
  applicationName: appName,
  openGraph: {
    siteName: appName,
    type: 'website',
  },
  twitter: {
    card: 'summary_large_image',
  },
  icons: {
    icon: '/favicon.png',
    apple: '/icon.png',
  },
};

export default async function RootLayout({
  children,
  params,
}: {
  children: ReactNode;
  params: Promise<{ lang?: string }>;
}) {
  const { lang: rawLang } = await params;
  const lang = i18n.languages.includes(rawLang as (typeof i18n.languages)[number])
    ? (rawLang as (typeof i18n.languages)[number])
    : i18n.defaultLanguage;
  const dir = localeDirection(lang);
  const fontClassName = lang === 'fa' ? vazirmatn.className : inter.className;

  return (
    <html lang={lang} dir={dir} className={fontClassName} suppressHydrationWarning>
      <body className="flex min-h-screen flex-col" suppressHydrationWarning>
        {children}
      </body>
    </html>
  );
}
