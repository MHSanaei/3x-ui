/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import { genVmessLink } from '@/lib/xray/inbound-link';
import { Inbound as LegacyInbound } from '@/models/inbound';
import { InboundSchema } from '@/schemas/api/inbound';

// Parity harness for the share-link extraction. For each full inbound
// fixture matching the protocol under test, we:
//   1. Parse with the Zod InboundSchema -> typed input for the new pure fn
//   2. Construct the legacy Inbound class via Inbound.fromJson(fixture)
//   3. Call both link generators with matching args
//   4. Assert the URLs match byte-for-byte
// Drift between the new pure fn and the legacy class method fails the
// test here, before the call sites in pages/ get swapped.

const fullFixtures = import.meta.glob<unknown>(
  './golden/fixtures/inbound-full/*.json',
  { eager: true, import: 'default' },
);

function fixtureName(path: string): string {
  const file = path.split('/').pop() ?? path;
  return file.replace(/\.json$/, '');
}

function fixturesForProtocol(protocol: string): Array<[string, Record<string, unknown>]> {
  return Object.entries(fullFixtures)
    .filter(([, raw]) => (raw as { protocol?: string }).protocol === protocol)
    .map(([path, raw]): [string, Record<string, unknown>] => [fixtureName(path), raw as Record<string, unknown>])
    .sort(([a], [b]) => a.localeCompare(b));
}

describe('genVmessLink parity', () => {
  const fixtures = fixturesForProtocol('vmess');
  expect(fixtures.length, 'need at least one vmess full-inbound fixture').toBeGreaterThan(0);

  for (const [name, raw] of fixtures) {
    it(`${name}: matches legacy Inbound.genVmessLink`, () => {
      const typed = InboundSchema.parse(raw);
      const settings = (raw as { settings: { clients: Array<{ id: string; security?: string }> } }).settings;
      const client = settings.clients[0];

      const address = 'example.test';
      const port = typed.port;
      const remark = 'parity-test';

      const newLink = genVmessLink({
        inbound: typed,
        address,
        port,
        forceTls: 'same',
        remark,
        clientId: client.id,
        security: client.security as never,
        externalProxy: null,
      });

      const legacy = LegacyInbound.fromJson(raw);
      const legacyLink = legacy.genVmessLink(address, port, 'same', remark, client.id, client.security, null);

      expect(newLink).toBe(legacyLink);
    });
  }
});
