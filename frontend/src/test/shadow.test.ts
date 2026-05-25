/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import { Inbound } from '@/models/inbound';
import { InboundSettingsSchema } from '@/schemas/protocols';

// Walks every inbound golden fixture through both pipelines:
//   OLD:   Inbound.Settings.fromJson(protocol, raw.settings).toJson()
//   NEW:   InboundSettingsSchema.parse(raw).settings
// Then canonicalizes (deep key-sort, undefined-strip via JSON round-trip)
// and asserts byte-equality. This is the safety net for Step 3d — once we
// start extracting class methods into lib/xray/* pure functions, any
// normalization drift trips a snapshot diff here.

const fixtures = import.meta.glob<unknown>(
  './golden/fixtures/inbound/*.json',
  { eager: true, import: 'default' },
);

type FixtureShape = { protocol: string; settings: unknown };

// The OLD panel class collapses hysteria + hysteria2 onto a single
// HysteriaSettings (distinguished only by `version`), so when a fixture
// carries the wire-level hysteria2 protocol literal we dispatch to the
// HYSTERIA branch on the legacy side.
function legacyProtocolFor(protocol: string): string {
  if (protocol === 'hysteria2') return 'hysteria';
  return protocol;
}

// Drops empty arrays and undefined/null fields, then sorts keys. The legacy
// class's toJson() omits optional fields whose value is the empty array
// (e.g. fallbacks: []); the Zod schema includes them because of .default([]).
// Both represent the same wire state, so we treat them as equivalent here.
function canonicalize(value: unknown): string {
  function normalize(v: unknown): unknown {
    if (Array.isArray(v)) {
      const items = v.map(normalize).filter((x) => x !== undefined);
      return items.length === 0 ? undefined : items;
    }
    if (v && typeof v === 'object') {
      const entries = Object.entries(v as Record<string, unknown>)
        .map(([k, val]) => [k, normalize(val)] as const)
        .filter(([, val]) => val !== undefined && val !== null)
        .sort(([a], [b]) => a.localeCompare(b));
      return entries.length === 0 ? undefined : Object.fromEntries(entries);
    }
    return v;
  }
  return JSON.stringify(normalize(value) ?? null);
}

function fixtureName(path: string): string {
  const file = path.split('/').pop() ?? path;
  return file.replace(/\.json$/, '');
}

describe('shadow parse: legacy class vs Zod schema', () => {
  const entries = Object.entries(fixtures).sort(([a], [b]) => a.localeCompare(b));

  for (const [path, raw] of entries) {
    const fixture = raw as FixtureShape;
    const name = fixtureName(path);

    it(`${name}: legacy toJson() and Zod parse converge`, () => {
      const legacyInstance = Inbound.Settings.fromJson(
        legacyProtocolFor(fixture.protocol),
        fixture.settings,
      );
      expect(legacyInstance, `legacy dispatch returned null for ${fixture.protocol}`).not.toBeNull();
      const legacyJson = legacyInstance.toJson();

      const zodParsed = InboundSettingsSchema.parse(fixture);

      expect(canonicalize(zodParsed.settings)).toBe(canonicalize(legacyJson));
    });
  }
});
