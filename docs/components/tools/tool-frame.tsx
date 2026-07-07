'use client';

import { RotateCcw } from 'lucide-react';
import type { ReactNode } from 'react';

export function ToolFrame({
  title,
  description,
  onReset,
  children,
}: {
  title: string;
  description?: string;
  onReset?: () => void;
  children: ReactNode;
}) {
  return (
    <section
      role="group"
      aria-label={title}
      className="not-prose my-6 overflow-hidden rounded-2xl border bg-fd-card text-fd-foreground"
    >
      <header className="flex items-start justify-between gap-3 border-b px-4 py-3">
        <div>
          <h3 className="font-semibold">{title}</h3>
          {description ? (
            <p className="mt-0.5 text-sm text-fd-muted-foreground">{description}</p>
          ) : null}
        </div>
        {onReset ? (
          <button
            type="button"
            onClick={onReset}
            className="inline-flex shrink-0 items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-xs font-medium transition-colors hover:bg-fd-accent hover:text-fd-accent-foreground"
          >
            <RotateCcw className="size-3.5" aria-hidden />
            Reset
          </button>
        ) : null}
      </header>
      <div className="p-4">{children}</div>
    </section>
  );
}
