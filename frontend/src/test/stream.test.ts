/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import { NetworkSettingsSchema } from '@/schemas/protocols';

const streamFixtures = import.meta.glob<unknown>('./golden/fixtures/stream/*.json', {
  eager: true,
  import: 'default',
});

function fixtureName(path: string): string {
  const file = path.split('/').pop() ?? path;
  return file.replace(/\.json$/, '');
}

describe('NetworkSettingsSchema fixtures', () => {
  const entries = Object.entries(streamFixtures).sort(([a], [b]) => a.localeCompare(b));
  expect(
    entries.length,
    'expected at least one fixture under golden/fixtures/stream',
  ).toBeGreaterThan(0);

  for (const [path, raw] of entries) {
    it(`parses ${fixtureName(path)} byte-stably`, () => {
      const parsed = NetworkSettingsSchema.parse(raw);
      expect(parsed).toMatchSnapshot();
    });
  }
});

describe('NetworkSettingsSchema method alias', () => {
  it('folds xray-core v26.7.11 method alias back into network', () => {
    const parsed = NetworkSettingsSchema.parse({ method: 'ws', wsSettings: {} });
    expect((parsed as { network?: string }).network).toBe('ws');
    expect((parsed as Record<string, unknown>).method).toBeUndefined();
  });

  it('prefers method over network when both are present', () => {
    const parsed = NetworkSettingsSchema.parse({
      method: 'grpc',
      network: 'tcp',
      grpcSettings: {},
    });
    expect((parsed as { network?: string }).network).toBe('grpc');
  });
});

// xray-core 26.10.17 (#7182) renamed the XDRIVE service, reading "Google Drive" only as an alias.
// XDRIVE's and MASQUE's xmux are client keys a JSON subscription hands on from the inbound.
describe('NetworkSettingsSchema xray-core 26.10.21 keys', () => {
  it('folds the "Google Drive" service into google-drive', () => {
    const parsed = NetworkSettingsSchema.parse({
      network: 'xdrive',
      xdriveSettings: { service: 'Google Drive', remoteFolder: 'f', secrets: ['a', 'b', 'c'] },
    }) as { xdriveSettings: { service: string } };
    expect(parsed.xdriveSettings.service).toBe('google-drive');
  });

  it('keeps the XDRIVE and MASQUE xmux', () => {
    const xmux = { maxConnections: 2, hMaxReusableSecs: '1800-3000' };
    const xdrive = NetworkSettingsSchema.parse({
      network: 'xdrive',
      xdriveSettings: { service: 'local', remoteFolder: '/srv/drive', xmux },
    }) as { xdriveSettings: { xmux?: unknown } };
    const masque = NetworkSettingsSchema.parse({
      network: 'masque',
      masqueSettings: { host: 'masque.example.com', xmux },
    }) as { masqueSettings: { xmux?: unknown } };
    expect(xdrive.xdriveSettings.xmux).toEqual(xmux);
    expect(masque.masqueSettings.xmux).toEqual(xmux);
  });
});
