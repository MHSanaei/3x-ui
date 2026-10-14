import { describe, expect, it, beforeEach } from 'vitest';
import {
  buildAntdConfig,
  applyCSSVariables,
  migrateLegacyTheme,
  saveActiveThemeId,
  readActiveThemeId,
  readCustomThemes,
  saveCustomThemes,
  resolveTheme,
} from '../lib/themes/engine';
import { BUILTIN_PRESETS } from '../lib/themes/presets';

describe('Theme Engine', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('builds Ant Design 5 config for default dark preset', () => {
    const config = buildAntdConfig(BUILTIN_PRESETS['default-dark']);
    expect(config.token?.colorPrimary).toBe('#1668dc');
    expect(config.token?.colorBgBase).toBe('#1a1b1f');
  });

  it('builds Ant Design 5 config for classic 2.x preset with compact algorithm', () => {
    const config = buildAntdConfig(BUILTIN_PRESETS['classic-2x']);
    expect(config.token?.colorPrimary).toBe('#1890ff');
    expect(config.token?.borderRadius).toBe(4);
    expect(Array.isArray(config.algorithm)).toBe(true);
    expect(config.algorithm).toHaveLength(2); // darkAlgorithm + compactAlgorithm
  });

  it('migrates legacy boolean localStorage keys to preset id', () => {
    localStorage.setItem('dark-mode', 'true');
    localStorage.setItem('isUltraDarkThemeEnabled', 'true');

    const migrated = migrateLegacyTheme();
    expect(migrated).toBe('ultra-dark');
    expect(localStorage.getItem('xui_active_theme_id')).toBe('ultra-dark');
    expect(localStorage.getItem('dark-mode')).toBeNull();
    expect(localStorage.getItem('isUltraDarkThemeEnabled')).toBeNull();
  });

  it('migrates legacy light mode correctly', () => {
    localStorage.setItem('dark-mode', 'false');

    const migrated = migrateLegacyTheme();
    expect(migrated).toBe('default-light');
  });

  it('saves and reads active theme id', () => {
    saveActiveThemeId('nord');
    expect(readActiveThemeId()).toBe('nord');
  });

  it('saves and reads custom themes in localStorage', () => {
    const custom = [
      {
        ...BUILTIN_PRESETS['classic-2x'],
        id: 'custom-blue',
        name: 'Custom Blue',
      },
    ];
    saveCustomThemes(custom);
    const loaded = readCustomThemes();
    expect(loaded).toHaveLength(1);
    expect(loaded[0].id).toBe('custom-blue');
  });

  it('resolves preset or falls back to default-dark', () => {
    expect(resolveTheme('nord').id).toBe('nord');
    expect(resolveTheme('non-existent').id).toBe('default-dark');
  });

  it('applies CSS variables to documentElement', () => {
    applyCSSVariables(BUILTIN_PRESETS['classic-2x']);
    const root = document.documentElement;
    expect(root.style.getPropertyValue('--bg-page')).toBe('#000000');
    expect(root.style.getPropertyValue('--bg-card')).toBe('#141414');
    expect(root.style.getPropertyValue('--card-radius')).toBe('4px');
    expect(root.style.getPropertyValue('--card-shadow')).toBe('none');
    expect(root.style.getPropertyValue('--card-accent-opacity')).toBe('0');
  });
});
