import Link from 'next/link';
import { Languages } from 'lucide-react';
import { i18n, locales } from '@/lib/i18n';
import { cn } from '@/lib/cn';

// Home-navbar language switcher.
//
// fumadocs' built-in popover switcher (`LanguageSelect`) has its item clicks
// swallowed when it is nested inside HomeLayout's Radix `NavigationMenu` — the
// dropdown opens but selecting a locale never fires `onChange`/`router.push`.
// The docs sidebar isn't wrapped in a NavigationMenu, so the built-in one works
// there and is kept. Here we use a native `<details>` toggle + real `<Link>`
// anchors, which navigate reliably inside the navbar (like the other nav links).
//
// The home navbar only renders on the landing page, so the targets are simply
// each locale's home (`/`, `/fa`, `/ru`, `/zh`).
export function HomeLanguageSwitcher({ current }: { current: string }) {
  return (
    <details className="group relative [&>summary::-webkit-details-marker]:hidden">
      <summary
        aria-label="Choose a language"
        className="flex cursor-pointer list-none items-center rounded-lg p-1.5 text-fd-muted-foreground transition-colors hover:bg-fd-accent hover:text-fd-accent-foreground group-open:bg-fd-accent"
      >
        <Languages className="size-5" />
      </summary>
      <div className="absolute end-0 z-50 mt-1.5 flex min-w-40 flex-col gap-0.5 rounded-lg border bg-fd-popover p-1 text-fd-popover-foreground shadow-lg">
        <p className="p-2 text-xs font-medium text-fd-muted-foreground">Choose a language</p>
        {locales.map(({ locale, name }) => (
          <Link
            key={locale}
            href={locale === i18n.defaultLanguage ? '/' : `/${locale}`}
            className={cn(
              'rounded-md px-2 py-1.5 text-start text-sm transition-colors',
              locale === current
                ? 'bg-fd-primary/10 text-fd-primary'
                : 'text-fd-muted-foreground hover:bg-fd-accent hover:text-fd-accent-foreground',
            )}
          >
            {name}
          </Link>
        ))}
      </div>
    </details>
  );
}
