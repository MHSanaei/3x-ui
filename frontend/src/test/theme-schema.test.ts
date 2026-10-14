import { describe, expect, it } from 'vitest';
import { ThemeTokenSchema } from '../schemas/forms/theme';
import { BUILTIN_PRESETS, PRESET_ORDER } from '../lib/themes/presets';

describe('ThemeTokenSchema', () => {
  it('validates all built-in presets', () => {
    for (const id of PRESET_ORDER) {
      const preset = BUILTIN_PRESETS[id];
      const result = ThemeTokenSchema.safeParse(preset);
      expect(result.success, `Preset "${id}" failed validation`).toBe(true);
    }
  });

  it('rejects invalid hex colors', () => {
    const bad = {
      ...BUILTIN_PRESETS['default-dark'],
      colorPrimary: 'not-a-color',
    };
    const result = ThemeTokenSchema.safeParse(bad);
    expect(result.success).toBe(false);
  });

  it('rejects borderRadius out of range', () => {
    const bad = {
      ...BUILTIN_PRESETS['default-dark'],
      borderRadius: 99,
    };
    const result = ThemeTokenSchema.safeParse(bad);
    expect(result.success).toBe(false);
  });

  it('rejects unknown properties in strict mode', () => {
    const bad = {
      ...BUILTIN_PRESETS['default-dark'],
      unknownProperty: 'should-fail',
    };
    const result = ThemeTokenSchema.safeParse(bad);
    expect(result.success).toBe(false);
  });
});
