/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import { BalancerObjectSchema } from '@/schemas/routing';

const fixtures = import.meta.glob<unknown>(
  './golden/fixtures/balancer/*.json',
  { eager: true, import: 'default' },
);

function fixtureName(path: string): string {
  const file = path.split('/').pop() ?? path;
  return file.replace(/\.json$/, '');
}

describe('BalancerObjectSchema fixtures', () => {
  const entries = Object.entries(fixtures).sort(([a], [b]) => a.localeCompare(b));
  expect(entries.length, 'expected at least one fixture under golden/fixtures/balancer').toBeGreaterThan(0);

  for (const [path, raw] of entries) {
    it(`parses ${fixtureName(path)} byte-stably`, () => {
      const parsed = BalancerObjectSchema.parse(raw);
      expect(parsed).toMatchSnapshot();
    });
  }
});
