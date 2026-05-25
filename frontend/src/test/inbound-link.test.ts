/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import {
  genHysteriaLink,
  genInboundLinks,
  genShadowsocksLink,
  genTrojanLink,
  genVlessLink,
  genVmessLink,
  genWireguardConfig,
  genWireguardLink,
  resolveAddr,
} from '@/lib/xray/inbound-link';
import { Inbound as LegacyInbound } from '@/models/inbound';
import { InboundSchema } from '@/schemas/api/inbound';
import type { WireguardInboundSettings } from '@/schemas/protocols/inbound/wireguard';

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

describe('genVlessLink parity', () => {
  const fixtures = fixturesForProtocol('vless');
  expect(fixtures.length, 'need at least one vless full-inbound fixture').toBeGreaterThan(0);

  for (const [name, raw] of fixtures) {
    it(`${name}: matches legacy Inbound.genVLESSLink`, () => {
      const typed = InboundSchema.parse(raw);
      const settings = (raw as { settings: { clients: Array<{ id: string; flow?: string }> } }).settings;
      const client = settings.clients[0];

      const address = 'example.test';
      const port = typed.port;
      const remark = 'parity-test';

      const newLink = genVlessLink({
        inbound: typed,
        address,
        port,
        forceTls: 'same',
        remark,
        clientId: client.id,
        flow: client.flow as never,
        externalProxy: null,
      });

      const legacy = LegacyInbound.fromJson(raw);
      const legacyLink = legacy.genVLESSLink(address, port, 'same', remark, client.id, client.flow, null);

      expect(newLink).toBe(legacyLink);
    });
  }
});

describe('genTrojanLink parity', () => {
  const fixtures = fixturesForProtocol('trojan');
  expect(fixtures.length, 'need at least one trojan full-inbound fixture').toBeGreaterThan(0);

  for (const [name, raw] of fixtures) {
    it(`${name}: matches legacy Inbound.genTrojanLink`, () => {
      const typed = InboundSchema.parse(raw);
      const settings = (raw as { settings: { clients: Array<{ password: string }> } }).settings;
      const client = settings.clients[0];

      const address = 'example.test';
      const port = typed.port;
      const remark = 'parity-test';

      const newLink = genTrojanLink({
        inbound: typed,
        address,
        port,
        forceTls: 'same',
        remark,
        clientPassword: client.password,
        externalProxy: null,
      });

      const legacy = LegacyInbound.fromJson(raw);
      const legacyLink = legacy.genTrojanLink(address, port, 'same', remark, client.password, null);

      expect(newLink).toBe(legacyLink);
    });
  }
});

describe('genHysteriaLink parity', () => {
  const fixtures = fixturesForProtocol('hysteria');
  expect(fixtures.length, 'need at least one hysteria full-inbound fixture').toBeGreaterThan(0);

  for (const [name, raw] of fixtures) {
    it(`${name}: matches legacy Inbound.genHysteriaLink`, () => {
      const typed = InboundSchema.parse(raw);
      const settings = (raw as { settings: { clients: Array<{ auth: string }> } }).settings;
      const client = settings.clients[0];

      const address = 'example.test';
      const port = typed.port;
      const remark = 'parity-test';

      const newLink = genHysteriaLink({
        inbound: typed,
        address,
        port,
        remark,
        clientAuth: client.auth,
      });

      const legacy = LegacyInbound.fromJson(raw);
      const legacyLink = legacy.genHysteriaLink(address, port, remark, client.auth);

      expect(newLink).toBe(legacyLink);
    });
  }
});

describe('genWireguardLink + genWireguardConfig parity', () => {
  const fixtures = fixturesForProtocol('wireguard');
  expect(fixtures.length, 'need at least one wireguard full-inbound fixture').toBeGreaterThan(0);

  for (const [name, raw] of fixtures) {
    it(`${name}: matches legacy getWireguardLink + getWireguardTxt`, () => {
      const typed = InboundSchema.parse(raw);
      if (typed.protocol !== 'wireguard') throw new Error('not a wireguard fixture');
      // InboundSchema is an intersection of two DUs, so TS can't auto-narrow
      // `settings` from `protocol`. The runtime guard above is the real
      // check; this cast just helps the type checker.
      const settings = typed.settings as WireguardInboundSettings;

      const address = 'wg.example.test';
      const port = typed.port;
      const remark = 'wg-peer-1';
      const peerIndex = 0;

      const newLink = genWireguardLink({ settings, address, port, remark, peerIndex });
      const newConfig = genWireguardConfig({ settings, address, port, remark, peerIndex });

      const legacy = LegacyInbound.fromJson(raw);
      const legacyLink = legacy.getWireguardLink(address, port, remark, peerIndex);
      const legacyConfig = legacy.getWireguardTxt(address, port, remark, peerIndex);

      expect(newLink).toBe(legacyLink);
      expect(newConfig).toBe(legacyConfig);
    });
  }
});

