import { createContext, useCallback, useContext, useLayoutEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { theme as antdTheme } from 'antd';
import type { ThemeConfig } from 'antd';

const STORAGE_DARK = 'dark-mode';
const STORAGE_ULTRA = 'isUltraDarkThemeEnabled';
const STORAGE_THEME = 'xui-theme';

export type ThemeMode = 'light' | 'dark' | 'ultra-dark' | 'colorful' | 'blue-gray';

function readBool(key: string, fallback: boolean): boolean {
  const raw = localStorage.getItem(key);
  if (raw === null) return fallback;
  return raw === 'true';
}

function readThemeMode(): ThemeMode {
  const saved = localStorage.getItem(STORAGE_THEME);
  if (saved === 'light' || saved === 'dark' || saved === 'ultra-dark' || saved === 'colorful' || saved === 'blue-gray') return saved;
  if (readBool(STORAGE_ULTRA, false)) return 'ultra-dark';
  return readBool(STORAGE_DARK, true) ? 'dark' : 'light';
}

function applyDom(mode: ThemeMode) {
  const isDark = mode === 'dark' || mode === 'ultra-dark' || mode === 'blue-gray';
  document.body.classList.remove('dark', 'light', 'theme-ultra-dark', 'theme-colorful', 'theme-blue-gray');
  document.body.classList.add(isDark ? 'dark' : 'light', 'theme-' + mode);
  document.documentElement.style.colorScheme = isDark ? 'dark' : 'light';
  document.documentElement.setAttribute('data-theme', mode);
  const msg = document.getElementById('message');
  if (msg) {
    msg.classList.remove('dark', 'light');
    msg.classList.add(isDark ? 'dark' : 'light');
  }
}

const initialMode = readThemeMode();
applyDom(initialMode);

const ULTRA_DARK_TOKENS = {
  colorBgBase: '#000000',
  colorBgLayout: '#000000',
  colorBgContainer: '#08090c',
  colorBgElevated: '#111318',
};
const ULTRA_DARK_LAYOUT_TOKENS = {
  bodyBg: '#000000',
  headerBg: '#050507',
  headerColor: '#ffffff',
  footerBg: '#000000',
  siderBg: '#050507',
  triggerBg: '#1a1a1e',
  triggerColor: '#ffffff',
};
const ULTRA_DARK_MENU_TOKENS = {
  darkItemBg: '#050507',
  darkSubMenuItemBg: '#0a0b0e',
  darkPopupBg: '#111318',
};
const DARK_TOKENS = {
  colorBgBase: '#1a1b1f',
  colorBgLayout: '#1a1b1f',
  colorBgContainer: '#23252b',
  colorBgElevated: '#2d2f37',
};
const BLUE_GRAY_TOKENS = {
  colorBgBase: '#101722',
  colorBgLayout: '#101722',
  colorBgContainer: '#182333',
  colorBgElevated: '#223147',
};
const COLORFUL_TOKENS = {
  colorBgBase: '#fbf8ff',
  colorBgLayout: '#f8f5ff',
  colorBgContainer: '#fffaff',
  colorBgElevated: '#f1ecff',
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
const BLUE_GRAY_LAYOUT_TOKENS = {
  bodyBg: '#101722',
  headerBg: '#0c131d',
  headerColor: '#f2f6fb',
  footerBg: '#101722',
  siderBg: '#0c131d',
  triggerBg: '#182333',
  triggerColor: '#f2f6fb',
};
const DARK_MENU_TOKENS = {
  darkItemBg: '#15161a',
  darkSubMenuItemBg: '#1a1b1f',
  darkPopupBg: '#23252b',
};
const BLUE_GRAY_MENU_TOKENS = {
  darkItemBg: '#0c131d',
  darkSubMenuItemBg: '#101722',
  darkPopupBg: '#182333',
};
const DARK_CARD_TOKENS = {
  colorBorderSecondary: 'rgba(255, 255, 255, 0.06)',
};
const BLUE_GRAY_CARD_TOKENS = {
  colorBorderSecondary: 'rgba(190, 210, 235, 0.14)',
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
const MATERIAL_TOKENS = {
  colorPrimary: '#6750a4',
  colorPrimaryHover: '#7f67be',
  colorPrimaryActive: '#4f378b',
  colorLink: '#6750a4',
  borderRadius: 12,
  borderRadiusLG: 16,
  controlHeight: 40,
  fontFamily: 'Roboto, Inter, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
};

const SHARED_STYLE_CONFIG = {
  hashed: false,
  cssVar: { key: 'xui' },
} as const;

export function buildAntdThemeConfig(mode: ThemeMode): ThemeConfig {
  if (mode === 'ultra-dark') {
    return {
      ...SHARED_STYLE_CONFIG,
      algorithm: antdTheme.darkAlgorithm,
      token: { ...ULTRA_DARK_TOKENS, ...MATERIAL_TOKENS },
      components: {
        Layout: ULTRA_DARK_LAYOUT_TOKENS,
        Menu: ULTRA_DARK_MENU_TOKENS,
        Statistic: STATISTIC_TOKENS,
      },
    };
  }
  if (mode === 'colorful') {
    return {
      ...SHARED_STYLE_CONFIG,
      algorithm: antdTheme.defaultAlgorithm,
      token: {
        ...COLORFUL_TOKENS,
        ...MATERIAL_TOKENS,
        colorPrimary: '#7c3aed',
        colorPrimaryHover: '#8b5cf6',
        colorPrimaryActive: '#6d28d9',
        colorLink: '#7c3aed',
      },
      components: { Statistic: STATISTIC_TOKENS },
    };
  }
  if (mode === 'blue-gray') {
    return {
      ...SHARED_STYLE_CONFIG,
      algorithm: antdTheme.darkAlgorithm,
      token: {
        ...BLUE_GRAY_TOKENS,
        ...MATERIAL_TOKENS,
        colorPrimary: '#8ab4f8',
        colorPrimaryHover: '#a8c7fa',
        colorPrimaryActive: '#6ea0e8',
        colorLink: '#8ab4f8',
      },
      components: {
        Layout: BLUE_GRAY_LAYOUT_TOKENS,
        Menu: BLUE_GRAY_MENU_TOKENS,
        Card: BLUE_GRAY_CARD_TOKENS,
        Statistic: STATISTIC_TOKENS,
      },
    };
  }
  if (mode === 'light') {
    return {
      ...SHARED_STYLE_CONFIG,
      algorithm: antdTheme.defaultAlgorithm,
      token: { ...LIGHT_CONTRAST_TOKENS, ...MATERIAL_TOKENS },
      components: { Statistic: STATISTIC_TOKENS },
    };
  }
  return {
    ...SHARED_STYLE_CONFIG,
    algorithm: antdTheme.darkAlgorithm,
    token: { ...DARK_TOKENS, ...MATERIAL_TOKENS },
    components: {
      Layout: DARK_LAYOUT_TOKENS,
      Menu: DARK_MENU_TOKENS,
      Card: DARK_CARD_TOKENS,
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
  mode: ThemeMode;
  isDark: boolean;
  isUltra: boolean;
  toggleTheme: () => void;
  toggleUltra: () => void;
  setThemeMode: (mode: ThemeMode) => void;
  antdThemeConfig: ThemeConfig;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [mode, setMode] = useState<ThemeMode>(initialMode);
  const isDark = mode === 'dark' || mode === 'ultra-dark' || mode === 'blue-gray';
  const isUltra = mode === 'ultra-dark';

  useLayoutEffect(() => {
    applyDom(mode);
    localStorage.setItem(STORAGE_THEME, mode);
    localStorage.setItem(STORAGE_DARK, String(isDark));
    localStorage.setItem(STORAGE_ULTRA, String(isUltra));
  }, [mode, isDark]);

  const toggleTheme = useCallback(() => setMode((v) => (v === 'light' ? 'dark' : v === 'dark' || v === 'ultra-dark' ? 'light' : v)), []);
  const toggleUltra = useCallback(() => setMode((v) => (v === 'dark' ? 'ultra-dark' : v === 'ultra-dark' ? 'dark' : v)), []);
  const setThemeMode = useCallback((next: ThemeMode) => setMode(next), []);

  const antdThemeConfig = useMemo(() => buildAntdThemeConfig(mode), [mode]);

  const value = useMemo<ThemeContextValue>(
    () => ({ mode, isDark, isUltra, toggleTheme, toggleUltra, setThemeMode, antdThemeConfig }),
    [mode, isDark, isUltra, toggleTheme, toggleUltra, setThemeMode, antdThemeConfig],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error('useTheme must be used inside <ThemeProvider>');
  return ctx;
}
