/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import { NetworkSettingsSchema } from '@/schemas/protocols';

const streamFixtures = import.meta.glob<unknown>(
  './golden/fixtures/stream/*.json',
  { eager: true, import: 'default' },
);

function fixtureName(path: string): string {
  const file = path.split('/').pop() ?? path;
  return file.replace(/\.json$/, '');
}

describe('NetworkSettingsSchema fixtures', () => {
  const entries = Object.entries(streamFixtures).sort(([a], [b]) => a.localeCompare(b));
  expect(entries.length, 'expected at least one fixture under golden/fixtures/stream').toBeGreaterThan(0);

  for (const [path, raw] of entries) {
    it(`parses ${fixtureName(path)} byte-stably`, () => {
      const parsed = NetworkSettingsSchema.parse(raw);
      expect(parsed).toMatchSnapshot();
    });
  }
});