describe('resolveAddr precedence', () => {
  const baseInbound = {
    listen: '',
    port: 443,
    protocol: 'vless' as const,
  };

  it('prefers hostOverride over listen and fallback', () => {
    expect(resolveAddr(
      { ...baseInbound, listen: '10.0.0.1' } as never,
      'cdn.example.test',
      'fallback.test',
    )).toBe('cdn.example.test');
  });

  it('uses listen when override is empty and listen is explicit', () => {
    expect(resolveAddr(
      { ...baseInbound, listen: '10.0.0.1' } as never,
      '',
      'fallback.test',
    )).toBe('10.0.0.1');
  });

  it('skips listen when it is 0.0.0.0 and falls through to fallbackHostname', () => {
    expect(resolveAddr(
      { ...baseInbound, listen: '0.0.0.0' } as never,
      '',
      'fallback.test',
    )).toBe('fallback.test');
  });

  it('falls through to fallbackHostname when listen is empty', () => {
    expect(resolveAddr(
      baseInbound as never,
      '',
      'fallback.test',
    )).toBe('fallback.test');
  });
});

describe('genInboundLinks orchestrator parity', () => {
  // Every full-inbound fixture should produce the same \r\n-joined link
  // block as the legacy Inbound.genInboundLinks. Pass hostOverride
  // explicitly so neither pipeline reaches for location.hostname.
  const fixtures = Object.entries(fullFixtures)
    .map(([path, raw]): [string, Record<string, unknown>] => [fixtureName(path), raw as Record<string, unknown>])
    .sort(([a], [b]) => a.localeCompare(b));

  for (const [name, raw] of fixtures) {
    const protocol = (raw as { protocol?: string }).protocol;
    // Skip protocols the legacy class can't dispatch (hysteria2 has no
    // dispatch case; getSettings(protocol) returns null and crashes
    // genHysteriaLink). Orchestrator-level parity covers the others.
    if (protocol === 'hysteria2') continue;

    it(`${name}: matches legacy Inbound.genInboundLinks`, () => {
      const typed = InboundSchema.parse(raw);

      const remark = 'parity-test';
      const hostOverride = 'override.test';
      const fallbackHostname = 'fallback.test';

      const newBlock = genInboundLinks({
        inbound: typed,
        remark,
        hostOverride,
        fallbackHostname,
      });

      const legacy = LegacyInbound.fromJson(raw);
      const legacyBlock = legacy.genInboundLinks(remark, '-ieo', hostOverride);

      expect(newBlock).toBe(legacyBlock);
    });
  }
});

describe('genShadowsocksLink parity', () => {
  const fixtures = fixturesForProtocol('shadowsocks');
  expect(fixtures.length, 'need at least one shadowsocks full-inbound fixture').toBeGreaterThan(0);

  for (const [name, raw] of fixtures) {
    it(`${name}: matches legacy Inbound.genSSLink`, () => {
      const typed = InboundSchema.parse(raw);
      const settings = (raw as { settings: { clients?: Array<{ password: string }> } }).settings;
      const client = settings.clients?.[0];

      const address = 'example.test';
      const port = typed.port;
      const remark = 'parity-test';
      const clientPassword = client?.password ?? '';

      const newLink = genShadowsocksLink({
        inbound: typed,
        address,
        port,
        forceTls: 'same',
        remark,
        clientPassword,
        externalProxy: null,
      });

      const legacy = LegacyInbound.fromJson(raw);
      const legacyLink = legacy.genSSLink(address, port, 'same', remark, clientPassword, null);

      expect(newLink).toBe(legacyLink);
    });
  }
});
