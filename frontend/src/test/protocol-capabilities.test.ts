/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import { Inbound } from '@/models/inbound';
import {
  canEnableTls,
  canEnableReality,
  canEnableTlsFlow,
  canEnableStream,
  canEnableVisionSeed,
  isSS2022,
  isSSMultiUser,
} from '@/lib/xray/protocol-capabilities';

// Parity harness for the capability predicates. For each golden fixture
// (protocol+settings), cross with a matrix of stream configurations
// (network × security), build the legacy Inbound class via fromJson, and
// assert each pure-function predicate matches the class method.
//
// Only the (protocol × stream-shape) cross matters here — the predicates
// never read sniffing/port/listen, so we hold those constant.

const fixtures = import.meta.glob<unknown>(
  './golden/fixtures/inbound/*.json',
  { eager: true, import: 'default' },
);

interface FixtureShape { protocol: string; settings: Record<string, unknown> }

const STREAM_CASES: { network: string; security: string }[] = [
  { network: 'tcp',         security: 'none' },
  { network: 'tcp',         security: 'tls' },
  { network: 'tcp',         security: 'reality' },
  { network: 'ws',          security: 'none' },
  { network: 'ws',          security: 'tls' },
  { network: 'grpc',        security: 'none' },
  { network: 'grpc',        security: 'tls' },
  { network: 'grpc',        security: 'reality' },
  { network: 'kcp',         security: 'none' },
  { network: 'httpupgrade', security: 'none' },
  { network: 'httpupgrade', security: 'tls' },
  { network: 'xhttp',       security: 'none' },
  { network: 'xhttp',       security: 'tls' },
  { network: 'xhttp',       security: 'reality' },
];

function fixtureName(path: string): string {
  return (path.split('/').pop() ?? path).replace(/\.json$/, '');
}

describe('protocol capability predicates: pure ↔ legacy parity', () => {
  const entries = Object.entries(fixtures).sort(([a], [b]) => a.localeCompare(b));
  for (const [path, raw] of entries) {
    const name = fixtureName(path);
    const fix = raw as FixtureShape;

    for (const stream of STREAM_CASES) {

      it(`${name} :: ${stream.network}/${stream.security}`, () => {
        const wireConfig = {
          port: 12345,
          listen: '127.0.0.1',
          protocol: fix.protocol,
          settings: fix.settings,
          streamSettings: { network: stream.network, security: stream.security },
          sniffing: {},
        };
        const legacy = Inbound.fromJson(wireConfig);
        const values = {
          protocol: fix.protocol,
          streamSettings: { network: stream.network, security: stream.security },
          settings: fix.settings,
        };

        expect(canEnableTls(values)).toBe(legacy.canEnableTls());
        expect(canEnableReality(values)).toBe(legacy.canEnableReality());
        expect(canEnableTlsFlow(values)).toBe(legacy.canEnableTlsFlow());
        expect(canEnableStream(values)).toBe(legacy.canEnableStream());
        expect(canEnableVisionSeed(values)).toBe(legacy.canEnableVisionSeed());
        expect(isSS2022(values)).toBe(legacy.isSS2022);
        expect(isSSMultiUser(values)).toBe(legacy.isSSMultiUser);
      });
    }
  }
});
