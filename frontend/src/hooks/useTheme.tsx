import { createContext, useCallback, useContext, useLayoutEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { theme as antdTheme } from 'antd';
import type { ThemeConfig } from 'antd';

const STORAGE_DARK = 'dark-mode';
const STORAGE_ULTRA = 'isUltraDarkThemeEnabled';
const STORAGE_ACCENT = 'accentColor';

export const ACCENT_COLORS = [
  'default',
  'red',
  'pink',
  'orange',
  'yellow',
  'green',
  'purple',
] as const;
export type AccentColor = (typeof ACCENT_COLORS)[number];

// light: white text on it stays >= 4.5:1; dark: readable on dark cards, with
// dark text on the button where white text would not be (orange/yellow/green).
const ACCENT_SHADES: Record<
  Exclude<AccentColor, 'default'>,
  { light: string; dark: string; darkInk: boolean }
> = {
  red: { light: '#dc2626', dark: '#ef4444', darkInk: false },
  pink: { light: '#db2777', dark: '#ec4899', darkInk: false },
  orange: { light: '#c2410c', dark: '#fb923c', darkInk: true },
  yellow: { light: '#a16207', dark: '#facc15', darkInk: true },
  green: { light: '#047857', dark: '#34d399', darkInk: true },
  purple: { light: '#7c3aed', dark: '#8b5cf6', darkInk: false },
};
const DEFAULT_PRIMARY = { light: '#0958d9', dark: '#1677ff' };
const DARK_INK = '#141414';

export function accentPrimary(accent: AccentColor, isDark: boolean): string {
  const shades = accent === 'default' ? DEFAULT_PRIMARY : ACCENT_SHADES[accent];
  return isDark ? shades.dark : shades.light;
}

function readBool(key: string, fallback: boolean): boolean {
  const raw = localStorage.getItem(key);
  if (raw === null) return fallback;
  return raw === 'true';
}

function readAccent(): AccentColor {
  const raw = localStorage.getItem(STORAGE_ACCENT);
  return (ACCENT_COLORS as readonly string[]).includes(raw ?? '')
    ? (raw as AccentColor)
    : 'default';
}

function applyDom(isDark: boolean, isUltra: boolean) {
  document.body.classList.remove('dark', 'light');
  document.body.classList.add(isDark ? 'dark' : 'light');
  // Native scrollbars read color-scheme, not the body class.
  document.documentElement.style.colorScheme = isDark ? 'dark' : 'light';
  if (isUltra) {
    document.documentElement.setAttribute('data-theme', 'ultra-dark');
  } else {
    document.documentElement.removeAttribute('data-theme');
  }
  const msg = document.getElementById('message');
  if (msg) {
    msg.classList.remove('dark', 'light');
    msg.classList.add(isDark ? 'dark' : 'light');
  }
}

// module load so the document is in the right theme before React mounts.
const initialDark = readBool(STORAGE_DARK, true);
const initialUltra = readBool(STORAGE_ULTRA, false);
const initialAccent = readAccent();
applyDom(initialDark, initialUltra);

const DARK_TOKENS = {
  colorBgBase: '#1a1b1f',
  colorBgLayout: '#1a1b1f',
  colorBgContainer: '#23252b',
  colorBgElevated: '#2d2f37',
};
const ULTRA_DARK_TOKENS = {
  colorBgBase: '#000',
  colorBgLayout: '#000',
  colorBgContainer: '#101013',
  colorBgElevated: '#1a1a1e',
};
const DARK_LAYOUT_TOKENS = {
  bodyBg: '#1a1b1f',
  headerBg: '#15161a',
  headerColor: '#ffffff',
  footerBg: '#1a1b1f',
  siderBg: '#15161a',
  triggerBg: '#23252b',
  triggerColor: '#ffffff',
};
const ULTRA_DARK_LAYOUT_TOKENS = {
  bodyBg: '#000',
  headerBg: '#050507',
  headerColor: '#ffffff',
  footerBg: '#000',
  siderBg: '#050507',
  triggerBg: '#1a1a1e',
  triggerColor: '#ffffff',
};
const DARK_MENU_TOKENS = {
  darkItemBg: '#15161a',
  darkSubMenuItemBg: '#1a1b1f',
  darkPopupBg: '#23252b',
};
const ULTRA_DARK_MENU_TOKENS = {
  darkItemBg: '#050507',
  darkSubMenuItemBg: '#000',
  darkPopupBg: '#101013',
};
const DARK_CARD_TOKENS = {
  colorBorderSecondary: 'rgba(255, 255, 255, 0.06)',
};
const ULTRA_DARK_CARD_TOKENS = {
  colorBorderSecondary: 'rgba(255, 255, 255, 0.04)',
};
const STATISTIC_TOKENS = {
  contentFontSize: 17,
  titleFontSize: 11,
};
const LIGHT_CONTRAST_TOKENS = {
  colorTextDescription: 'rgba(0, 0, 0, 0.58)',
  colorTextTertiary: 'rgba(0, 0, 0, 0.58)',
  colorTextPlaceholder: '#767676',
  colorError: '#cf1322',
  colorErrorText: '#cf1322',
  colorSuccessText: '#237804',
};
const LIGHT_BUTTON_TOKENS = {
  colorPrimary: '#0958d9',
  colorPrimaryHover: '#2468e5',
  colorPrimaryActive: '#073ea8',
};

// hashed:false drops the `:where(.css-<hash>)` wrapper antd puts around every
// rule. It costs nothing in specificity — `:where()` contributes zero, so the
// panel's own `.ant-*` overrides still win — and it removes roughly 5,700
// wrappers, 16% of the generated stylesheet, from what the browser has to parse.
//
// cssVar.key pins the CSS-variable scope. Every panel page mounts its own
// ConfigProvider (there is no root one), and without a fixed key each mints a
// fresh useId-derived scope, so navigating re-serialises and re-injects the whole
// token block under a new class instead of reusing the one already in the head.
const SHARED_STYLE_CONFIG = {
  hashed: false,
  cssVar: { key: 'xui' },
} as const;

export function buildAntdThemeConfig(
  isDark: boolean,
  isUltra: boolean,
  accent: AccentColor = 'default',
): ThemeConfig {
  if (!isDark) {
    const accentToken = accent === 'default' ? {} : { colorPrimary: accentPrimary(accent, false) };
    return {
      ...SHARED_STYLE_CONFIG,
      algorithm: antdTheme.defaultAlgorithm,
      token: { ...LIGHT_CONTRAST_TOKENS, ...accentToken },
      components: {
        Statistic: STATISTIC_TOKENS,
        Button: accent === 'default' ? LIGHT_BUTTON_TOKENS : {},
      },
    };
  }
  const accentToken = accent === 'default' ? {} : { colorPrimary: accentPrimary(accent, true) };
  const darkInk = accent !== 'default' && ACCENT_SHADES[accent].darkInk;
  return {
    ...SHARED_STYLE_CONFIG,
    algorithm: antdTheme.darkAlgorithm,
    token: { ...(isUltra ? ULTRA_DARK_TOKENS : DARK_TOKENS), ...accentToken },
    components: {
      Layout: isUltra ? ULTRA_DARK_LAYOUT_TOKENS : DARK_LAYOUT_TOKENS,
      Menu: isUltra ? ULTRA_DARK_MENU_TOKENS : DARK_MENU_TOKENS,
      Card: isUltra ? ULTRA_DARK_CARD_TOKENS : DARK_CARD_TOKENS,
      Statistic: STATISTIC_TOKENS,
      ...(darkInk ? { Button: { primaryColor: DARK_INK } } : {}),
    },
  };
}

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

interface ThemeContextValue {
  isDark: boolean;
  isUltra: boolean;
  toggleTheme: () => void;
  toggleUltra: () => void;
  accent: AccentColor;
  setAccent: (accent: AccentColor) => void;
  antdThemeConfig: ThemeConfig;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [isDark, setIsDark] = useState<boolean>(initialDark);
  const [isUltra, setIsUltra] = useState<boolean>(initialUltra);
  const [accent, setAccent] = useState<AccentColor>(initialAccent);

  useLayoutEffect(() => {
    applyDom(isDark, isUltra);
    localStorage.setItem(STORAGE_DARK, String(isDark));
    localStorage.setItem(STORAGE_ULTRA, String(isUltra));
  }, [isDark, isUltra]);

  useLayoutEffect(() => {
    localStorage.setItem(STORAGE_ACCENT, accent);
  }, [accent]);

  const toggleTheme = useCallback(() => setIsDark((v) => !v), []);
  const toggleUltra = useCallback(() => setIsUltra((v) => !v), []);

  const antdThemeConfig = useMemo(
    () => buildAntdThemeConfig(isDark, isUltra, accent),
    [isDark, isUltra, accent],
  );

  const value = useMemo<ThemeContextValue>(
    () => ({ isDark, isUltra, toggleTheme, toggleUltra, accent, setAccent, antdThemeConfig }),
    [isDark, isUltra, toggleTheme, toggleUltra, accent, antdThemeConfig],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error('useTheme must be used inside <ThemeProvider>');
  return ctx;
}
