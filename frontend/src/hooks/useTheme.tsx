import {
  createContext,
  useCallback,
  useContext,
  useLayoutEffect,
  useMemo,
  useState,
  useSyncExternalStore,
} from 'react';
import type { ReactNode } from 'react';
import { theme as antdTheme } from 'antd';
import type { ThemeConfig } from 'antd';

export type ThemeMode = 'light' | 'dark' | 'system';

const STORAGE_MODE = 'theme-mode';
// Written alongside `theme-mode` because older panel builds and third-party
// scripts still read the boolean.
const STORAGE_DARK = 'dark-mode';
const STORAGE_ULTRA = 'isUltraDarkThemeEnabled';
const SCHEME_QUERY = '(prefers-color-scheme: dark)';

function readBool(key: string, fallback: boolean): boolean {
  const raw = localStorage.getItem(key);
  if (raw === null) return fallback;
  return raw === 'true';
}

function systemPrefersDark(): boolean {
  try {
    return window.matchMedia(SCHEME_QUERY).matches;
  } catch {
    return false;
  }
}

function subscribeSystemScheme(onChange: () => void): () => void {
  const mql = window.matchMedia(SCHEME_QUERY);
  mql.addEventListener('change', onChange);
  return () => mql.removeEventListener('change', onChange);
}

/** Reads a stored preference, falling back to the legacy boolean panels wrote. */
function parseStoredMode(raw: string | null, legacyDark: boolean): ThemeMode {
  if (raw === 'light' || raw === 'dark' || raw === 'system') return raw;
  return legacyDark ? 'dark' : 'light';
}

function readMode(): ThemeMode {
  try {
    return parseStoredMode(localStorage.getItem(STORAGE_MODE), readBool(STORAGE_DARK, true));
  } catch {
    return 'dark';
  }
}

function readUltra(): boolean {
  try {
    return readBool(STORAGE_ULTRA, false);
  } catch {
    return false;
  }
}

function resolveDark(mode: ThemeMode, systemDark: boolean): boolean {
  return mode === 'system' ? systemDark : mode === 'dark';
}

/**
 * light → dark → ultra dark → system → light. Ultra dark only exists on top of
 * dark, so every step is visible even when the OS scheme matches the current
 * one: leaving system always lands on light.
 */
function nextTheme(mode: ThemeMode, isUltra: boolean): { mode: ThemeMode; ultra: boolean } {
  if (mode === 'light') return { mode: 'dark', ultra: false };
  if (mode === 'dark') {
    return isUltra ? { mode: 'system', ultra: false } : { mode: 'dark', ultra: true };
  }
  return { mode: 'light', ultra: false };
}

/** i18n key naming the current mode, used for the toggle's label and tooltip. */
export function themeModeKey(mode: ThemeMode, isUltra: boolean): string {
  if (mode === 'system') return 'menu.themeSystem';
  if (mode === 'light') return 'menu.themeLight';
  return isUltra ? 'menu.themeUltraDark' : 'menu.themeDark';
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
const initialMode = readMode();
const initialUltra = readUltra();
const initialDark = resolveDark(initialMode, systemPrefersDark());
applyDom(initialDark, initialUltra && initialDark);

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

export function buildAntdThemeConfig(isDark: boolean, isUltra: boolean): ThemeConfig {
  if (!isDark) {
    return {
      ...SHARED_STYLE_CONFIG,
      algorithm: antdTheme.defaultAlgorithm,
      token: LIGHT_CONTRAST_TOKENS,
      components: {
        Statistic: STATISTIC_TOKENS,
        Button: LIGHT_BUTTON_TOKENS,
      },
    };
  }
  return {
    ...SHARED_STYLE_CONFIG,
    algorithm: antdTheme.darkAlgorithm,
    token: isUltra ? ULTRA_DARK_TOKENS : DARK_TOKENS,
    components: {
      Layout: isUltra ? ULTRA_DARK_LAYOUT_TOKENS : DARK_LAYOUT_TOKENS,
      Menu: isUltra ? ULTRA_DARK_MENU_TOKENS : DARK_MENU_TOKENS,
      Card: isUltra ? ULTRA_DARK_CARD_TOKENS : DARK_CARD_TOKENS,
      Statistic: STATISTIC_TOKENS,
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
  /** The stored preference: light, dark, or follow the OS. */
  mode: ThemeMode;
  /** Resolved appearance — `system` collapsed against the OS scheme. */
  isDark: boolean;
  /** Ultra dark is a variant of dark and never applies to light. */
  isUltra: boolean;
  /** Whether the OS currently asks for a dark scheme. */
  systemDark: boolean;
  toggleTheme: () => void;
  toggleUltra: () => void;
  setMode: (mode: ThemeMode) => void;
  /** Walks light → dark → ultra dark → system → light. */
  cycleTheme: () => void;
  antdThemeConfig: ThemeConfig;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

export function ThemeProvider({ children }: { children: ReactNode }) {
  // Read at mount rather than reusing the module-level snapshot so a stored
  // preference written by another tab (or by a test) is honoured.
  const [mode, setModeState] = useState<ThemeMode>(readMode);
  const [ultraRequested, setUltraRequested] = useState<boolean>(readUltra);
  // Tracks the OS scheme live so `system` mode follows the user flipping their
  // desktop appearance without a reload.
  const systemDark = useSyncExternalStore(subscribeSystemScheme, systemPrefersDark, () => false);

  const isDark = resolveDark(mode, systemDark);
  const isUltra = ultraRequested && isDark;

  useLayoutEffect(() => {
    applyDom(isDark, isUltra);
    localStorage.setItem(STORAGE_MODE, mode);
    localStorage.setItem(STORAGE_DARK, String(isDark));
    localStorage.setItem(STORAGE_ULTRA, String(isUltra));
  }, [mode, isDark, isUltra]);

  const toggleTheme = useCallback(() => setModeState(isDark ? 'light' : 'dark'), [isDark]);
  const toggleUltra = useCallback(() => setUltraRequested((v) => !v), []);
  const setMode = useCallback((next: ThemeMode) => setModeState(next), []);
  const cycleTheme = useCallback(() => {
    const next = nextTheme(mode, isUltra);
    setModeState(next.mode);
    setUltraRequested(next.ultra);
  }, [mode, isUltra]);

  const antdThemeConfig = useMemo(() => buildAntdThemeConfig(isDark, isUltra), [isDark, isUltra]);

  const value = useMemo<ThemeContextValue>(
    () => ({
      mode,
      isDark,
      isUltra,
      systemDark,
      toggleTheme,
      toggleUltra,
      setMode,
      cycleTheme,
      antdThemeConfig,
    }),
    [
      mode,
      isDark,
      isUltra,
      systemDark,
      toggleTheme,
      toggleUltra,
      setMode,
      cycleTheme,
      antdThemeConfig,
    ],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error('useTheme must be used inside <ThemeProvider>');
  return ctx;
}
