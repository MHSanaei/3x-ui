import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { theme as antdTheme } from 'antd';
import type { ThemeConfig } from 'antd';

const STORAGE_DARK = 'dark-mode';
const STORAGE_ULTRA = 'isUltraDarkThemeEnabled';

function readBool(key: string, fallback: boolean): boolean {
  const raw = localStorage.getItem(key);
  if (raw === null) return fallback;
  return raw === 'true';
}

function applyDom(isDark: boolean, isUltra: boolean) {
  document.body.setAttribute('class', isDark ? 'dark' : 'light');
  if (isUltra) {
    document.documentElement.setAttribute('data-theme', 'ultra-dark');
  } else {
    document.documentElement.removeAttribute('data-theme');
  }
  const msg = document.getElementById('message');
  if (msg) msg.className = isDark ? 'dark' : 'light';
}

// module load so the document is in the right theme before React mounts.
const initialDark = readBool(STORAGE_DARK, true);
const initialUltra = readBool(STORAGE_ULTRA, false);
applyDom(initialDark, initialUltra);

/* ── Dune palette — shared semantic colors ───────────────────────── */

const DUNE_PRIMARY = '#A67C52';
const DUNE_PRIMARY_HOVER = '#B8860B';
const DUNE_PRIMARY_ACTIVE = '#8B6914';
const DUNE_GOLD = '#C9A84C';
const DUNE_GOLD_BRIGHT = '#D4AF37';

const LIGHT_TOKENS = {
  colorPrimary: DUNE_PRIMARY,
  colorPrimaryHover: DUNE_PRIMARY_HOVER,
  colorPrimaryActive: DUNE_PRIMARY_ACTIVE,
  colorInfo: '#8B7355',
  colorSuccess: '#6B8F5E',
  colorWarning: DUNE_GOLD,
  colorError: '#A0522D',
  colorBgBase: '#EDE4D4',
  colorBgLayout: '#E8DFD0',
  colorBgContainer: '#FAF6F0',
  colorBgElevated: '#FFFDF8',
  colorText: '#2C1810',
  colorTextSecondary: '#5C4033',
  colorTextTertiary: '#8B7355',
  colorTextQuaternary: '#A89880',
  colorBorder: '#D4C4A8',
  colorBorderSecondary: '#E8DFD0',
  colorFill: 'rgba(60, 36, 16, 0.06)',
  colorFillSecondary: 'rgba(60, 36, 16, 0.04)',
  colorFillTertiary: 'rgba(60, 36, 16, 0.02)',
  borderRadius: 8,
  borderRadiusLG: 12,
  borderRadiusSM: 6,
  fontFamily:
    "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', sans-serif",
  fontSize: 14,
  fontSizeHeading1: 38,
  fontSizeHeading2: 30,
  fontSizeHeading3: 24,
  fontSizeHeading4: 20,
  fontSizeHeading5: 16,
  lineHeight: 1.6,
  lineHeightHeading1: 1.2,
  controlHeight: 38,
  boxShadow: '0 2px 12px rgba(44, 24, 16, 0.06)',
  boxShadowSecondary: '0 4px 20px rgba(44, 24, 16, 0.08)',
};

