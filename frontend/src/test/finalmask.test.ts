/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import { parseGeckoPacketSize } from '@/lib/xray/forms/transport/FinalMaskForm';
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

describe('parseGeckoPacketSize', () => {
  it('accepts positive ordered packet size ranges', () => {
    expect(parseGeckoPacketSize('512-1200')).toEqual({ min: 512, max: 1200 });
    expect(parseGeckoPacketSize('1200-1200')).toEqual({ min: 1200, max: 1200 });
    expect(parseGeckoPacketSize('1-2048')).toEqual({ min: 1, max: 2048 });
  });

  it('rejects invalid packet size ranges', () => {
    expect(parseGeckoPacketSize('')).toBeNull();
    expect(parseGeckoPacketSize('0-1200')).toBeNull();
    expect(parseGeckoPacketSize('1200-512')).toBeNull();
    expect(parseGeckoPacketSize('512')).toBeNull();
    expect(parseGeckoPacketSize('512-abc')).toBeNull();
    // exceeds xray-core's gecko buffer (max 2048)
    expect(parseGeckoPacketSize('512-2049')).toBeNull();
    expect(parseGeckoPacketSize('512-9999')).toBeNull();
  });
});
