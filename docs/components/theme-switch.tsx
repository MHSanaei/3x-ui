'use client';

import { Moon, Sun } from 'lucide-react';
import { useEffect, useState, useSyncExternalStore } from 'react';
import type { ComponentProps } from 'react';
import { cn } from '@/lib/cn';

type ThemeMode = 'light-dark' | 'light-dark-system';
type ThemePref = 'light' | 'dark' | 'system';

const STORAGE_KEY = 'docs-theme';

// `useSyncExternalStore` supplies the same value for SSR and hydration, then
// switches to the browser value after React has attached to the markup.
const subscribeToHydration = () => () => {};
const getHydrationClientSnapshot = () => true;
const getHydrationServerSnapshot = () => false;

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
  // Keep the server and first client render identical. Reading localStorage or
  // matchMedia here would make a persisted/system preference change the client
  // markup before React has finished hydrating it.
  const [selectedTheme, setSelectedTheme] = useState<ThemePref>('system');
  const hydrated = useSyncExternalStore(
    subscribeToHydration,
    getHydrationClientSnapshot,
    getHydrationServerSnapshot,
  );
  const theme = hydrated ? getStoredTheme() : selectedTheme;

  useEffect(() => {
    if (hydrated) applyTheme(theme);
  }, [hydrated, theme]);

  useEffect(() => {
    if (!hydrated) return;
    if (theme !== 'system') return;
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    const update = () => applyTheme('system');
    media.addEventListener('change', update);
    return () => media.removeEventListener('change', update);
  }, [hydrated, theme]);

  const resolved = hydrated ? getResolvedTheme(theme) : 'light';

  const setTheme = (nextTheme: ThemePref) => {
    window.localStorage.setItem(STORAGE_KEY, nextTheme);
    applyTheme(nextTheme);
    setSelectedTheme(nextTheme);
  };

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
