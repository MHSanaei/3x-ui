import { RootProvider } from 'fumadocs-ui/provider/next';
import { i18n } from '@/lib/i18n';
import { provider } from '@/lib/i18n-ui';
import SearchDialog from '@/components/search-dialog';

export function generateStaticParams() {
  return i18n.languages.map((lang) => ({ lang }));
}

export default async function LangLayout({ params, children }: LayoutProps<'/[lang]'>) {
  const { lang } = await params;

  return (
    <RootProvider i18n={provider(lang)} search={{ SearchDialog }} theme={{ enabled: false }}>
      {children}
    </RootProvider>
  );
}
