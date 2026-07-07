import '../global.css';
import { RootProvider } from 'fumadocs-ui/provider/next';
import { Inter, Vazirmatn } from 'next/font/google';
import { i18n, localeDirection } from '@/lib/i18n';
import { provider } from '@/lib/i18n-ui';
import SearchDialog from '@/components/search-dialog';

const inter = Inter({ subsets: ['latin'], display: 'swap' });
// Persian UI font; covers Arabic + Latin glyphs so mixed content renders well.
const vazirmatn = Vazirmatn({ subsets: ['arabic'], display: 'swap' });

export function generateStaticParams() {
  return i18n.languages.map((lang) => ({ lang }));
}

export default async function LangLayout({ params, children }: LayoutProps<'/[lang]'>) {
  const { lang } = await params;
  const dir = localeDirection(lang);
  const fontClassName = lang === 'fa' ? vazirmatn.className : inter.className;

  return (
    <html lang={lang} dir={dir} className={fontClassName} suppressHydrationWarning>
      <body className="flex min-h-screen flex-col" suppressHydrationWarning>
        <RootProvider i18n={provider(lang)} search={{ SearchDialog }}>
          {children}
        </RootProvider>
      </body>
    </html>
  );
}