const DARK_TOKENS = {
  colorPrimary: DUNE_GOLD_BRIGHT,
  colorPrimaryHover: '#E0C060',
  colorPrimaryActive: DUNE_GOLD,
  colorInfo: '#C4A882',
  colorSuccess: '#7DA86E',
  colorWarning: DUNE_GOLD,
  colorError: '#C06040',
  colorBgBase: '#1A1612',
  colorBgLayout: '#1A1612',
  colorBgContainer: '#252018',
  colorBgElevated: '#2E2820',
  colorText: 'rgba(232, 213, 183, 0.92)',
  colorTextSecondary: 'rgba(196, 168, 130, 0.72)',
  colorTextTertiary: 'rgba(160, 140, 110, 0.55)',
  colorTextQuaternary: 'rgba(140, 120, 95, 0.4)',
  colorBorder: 'rgba(201, 168, 76, 0.12)',
  colorBorderSecondary: 'rgba(255, 255, 255, 0.06)',
  colorFill: 'rgba(255, 255, 255, 0.06)',
  colorFillSecondary: 'rgba(255, 255, 255, 0.04)',
  colorFillTertiary: 'rgba(255, 255, 255, 0.02)',
  borderRadius: 8,
  borderRadiusLG: 12,
  borderRadiusSM: 6,
  fontFamily:
    "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', sans-serif",
  fontSize: 14,
  fontSizeHeading1: 38,
  fontSizeHeading2: 30,
  fontSizeHeading3: 24,
  fontSizeHeading4: 20,
  fontSizeHeading5: 16,
  lineHeight: 1.6,
  lineHeightHeading1: 1.2,
  controlHeight: 38,
  boxShadow: '0 2px 16px rgba(0, 0, 0, 0.35)',
  boxShadowSecondary: '0 4px 24px rgba(0, 0, 0, 0.4)',
};

const ULTRA_DARK_TOKENS = {
  ...DARK_TOKENS,
  colorBgBase: '#0A0806',
  colorBgLayout: '#0A0806',
  colorBgContainer: '#12100C',
  colorBgElevated: '#1A1612',
};

const LIGHT_LAYOUT_TOKENS = {
  bodyBg: '#E8DFD0',
  headerBg: '#FAF6F0',
  headerColor: '#2C1810',
  footerBg: '#E8DFD0',
  siderBg: '#F0E6D6',
  triggerBg: '#E8DFD0',
  triggerColor: '#2C1810',
};

const DARK_LAYOUT_TOKENS = {
  bodyBg: '#1A1612',
  headerBg: '#14110E',
  headerColor: 'rgba(232, 213, 183, 0.92)',
  footerBg: '#1A1612',
  siderBg: '#14110E',
  triggerBg: '#252018',
  triggerColor: 'rgba(232, 213, 183, 0.92)',
};

const ULTRA_DARK_LAYOUT_TOKENS = {
  bodyBg: '#0A0806',
  headerBg: '#080604',
  headerColor: 'rgba(232, 213, 183, 0.92)',
  footerBg: '#0A0806',
  siderBg: '#080604',
  triggerBg: '#1A1612',
  triggerColor: 'rgba(232, 213, 183, 0.92)',
};

const LIGHT_MENU_TOKENS = {
  itemBg: '#F0E6D6',
  subMenuItemBg: '#E8DFD0',
  itemSelectedBg: 'rgba(166, 124, 82, 0.15)',
  itemHoverBg: 'rgba(166, 124, 82, 0.08)',
  itemSelectedColor: DUNE_PRIMARY,
  itemColor: '#5C4033',
  darkItemBg: '#F0E6D6',
  darkSubMenuItemBg: '#E8DFD0',
  darkPopupBg: '#FAF6F0',
};

const DARK_MENU_TOKENS = {
  darkItemBg: '#14110E',
  darkSubMenuItemBg: '#1A1612',
  darkPopupBg: '#252018',
  itemSelectedBg: 'rgba(212, 175, 55, 0.15)',
  itemHoverBg: 'rgba(212, 175, 55, 0.08)',
};

const ULTRA_DARK_MENU_TOKENS = {
  darkItemBg: '#080604',
  darkSubMenuItemBg: '#0A0806',
  darkPopupBg: '#12100C',
  itemSelectedBg: 'rgba(212, 175, 55, 0.12)',
  itemHoverBg: 'rgba(212, 175, 55, 0.06)',
};

const LIGHT_CARD_TOKENS = {
  colorBorderSecondary: 'rgba(60, 36, 16, 0.08)',
  headerBg: 'transparent',
};

const DARK_CARD_TOKENS = {
  colorBorderSecondary: 'rgba(255, 255, 255, 0.06)',
};

const ULTRA_DARK_CARD_TOKENS = {
  colorBorderSecondary: 'rgba(255, 255, 255, 0.04)',
};

const BUTTON_TOKENS = {
  primaryShadow: '0 2px 8px rgba(166, 124, 82, 0.25)',
  defaultShadow: 'none',
  fontWeight: 500,
};

