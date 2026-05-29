/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import { DnsObjectSchema, DnsServerObjectSchema } from '@/schemas/dns';

function fixtureName(path: string): string {
  const file = path.split('/').pop() ?? path;
  return file.replace(/\.json$/, '');
}

const dnsFixtures = import.meta.glob<unknown>(
  './golden/fixtures/dns/*.json',
  { eager: true, import: 'default' },
);

const serverFixtures = import.meta.glob<unknown>(
  './golden/fixtures/dns-server/*.json',
  { eager: true, import: 'default' },
);

describe('DnsObjectSchema fixtures', () => {
  const entries = Object.entries(dnsFixtures).sort(([a], [b]) => a.localeCompare(b));
  expect(entries.length, 'expected at least one fixture under golden/fixtures/dns').toBeGreaterThan(0);

  for (const [path, raw] of entries) {
    it(`parses ${fixtureName(path)} byte-stably`, () => {
      const parsed = DnsObjectSchema.parse(raw);
      expect(parsed).toMatchSnapshot();
    });
  }
});

describe('DnsServerObjectSchema fixtures', () => {
  const entries = Object.entries(serverFixtures).sort(([a], [b]) => a.localeCompare(b));
  expect(entries.length, 'expected at least one fixture under golden/fixtures/dns-server').toBeGreaterThan(0);

  for (const [path, raw] of entries) {
    it(`parses ${fixtureName(path)} byte-stably`, () => {
      const parsed = DnsServerObjectSchema.parse(raw);
      expect(parsed).toMatchSnapshot();
    });
  }
});
