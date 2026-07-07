import {
  Boxes,
  Network,
  Send,
  ShieldCheck,
  TerminalSquare,
  Users,
  type LucideIcon,
} from 'lucide-react';

// Icons map by position to the localized feature items in lib/site-i18n.ts
// (Every major protocol, REALITY, Clients, Multi-node, Telegram, Self-hosted).
const ICONS: LucideIcon[] = [Boxes, ShieldCheck, Users, Network, Send, TerminalSquare];

export function Features({
  heading,
  subtitle,
  items,
}: {
  heading: string;
  subtitle: string;
  items: { title: string; description: string }[];
}) {
  return (
    <section className="mx-auto w-full max-w-6xl px-4 py-16 sm:py-24">
      <div className="mx-auto max-w-2xl text-center">
        <h2 className="text-2xl font-bold tracking-tight sm:text-3xl">{heading}</h2>
        <p className="mt-3 text-fd-muted-foreground">{subtitle}</p>
      </div>
      <div className="mt-12 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {items.map(({ title, description }, i) => {
          const Icon = ICONS[i] ?? Boxes;
          return (
            <div
              key={title}
              className="rounded-2xl border bg-fd-card p-6 transition-colors hover:border-fd-primary/40"
            >
              <div className="inline-flex size-11 items-center justify-center rounded-xl bg-brand/10 text-brand">
                <Icon className="size-6" aria-hidden />
              </div>
              <h3 className="mt-4 font-semibold">{title}</h3>
              <p className="mt-2 text-sm text-fd-muted-foreground">{description}</p>
            </div>
          );
        })}
      </div>
    </section>
  );
}
