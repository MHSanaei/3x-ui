import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { appName, appTagline, siteUrl } from '@/lib/shared';

// Global SEO defaults. The real <html>/<body> live in `app/[lang]/layout.tsx`
// so we can set `lang`/`dir` per locale (RTL for fa); this root layout is a
// pass-through that only carries site-wide metadata.
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

export default function RootLayout({ children }: { children: ReactNode }) {
  return children;
}
