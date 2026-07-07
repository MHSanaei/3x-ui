import { HomeLayout } from 'fumadocs-ui/layouts/home';
import { baseOptions } from '@/lib/layout.shared';
import { HomeLanguageSwitcher } from '@/components/home/language-switcher';

export default async function Layout({ params, children }: LayoutProps<'/[lang]'>) {
  const { lang } = await params;
  const options = baseOptions(lang);
  return (
    <HomeLayout
      {...options}
      // Disable fumadocs' built-in popover language switcher here: nested in the
      // home navbar's Radix NavigationMenu its item clicks don't fire. We inject
      // an anchor-based one instead (see HomeLanguageSwitcher). Docs keep the
      // built-in switcher (its sidebar isn't a NavigationMenu, so it works).
      i18n={false}
      links={[
        ...(options.links ?? []),
        {
          type: 'custom',
          secondary: true,
          children: <HomeLanguageSwitcher current={lang} />,
        },
      ]}
    >
      {children}
    </HomeLayout>
  );
}
