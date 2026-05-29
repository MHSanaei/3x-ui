/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import { FinalMaskStreamSettingsSchema } from '@/schemas/protocols/stream';

const fixtures = import.meta.glob<unknown>(
  './golden/fixtures/finalmask/*.json',
  { eager: true, import: 'default' },
);

function fixtureName(path: string): string {
  const file = path.split('/').pop() ?? path;
  return file.replace(/\.json$/, '');
}

describe('FinalMaskStreamSettingsSchema fixtures', () => {
  const entries = Object.entries(fixtures).sort(([a], [b]) => a.localeCompare(b));
  expect(entries.length, 'expected at least one fixture under golden/fixtures/finalmask').toBeGreaterThan(0);

  for (const [path, raw] of entries) {
    it(`parses ${fixtureName(path)} byte-stably`, () => {
      const parsed = FinalMaskStreamSettingsSchema.parse(raw);
      expect(parsed).toMatchSnapshot();
    });
  }
});
