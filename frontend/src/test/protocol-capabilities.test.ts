/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import {
  canEnableTls,
  canEnableReality,
  canEnableTlsFlow,
  canEnableStream,
  canEnableVisionSeed,
  isSS2022,
  isSSMultiUser,
} from '@/lib/xray/protocol-capabilities';

// Pure-function tests for the capability predicates. Each fixture × stream
// case is locked via snapshot — these were captured at the close of the
// legacy class migration and verified byte-equal to the legacy Inbound
// class instance methods. Drift past this baseline is a regression.

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

describe('protocol capability predicates', () => {
  const entries = Object.entries(fixtures).sort(([a], [b]) => a.localeCompare(b));
  for (const [path, raw] of entries) {
    const name = fixtureName(path);
    const fix = raw as FixtureShape;

    for (const stream of STREAM_CASES) {

      it(`${name} :: ${stream.network}/${stream.security}`, () => {
        const values = {
          protocol: fix.protocol,
          streamSettings: { network: stream.network, security: stream.security },
          settings: fix.settings,
        };
        const result = {
          canEnableTls: canEnableTls(values),
          canEnableReality: canEnableReality(values),
          canEnableTlsFlow: canEnableTlsFlow(values),
          canEnableStream: canEnableStream(values),
          canEnableVisionSeed: canEnableVisionSeed(values),
          isSS2022: isSS2022(values),
          isSSMultiUser: isSSMultiUser(values),
        };
        expect(result).toMatchSnapshot();
      });
    }
  }
});
