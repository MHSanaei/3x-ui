import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { describe, expect, it } from 'vitest';

import { matchesFactoryDefault } from '@/components/ui/DefaultSettingTag';
import { AllSetting } from '@/models/setting';

/*
 * Contract test between the three homes of a setting's default value: the Go
 * defaultValueMap (authoritative, served by /setting/factoryDefaults), the
 * frontend AllSetting class defaults (what the form shows when the API omits
 * a value), and the tag's own comparison. If someone bumps a default on one
 * side only, the Default tag would start calling a different value "Default"
 * than the one the form displays — this test fails instead.
 */

function goDefaultLiterals(): Record<string, string> {
  const source = readFileSync(
    resolve(process.cwd(), '..', 'internal', 'web', 'service', 'setting.go'),
    'utf8',
  );
  const start = source.indexOf('var defaultValueMap = map[string]string{');
  const end = source.indexOf('\n}', start);
  const block = source.slice(start, end);
  const literals: Record<string, string> = {};
  for (const match of block.matchAll(/"([A-Za-z0-9]+)":\s+"((?:[^"\\]|\\.)*)"\s*,/g)) {
    literals[match[1]] = JSON.parse(`"${match[2]}"`);
  }
  return literals;
}

describe('factory defaults contract', () => {
  const goDefaults = goDefaultLiterals();
  const frontend = new AllSetting() as unknown as Record<string, unknown>;
  const sharedKeys = Object.keys(goDefaults).filter((key) => {
    const value = frontend[key];
    return typeof value === 'number' || typeof value === 'boolean' || typeof value === 'string';
  });

  it('parses a plausible slice of the Go map', () => {
    expect(goDefaults.webPort).toBe('2053');
    expect(goDefaults.subPort).toBe('2096');
    expect(sharedKeys.length).toBeGreaterThan(20);
  });

  it.each(sharedKeys)('frontend default for %s matches the shipped default', (key) => {
    expect(
      matchesFactoryDefault(frontend[key], goDefaults[key]),
      `AllSetting.${key} = ${JSON.stringify(frontend[key])} vs defaultValueMap ${JSON.stringify(goDefaults[key])}`,
    ).toBe(true);
  });
});
