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

describe('DnsServerObjectSchema port defaulting', () => {
  it('defaults port 53 for a plain address', () => {
    const parsed = DnsServerObjectSchema.parse({ address: '8.8.8.8' });
    expect(parsed.port).toBe(53);
  });

  it('defaults port 53 for a tcp address', () => {
    const parsed = DnsServerObjectSchema.parse({ address: 'tcp://1.1.1.1' });
    expect(parsed.port).toBe(53);
  });

  it('omits port for a DoH (https://) address', () => {
    const parsed = DnsServerObjectSchema.parse({ address: 'https://cloudflare-dns.com/dns-query' });
    expect(parsed.port).toBeUndefined();
  });

  it('omits port for a DoHL (https+local://) address', () => {
    const parsed = DnsServerObjectSchema.parse({ address: 'https+local://dns.google/dns-query' });
    expect(parsed.port).toBeUndefined();
  });

  it('omits port for a DoQ (quic+local://) address', () => {
    const parsed = DnsServerObjectSchema.parse({ address: 'quic+local://dns.adguard.com' });
    expect(parsed.port).toBeUndefined();
  });

  it('omits port for an h2c and h2c+local address', () => {
    expect(DnsServerObjectSchema.parse({ address: 'h2c://dns.example.com/dns-query' }).port).toBeUndefined();
    expect(DnsServerObjectSchema.parse({ address: 'h2c+local://dns.example.com/dns-query' }).port).toBeUndefined();
  });

  it('omits port for an uppercase encrypted scheme', () => {
    const parsed = DnsServerObjectSchema.parse({ address: 'HTTPS://dns.google/dns-query' });
    expect(parsed.port).toBeUndefined();
  });

  it('preserves an explicit port on an encrypted address', () => {
    const parsed = DnsServerObjectSchema.parse({ address: 'https://dns.google/dns-query', port: 8443 });
    expect(parsed.port).toBe(8443);
  });
});
