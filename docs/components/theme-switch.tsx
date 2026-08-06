'use client';

import { Moon, Sun } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import type { ComponentProps } from 'react';
import { cn } from '@/lib/cn';

type ThemeMode = 'light-dark' | 'light-dark-system';
type ThemePref = 'light' | 'dark' | 'system';

const STORAGE_KEY = 'docs-theme';

function getStoredTheme(): ThemePref {
  if (typeof window === 'undefined') return 'system';
  const raw = window.localStorage.getItem(STORAGE_KEY);
  return raw === 'light' || raw === 'dark' || raw === 'system' ? raw : 'system';
}

function getResolvedTheme(theme: ThemePref): 'light' | 'dark' {
  if (theme !== 'system') return theme;
  if (typeof window === 'undefined') return 'light';
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function applyTheme(theme: ThemePref): void {
  if (typeof document === 'undefined') return;
  const resolved = getResolvedTheme(theme);
  const root = document.documentElement;
  root.classList.toggle('dark', resolved === 'dark');
  root.style.colorScheme = resolved;
}

export function DocsThemeSwitch({
  className,
  mode = 'light-dark-system',
  ...props
}: {
  className?: string;
  mode?: ThemeMode;
} & Omit<ComponentProps<'div'>, 'children'>) {
  const [theme, setTheme] = useState<ThemePref>(getStoredTheme);

  useEffect(() => {
    window.localStorage.setItem(STORAGE_KEY, theme);
    applyTheme(theme);
  }, [theme]);

  useEffect(() => {
    if (theme !== 'system') return;
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    const update = () => applyTheme('system');
    media.addEventListener('change', update);
    return () => media.removeEventListener('change', update);
  }, [theme]);

  const resolved = useMemo(() => getResolvedTheme(theme), [theme]);

  const nextTheme = () => {
    if (mode === 'light-dark') return resolved === 'dark' ? 'light' : 'dark';
    if (theme === 'light') return 'dark';
    if (theme === 'dark') return 'system';
    return resolved === 'dark' ? 'light' : 'dark';
  };

  const label =
    mode === 'light-dark-system'
      ? `Switch theme (current: ${theme})`
      : `Switch to ${resolved === 'dark' ? 'light' : 'dark'} mode`;

  return (
    <div className={cn('inline-flex', className)} {...props}>
      <button
        type="button"
        aria-label={label}
        title={label}
        onClick={() => setTheme(nextTheme())}
        className="inline-flex size-8 items-center justify-center rounded-lg text-fd-muted-foreground transition-colors hover:bg-fd-accent hover:text-fd-accent-foreground"
      >
        {resolved === 'dark' ? <Moon className="size-4" /> : <Sun className="size-4" />}
      </button>
    </div>
  );
}