import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';
import { Heart } from 'lucide-react';
import { Logo } from '@/components/logo';
import { TelegramIcon } from '@/components/icons';
import { appName, productRepoUrl, telegramChannel, telegramChannelUrl, donateUrl } from './shared';
import { getSiteMessages } from './site-i18n';

// Build locale-aware shared layout options. With `hideLocale: 'default-locale'`,
// English URLs have no prefix while other locales are prefixed (`/fa`, `/ru`, `/zh`).
export function baseOptions(lang: string): BaseLayoutProps {
  const prefix = lang === 'en' ? '' : `/${lang}`;
  const m = getSiteMessages(lang);

  return {
    nav: {
      title: (
        <span className="inline-flex items-center gap-2 font-semibold">
          <Logo className="h-6" />
          {appName}
        </span>
      ),
      url: `${prefix}/`,
    },
    githubUrl: productRepoUrl,
    links: [
      {
        text: m.documentation,
        url: `${prefix}/docs`,
        active: 'nested-url',
      },
      {
        type: 'icon',
        label: `Telegram channel (@${telegramChannel})`,
        icon: <TelegramIcon />,
        text: 'Telegram',
        url: telegramChannelUrl,
        external: true,
      },
      // Compact heart icon in the top nav bars (home + docs).
      {
        type: 'icon',
        on: 'nav',
        label: m.donate,
        icon: <Heart />,
        text: m.donate,
        url: donateUrl,
        external: true,
      },
      // Prominent labelled entry in the docs sidebar / mobile menu.
      {
        type: 'main',
        on: 'menu',
        icon: <Heart />,
        text: m.donate,
        url: donateUrl,
        external: true,
      },
    ],
  };
}
