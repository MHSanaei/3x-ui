/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import { SecuritySettingsSchema } from '@/schemas/protocols';
import { RealityStreamSettingsSchema } from '@/schemas/protocols/security/reality';

const securityFixtures = import.meta.glob<unknown>(
  './golden/fixtures/security/*.json',
  { eager: true, import: 'default' },
);

function fixtureName(path: string): string {
  const file = path.split('/').pop() ?? path;
  return file.replace(/\.json$/, '');
}

describe('SecuritySettingsSchema fixtures', () => {
  const entries = Object.entries(securityFixtures).sort(([a], [b]) => a.localeCompare(b));
  expect(entries.length, 'expected at least one fixture under golden/fixtures/security').toBeGreaterThan(0);

  for (const [path, raw] of entries) {
    it(`parses ${fixtureName(path)} byte-stably`, () => {
      const parsed = SecuritySettingsSchema.parse(raw);
      expect(parsed).toMatchSnapshot();
    });
  }
});

describe('RealityStreamSettingsSchema dest -> target alias', () => {
  it('maps legacy `dest` to `target` when `target` is absent', () => {
    const parsed = RealityStreamSettingsSchema.parse({
      dest: 'example.com:443',
      serverNames: ['example.com'],
    });
    expect(parsed.target).toBe('example.com:443');
  });

  it('keeps `target` when both keys are present', () => {
    const parsed = RealityStreamSettingsSchema.parse({
      target: 'example.com:443',
      dest: 'other.com:443',
    });
    expect(parsed.target).toBe('example.com:443');
  });

  it('does not let an empty `target` shadow a present `dest`', () => {
    const parsed = RealityStreamSettingsSchema.parse({
      target: '',
      dest: 'example.com:443',
    });
    expect(parsed.target).toBe('example.com:443');
  });

  it('migrates `dest` through the security discriminated union', () => {
    const parsed = SecuritySettingsSchema.parse({
      security: 'reality',
      realitySettings: { dest: 'caddy:443', serverNames: ['volov.online'] },
    });
    if (parsed.security !== 'reality') throw new Error('expected reality branch');
    expect(parsed.realitySettings.target).toBe('caddy:443');
  });
});