const DARK_BUTTON_TOKENS = {
  primaryShadow: '0 2px 12px rgba(212, 175, 55, 0.2)',
  defaultShadow: 'none',
  fontWeight: 500,
};

const TABLE_TOKENS = {
  headerBg: 'color-mix(in srgb, #A67C52 6%, #FAF6F0)',
  headerColor: '#5C4033',
  rowHoverBg: 'rgba(166, 124, 82, 0.05)',
};

const DARK_TABLE_TOKENS = {
  headerBg: 'rgba(212, 175, 55, 0.06)',
  headerColor: 'rgba(196, 168, 130, 0.72)',
  rowHoverBg: 'rgba(212, 175, 55, 0.05)',
};

const INPUT_TOKENS = {
  activeBorderColor: DUNE_PRIMARY,
  hoverBorderColor: DUNE_GOLD,
  activeShadow: '0 0 0 2px rgba(201, 168, 76, 0.18)',
};

const DARK_INPUT_TOKENS = {
  activeBorderColor: DUNE_GOLD_BRIGHT,
  hoverBorderColor: DUNE_GOLD,
  activeShadow: '0 0 0 2px rgba(212, 175, 55, 0.15)',
};

const MODAL_TOKENS = {
  headerBg: 'transparent',
  titleFontSize: 18,
};

const STATISTIC_TOKENS = {
  contentFontSize: 17,
  titleFontSize: 11,
};

export function buildAntdThemeConfig(isDark: boolean, isUltra: boolean): ThemeConfig {
  if (!isDark) {
    return {
      algorithm: antdTheme.defaultAlgorithm,
      token: LIGHT_TOKENS,
      components: {
        Layout: LIGHT_LAYOUT_TOKENS,
        Menu: LIGHT_MENU_TOKENS,
        Card: LIGHT_CARD_TOKENS,
        Button: BUTTON_TOKENS,
        Table: TABLE_TOKENS,
        Input: INPUT_TOKENS,
        Modal: MODAL_TOKENS,
        Statistic: STATISTIC_TOKENS,
      },
    };
  }
  return {
    algorithm: antdTheme.darkAlgorithm,
    token: isUltra ? ULTRA_DARK_TOKENS : DARK_TOKENS,
    components: {
      Layout: isUltra ? ULTRA_DARK_LAYOUT_TOKENS : DARK_LAYOUT_TOKENS,
      Menu: isUltra ? ULTRA_DARK_MENU_TOKENS : DARK_MENU_TOKENS,
      Card: isUltra ? ULTRA_DARK_CARD_TOKENS : DARK_CARD_TOKENS,
      Button: DARK_BUTTON_TOKENS,
      Table: DARK_TABLE_TOKENS,
      Input: DARK_INPUT_TOKENS,
      Modal: MODAL_TOKENS,
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
  isDark: boolean;
  isUltra: boolean;
  toggleTheme: () => void;
  toggleUltra: () => void;
  antdThemeConfig: ThemeConfig;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [isDark, setIsDark] = useState<boolean>(initialDark);
  const [isUltra, setIsUltra] = useState<boolean>(initialUltra);

  useEffect(() => {
    applyDom(isDark, isUltra);
    localStorage.setItem(STORAGE_DARK, String(isDark));
    localStorage.setItem(STORAGE_ULTRA, String(isUltra));
  }, [isDark, isUltra]);

  const toggleTheme = useCallback(() => setIsDark((v) => !v), []);
  const toggleUltra = useCallback(() => setIsUltra((v) => !v), []);

  const antdThemeConfig = useMemo(() => buildAntdThemeConfig(isDark, isUltra), [isDark, isUltra]);

  const value = useMemo<ThemeContextValue>(
    () => ({ isDark, isUltra, toggleTheme, toggleUltra, antdThemeConfig }),
    [isDark, isUltra, toggleTheme, toggleUltra, antdThemeConfig],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error('useTheme must be used inside <ThemeProvider>');
  return ctx;
}
