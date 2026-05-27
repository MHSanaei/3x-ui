/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import { RuleObjectSchema } from '@/schemas/routing';

const fixtures = import.meta.glob<unknown>(
  './golden/fixtures/rule/*.json',
  { eager: true, import: 'default' },
);

function fixtureName(path: string): string {
  const file = path.split('/').pop() ?? path;
  return file.replace(/\.json$/, '');
}

describe('RuleObjectSchema fixtures', () => {
  const entries = Object.entries(fixtures).sort(([a], [b]) => a.localeCompare(b));
  expect(entries.length, 'expected at least one fixture under golden/fixtures/rule').toBeGreaterThan(0);

  for (const [path, raw] of entries) {
    it(`parses ${fixtureName(path)} byte-stably`, () => {
      const parsed = RuleObjectSchema.parse(raw);
      expect(parsed).toMatchSnapshot();
    });
  }
});
