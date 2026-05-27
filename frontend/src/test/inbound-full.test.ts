/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import { InboundSchema } from '@/schemas/api/inbound';

// Full Inbound parse tests — exercises the intersection of network DU,
// security DU, settings DU, and orthogonal extras in a single
// round-trip. These fixtures are the input the link generators in
// lib/xray/inbound-link.ts will consume once extracted.

const fixtures = import.meta.glob<unknown>(
  './golden/fixtures/inbound-full/*.json',
  { eager: true, import: 'default' },
);

function fixtureName(path: string): string {
  const file = path.split('/').pop() ?? path;
  return file.replace(/\.json$/, '');
}

describe('InboundSchema (full) fixtures', () => {
  const entries = Object.entries(fixtures).sort(([a], [b]) => a.localeCompare(b));
  expect(entries.length, 'expected at least one fixture under golden/fixtures/inbound-full').toBeGreaterThan(0);

  for (const [path, raw] of entries) {
    it(`parses ${fixtureName(path)} byte-stably`, () => {
      const parsed = InboundSchema.parse(raw);
      expect(parsed).toMatchSnapshot();
    });
  }
});
