'use client';

import { useState } from 'react';
import { Check, Copy } from 'lucide-react';
import { cn } from '@/lib/cn';

export function InstallCommand({
  command,
  className,
  copyLabel = 'Copy install command',
  copiedLabel = 'Copied',
}: {
  command: string;
  className?: string;
  copyLabel?: string;
  copiedLabel?: string;
}) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard unavailable (insecure context) — silently ignore.
    }
  }

  return (
    <div
      className={cn(
        'flex items-center gap-3 rounded-xl border bg-fd-card py-2.5 pe-2 ps-4 text-sm shadow-sm',
        className,
      )}
    >
      <span className="select-none font-mono text-fd-muted-foreground">$</span>
      {/* Commands are always LTR, even on RTL pages. */}
      <code dir="ltr" className="flex-1 overflow-x-auto whitespace-nowrap text-start font-mono">
        {command}
      </code>
      <button
        type="button"
        onClick={copy}
        aria-label={copied ? copiedLabel : copyLabel}
        className="inline-flex size-8 shrink-0 items-center justify-center rounded-lg text-fd-muted-foreground transition-colors hover:bg-fd-accent hover:text-fd-accent-foreground focus-visible:outline-2 focus-visible:outline-fd-ring"
      >
        {copied ? (
          <Check className="size-4 text-brand" aria-hidden />
        ) : (
          <Copy className="size-4" aria-hidden />
        )}
      </button>
    </div>
  );
}
