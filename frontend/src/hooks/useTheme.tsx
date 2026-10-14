import { createContext, useCallback, useContext, useLayoutEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import type { ThemeConfig } from 'antd';
import type { ThemeTokens } from '@/schemas/forms/theme';
import { BUILTIN_PRESETS } from '@/lib/themes/presets';
import {
  applyCSSVariables,
  buildAntdConfig,
  readActiveThemeId,
  readCustomThemes,
  resolveTheme,
  saveActiveThemeId,
  saveCustomThemes,
} from '@/lib/themes/engine';

export function pauseAnimationsUntilLeave(elementId: string): void {
  document.documentElement.setAttribute('data-theme-animations', 'off');
  const el = document.getElementById(elementId);
  if (!el) return;
  const restore = () => {
    document.documentElement.removeAttribute('data-theme-animations');
    el.removeEventListener('mouseleave', restore);
    el.removeEventListener('touchend', restore);
  };
  el.addEventListener('mouseleave', restore);
  el.addEventListener('touchend', restore);
}

// Check safe mode parameter (?theme=default-dark or ?theme=default)
function checkSafeMode(): string {
  if (typeof window === 'undefined') return readActiveThemeId();
  try {
    const params = new URLSearchParams(window.location.search);
    const themeParam = params.get('theme');
    if (themeParam === 'default' || themeParam === 'default-dark') {
      saveActiveThemeId('default-dark');
      return 'default-dark';
    }
    if (themeParam && BUILTIN_PRESETS[themeParam]) {
      saveActiveThemeId(themeParam);
      return themeParam;
    }
  } catch {}
  return readActiveThemeId();
}

interface ThemeContextValue {
  // Legacy backward-compatible API
  isDark: boolean;
  isUltra: boolean;
  toggleTheme: () => void;
  toggleUltra: () => void;
  antdThemeConfig: ThemeConfig;

  // New Theme Engine API
  activeTheme: ThemeTokens;
  setThemeId: (id: string) => void;
  setCustomTheme: (theme: ThemeTokens) => void;
  resetToDefault: () => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

// Apply DOM attributes on module load so initial render has correct background
const initialThemeId = checkSafeMode();
const initialCustomThemes = typeof window !== 'undefined' ? readCustomThemes() : [];
const initialTheme = resolveTheme(initialThemeId, initialCustomThemes);
applyCSSVariables(initialTheme);

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [activeId, setActiveId] = useState<string>(initialThemeId);
  const [customThemes, setCustomThemes] = useState<ThemeTokens[]>(initialCustomThemes);

  const activeTheme = useMemo<ThemeTokens>(
    () => resolveTheme(activeId, customThemes),
    [activeId, customThemes],
  );

  const isDark = activeTheme.baseMode === 'dark';
  const isUltra = activeTheme.id === 'ultra-dark';

  useLayoutEffect(() => {
    applyCSSVariables(activeTheme);
    saveActiveThemeId(activeTheme.id);
  }, [activeTheme]);

  const setThemeId = useCallback((id: string) => {
    setActiveId(id);
    saveActiveThemeId(id);
  }, []);

  const setCustomTheme = useCallback((theme: ThemeTokens) => {
    setCustomThemes((prev) => {
      const filtered = prev.filter((t) => t.id !== theme.id);
      const next = [...filtered, theme];
      saveCustomThemes(next);
      return next;
    });
    setActiveId(theme.id);
    saveActiveThemeId(theme.id);
  }, []);

  const resetToDefault = useCallback(() => {
    setActiveId('default-dark');
    saveActiveThemeId('default-dark');
  }, []);

  const toggleTheme = useCallback(() => {
    setActiveId((currentId) => {
      const current = resolveTheme(currentId, customThemes);
      return current.baseMode === 'dark' ? 'default-light' : 'default-dark';
    });
  }, [customThemes]);

  const toggleUltra = useCallback(() => {
    setActiveId((currentId) => {
      return currentId === 'ultra-dark' ? 'default-dark' : 'ultra-dark';
    });
  }, []);

  const antdThemeConfig = useMemo<ThemeConfig>(() => buildAntdConfig(activeTheme), [activeTheme]);

  const value = useMemo<ThemeContextValue>(
    () => ({
      isDark,
      isUltra,
      toggleTheme,
      toggleUltra,
      antdThemeConfig,
      activeTheme,
      setThemeId,
      setCustomTheme,
      resetToDefault,
    }),
    [
      isDark,
      isUltra,
      toggleTheme,
      toggleUltra,
      antdThemeConfig,
      activeTheme,
      setThemeId,
      setCustomTheme,
      resetToDefault,
    ],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error('useTheme must be used inside <ThemeProvider>');
  return ctx;
}

// Backward compatibility for Storybook and any legacy callers
export function buildAntdThemeConfig(isDark: boolean, isUltra: boolean): ThemeConfig {
  const id = !isDark ? 'default-light' : isUltra ? 'ultra-dark' : 'default-dark';
  return buildAntdConfig(resolveTheme(id));
}
