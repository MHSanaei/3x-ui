import { theme as antdTheme } from 'antd';
import type { ThemeConfig } from 'antd';
import type { ThemeTokens } from '@/schemas/forms/theme';
import { BUILTIN_PRESETS } from './presets';

export const STORAGE_ACTIVE_THEME = 'xui_active_theme_id';
export const STORAGE_CUSTOM_THEMES = 'xui_custom_themes';

// Fallback legacy keys used in 3x-ui prior to theme presets
const LEGACY_DARK_KEY = 'dark-mode';
const LEGACY_ULTRA_KEY = 'isUltraDarkThemeEnabled';

export function readActiveThemeId(): string {
  try {
    const stored = localStorage.getItem(STORAGE_ACTIVE_THEME);
    if (stored) return stored;

    // Migrate from legacy keys if present
    return migrateLegacyTheme();
  } catch {
    return 'default-dark';
  }
}

export function saveActiveThemeId(id: string): void {
  try {
    localStorage.setItem(STORAGE_ACTIVE_THEME, id);
  } catch {}
}

export function readCustomThemes(): ThemeTokens[] {
  try {
    const raw = localStorage.getItem(STORAGE_CUSTOM_THEMES);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

export function saveCustomThemes(themes: ThemeTokens[]): void {
  try {
    localStorage.setItem(STORAGE_CUSTOM_THEMES, JSON.stringify(themes));
  } catch {}
}

export function migrateLegacyTheme(): string {
  try {
    const darkRaw = localStorage.getItem(LEGACY_DARK_KEY);
    const ultraRaw = localStorage.getItem(LEGACY_ULTRA_KEY);

    if (darkRaw === null && ultraRaw === null) {
      return 'default-dark';
    }

    const isDark = darkRaw !== 'false';
    const isUltra = ultraRaw === 'true';

    let resolvedId = 'default-dark';
    if (!isDark) {
      resolvedId = 'default-light';
    } else if (isUltra) {
      resolvedId = 'ultra-dark';
    }

    localStorage.setItem(STORAGE_ACTIVE_THEME, resolvedId);
    localStorage.removeItem(LEGACY_DARK_KEY);
    localStorage.removeItem(LEGACY_ULTRA_KEY);
    return resolvedId;
  } catch {
    return 'default-dark';
  }
}

export function resolveTheme(themeId: string, customThemes: ThemeTokens[] = []): ThemeTokens {
  if (BUILTIN_PRESETS[themeId]) {
    return BUILTIN_PRESETS[themeId];
  }
  const custom = customThemes.find((t) => t.id === themeId);
  if (custom) {
    return custom;
  }
  return BUILTIN_PRESETS['default-dark'];
}

export function applyCSSVariables(theme: ThemeTokens): void {
  if (typeof document === 'undefined') return;

  const root = document.documentElement;
  const isDark = theme.baseMode === 'dark';
  const isUltra = theme.id === 'ultra-dark';

  document.body.classList.remove('dark', 'light');
  document.body.classList.add(isDark ? 'dark' : 'light');

  root.style.colorScheme = theme.baseMode;

  if (isUltra) {
    root.setAttribute('data-theme', 'ultra-dark');
  } else {
    root.removeAttribute('data-theme');
  }

  // Synchronize CSS custom properties for page shell & cards
  root.style.setProperty('--bg-page', theme.colorBgBase);
  root.style.setProperty('--bg-card', theme.colorBgContainer);
  root.style.setProperty('--card-radius', `${theme.borderRadius}px`);
  root.style.setProperty(
    '--card-shadow',
    theme.cardStyle === 'flat'
      ? 'none'
      : isDark
        ? '0 1px 2px rgba(0, 0, 0, 0.4)'
        : '0 1px 2px rgba(0, 0, 0, 0.04)',
  );
  root.style.setProperty('--card-accent-opacity', theme.cardStyle === 'flat' ? '0' : '1');

  const msg = document.getElementById('message');
  if (msg) {
    msg.classList.remove('dark', 'light');
    msg.classList.add(isDark ? 'dark' : 'light');
  }
}

const SHARED_STYLE_CONFIG = {
  hashed: false,
  cssVar: { key: 'xui' },
} as const;

export function buildAntdConfig(theme: ThemeTokens): ThemeConfig {
  const algorithms = [
    theme.baseMode === 'dark' ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
  ];

  if (theme.density === 'compact') {
    algorithms.push(antdTheme.compactAlgorithm);
  }

  return {
    ...SHARED_STYLE_CONFIG,
    algorithm: algorithms,
    token: {
      colorPrimary: theme.colorPrimary,
      colorPrimaryHover: theme.colorPrimaryHover || theme.colorPrimary,
      colorBgBase: theme.colorBgBase,
      colorBgContainer: theme.colorBgContainer,
      colorBgElevated: theme.colorBgElevated,
      colorBorder: theme.colorBorder,
      borderRadius: theme.borderRadius,
      colorText: theme.colorText,
      colorTextSecondary: theme.colorTextSecondary,
    },
    components: {
      Statistic: {
        contentFontSize: 17,
        titleFontSize: 11,
      },
      Button: {
        colorPrimary: theme.colorPrimary,
        colorPrimaryHover: theme.colorPrimaryHover || theme.colorPrimary,
      },
      Layout: {
        bodyBg: theme.colorBgBase,
        headerBg: theme.colorBgElevated,
        headerColor: theme.colorText || '#ffffff',
        footerBg: theme.colorBgBase,
        siderBg: theme.colorBgElevated,
        triggerBg: theme.colorBgContainer,
        triggerColor: theme.colorText || '#ffffff',
      },
      Menu: {
        darkItemBg: theme.colorBgElevated,
        darkSubMenuItemBg: theme.colorBgBase,
        darkPopupBg: theme.colorBgContainer,
      },
      Card: {
        colorBorderSecondary: theme.colorBorder,
      },
    },
  };
}
