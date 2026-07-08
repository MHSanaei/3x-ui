import Link from 'next/link';
import { ArrowRight, BookOpen, Heart } from 'lucide-react';
import { GitHubIcon, TelegramIcon } from '@/components/icons';
import { Logo } from '@/components/logo';
import { Features } from '@/components/home/features';
import { GitHubStatsRow } from '@/components/home/github-stats';
import { InstallCommand } from '@/components/home/install-command';
import { getGitHubStats } from '@/lib/github-stats';
import { i18n } from '@/lib/i18n';
import { appName, productRepoUrl, deepWikiUrl, telegramChannelUrl, donateUrl } from '@/lib/shared';
import { getSiteMessages, type SiteMessages } from '@/lib/site-i18n';

export function generateStaticParams() {
  return i18n.languages.map((lang) => ({ lang }));
}

const INSTALL_COMMAND =
  'bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)';

export default async function HomePage({ params }: PageProps<'/[lang]'>) {
  const { lang } = await params;
  const prefix = lang === 'en' ? '' : `/${lang}`;
  const m = getSiteMessages(lang);
  const stats = await getGitHubStats();

  return (
    <main className="flex flex-1 flex-col">
      {/* Hero */}
      <section className="relative overflow-hidden border-b">
        <div
          className="pointer-events-none absolute inset-x-0 -top-40 h-80 bg-gradient-to-b from-brand/15 to-transparent blur-3xl"
          aria-hidden
        />
        <div className="mx-auto flex w-full max-w-5xl flex-col items-center px-4 py-20 text-center sm:py-28">
          <Logo className="h-20 drop-shadow-sm" />
          <h1 className="mt-6 text-4xl font-bold tracking-tight sm:text-6xl">
            <span className="bg-gradient-to-r from-cyan-500 to-sky-600 bg-clip-text text-transparent dark:from-cyan-300 dark:to-sky-400">
              {appName}
            </span>
          </h1>
          <p className="mt-4 max-w-2xl text-lg text-fd-muted-foreground sm:text-xl">{m.tagline}</p>

          <div className="mt-8 flex flex-col items-center gap-3 sm:flex-row">
            <Link
              href={`${prefix}/docs`}
              className="inline-flex items-center gap-2 rounded-xl bg-fd-primary px-5 py-3 font-medium text-fd-primary-foreground transition-opacity hover:opacity-90"
            >
              {m.getStarted}
              <ArrowRight className="size-4 rtl:rotate-180" aria-hidden />
            </Link>
            <a
              href={productRepoUrl}
              target="_blank"
              rel="noreferrer noopener"
              className="inline-flex items-center gap-2 rounded-xl border px-5 py-3 font-medium transition-colors hover:bg-fd-accent hover:text-fd-accent-foreground"
            >
              <GitHubIcon className="size-4" />
              {m.viewOnGitHub}
            </a>
          </div>

          <InstallCommand
            command={INSTALL_COMMAND}
            copyLabel={m.copyCommand}
            copiedLabel={m.copied}
            className="mt-8 w-full max-w-2xl"
          />

          {/* Build-time stats as the initial render; refreshed live on the client. */}
          <GitHubStatsRow
            initial={stats}
            labels={{ stars: m.stars, forks: m.forks, latest: m.latest }}
          />
        </div>
      </section>

      <Features heading={m.featuresHeading} subtitle={m.featuresSubtitle} items={m.features} />

      <Footer prefix={prefix} m={m} />
    </main>
  );
}

function Footer({ prefix, m }: { prefix: string; m: SiteMessages }) {
  return (
    <footer className="border-t">
      <div className="mx-auto flex w-full max-w-6xl flex-col items-center justify-between gap-4 px-4 py-8 text-sm text-fd-muted-foreground sm:flex-row">
        <div className="inline-flex items-center gap-2">
          <Logo className="h-6" />
          <span>
            {appName} — {m.licenseBefore}
            <a
              href={`${productRepoUrl}/blob/main/LICENSE`}
              className="underline hover:text-fd-foreground"
            >
              GPL-3.0
            </a>
            {m.licenseAfter}
          </span>
        </div>
        <nav className="flex items-center gap-4">
          <Link href={`${prefix}/docs`} className="hover:text-fd-foreground">
            {m.docs}
          </Link>
          <a
            href={productRepoUrl}
            className="inline-flex items-center gap-1.5 hover:text-fd-foreground"
          >
            <GitHubIcon className="size-4" />
            GitHub
          </a>
          <a
            href={deepWikiUrl}
            target="_blank"
            rel="noreferrer noopener"
            className="inline-flex items-center gap-1.5 hover:text-fd-foreground"
          >
            <BookOpen className="size-4" />
            DeepWiki
          </a>
          <a
            href={telegramChannelUrl}
            target="_blank"
            rel="noreferrer noopener"
            className="inline-flex items-center gap-1.5 hover:text-fd-foreground"
          >
            <TelegramIcon className="size-4" />
            Telegram
          </a>
          <a
            href={donateUrl}
            target="_blank"
            rel="noreferrer noopener"
            className="inline-flex items-center gap-1.5 hover:text-fd-foreground"
          >
            <Heart className="size-4" />
            {m.donate}
          </a>
        </nav>
      </div>
    </footer>
  );
}
