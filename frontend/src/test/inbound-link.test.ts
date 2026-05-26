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
import { InboundSchema } from '@/schemas/api/inbound';
import type { WireguardInboundSettings } from '@/schemas/protocols/inbound/wireguard';

// Snapshot baseline for the share-link generators. Snapshots were locked
// at the close of the legacy class migration — at that point each
// generator was verified byte-equal to the corresponding legacy Inbound
// class method. Future drift past this baseline is a regression.

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

describe('genVmessLink', () => {
  const fixtures = fixturesForProtocol('vmess');
  expect(fixtures.length, 'need at least one vmess full-inbound fixture').toBeGreaterThan(0);

  for (const [name, raw] of fixtures) {
    it(`${name}: byte-stable`, () => {
      const typed = InboundSchema.parse(raw);
      const settings = (raw as { settings: { clients: Array<{ id: string; security?: string }> } }).settings;
      const client = settings.clients[0];

      const link = genVmessLink({
        inbound: typed,
        address: 'example.test',
        port: typed.port,
        forceTls: 'same',
        remark: 'parity-test',
        clientId: client.id,
        security: client.security as never,
        externalProxy: null,
      });
      expect(link).toMatchSnapshot();
    });
  }
});

describe('genVlessLink', () => {
  const fixtures = fixturesForProtocol('vless');
  expect(fixtures.length, 'need at least one vless full-inbound fixture').toBeGreaterThan(0);

  for (const [name, raw] of fixtures) {
    it(`${name}: byte-stable`, () => {
      const typed = InboundSchema.parse(raw);
      const settings = (raw as { settings: { clients: Array<{ id: string; flow?: string }> } }).settings;
      const client = settings.clients[0];

      const link = genVlessLink({
        inbound: typed,
        address: 'example.test',
        port: typed.port,
        forceTls: 'same',
        remark: 'parity-test',
        clientId: client.id,
        flow: client.flow as never,
        externalProxy: null,
      });
      expect(link).toMatchSnapshot();
    });
  }
});

describe('genTrojanLink', () => {
  const fixtures = fixturesForProtocol('trojan');
  expect(fixtures.length, 'need at least one trojan full-inbound fixture').toBeGreaterThan(0);

  for (const [name, raw] of fixtures) {
    it(`${name}: byte-stable`, () => {
      const typed = InboundSchema.parse(raw);
      const settings = (raw as { settings: { clients: Array<{ password: string }> } }).settings;
      const client = settings.clients[0];

      const link = genTrojanLink({
        inbound: typed,
        address: 'example.test',
        port: typed.port,
        forceTls: 'same',
        remark: 'parity-test',
        clientPassword: client.password,
        externalProxy: null,
      });
      expect(link).toMatchSnapshot();
    });
  }
});

describe('genHysteriaLink', () => {
  const fixtures = fixturesForProtocol('hysteria');
  expect(fixtures.length, 'need at least one hysteria full-inbound fixture').toBeGreaterThan(0);

  for (const [name, raw] of fixtures) {
    it(`${name}: byte-stable`, () => {
      const typed = InboundSchema.parse(raw);
      const settings = (raw as { settings: { clients: Array<{ auth: string }> } }).settings;
      const client = settings.clients[0];

      const link = genHysteriaLink({
        inbound: typed,
        address: 'example.test',
        port: typed.port,
        remark: 'parity-test',
        clientAuth: client.auth,
      });
      expect(link).toMatchSnapshot();
    });
  }
});

describe('genWireguardLink + genWireguardConfig', () => {
  const fixtures = fixturesForProtocol('wireguard');
  expect(fixtures.length, 'need at least one wireguard full-inbound fixture').toBeGreaterThan(0);

  for (const [name, raw] of fixtures) {
    it(`${name}: byte-stable`, () => {
      const typed = InboundSchema.parse(raw);
      if (typed.protocol !== 'wireguard') throw new Error('not a wireguard fixture');
      // InboundSchema is an intersection of two DUs, so TS can't auto-narrow
      // `settings` from `protocol`. The runtime guard above is the real
      // check; this cast just helps the type checker.
      const settings = typed.settings as WireguardInboundSettings;

      const link = genWireguardLink({
        settings,
        address: 'wg.example.test',
        port: typed.port,
        remark: 'wg-peer-1',
        peerIndex: 0,
      });
      const config = genWireguardConfig({
        settings,
        address: 'wg.example.test',
        port: typed.port,
        remark: 'wg-peer-1',
        peerIndex: 0,
      });
      expect({ link, config }).toMatchSnapshot();
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

describe('genInboundLinks orchestrator', () => {
  // Every full-inbound fixture should produce the same \r\n-joined link
  // block at this baseline.
  const fixtures = Object.entries(fullFixtures)
    .map(([path, raw]): [string, Record<string, unknown>] => [fixtureName(path), raw as Record<string, unknown>])
    .sort(([a], [b]) => a.localeCompare(b));

  for (const [name, raw] of fixtures) {
    it(`${name}: byte-stable`, () => {
      const typed = InboundSchema.parse(raw);
      const block = genInboundLinks({
        inbound: typed,
        remark: 'parity-test',
        hostOverride: 'override.test',
        fallbackHostname: 'fallback.test',
      });
      expect(block).toMatchSnapshot();
    });
  }
});

describe('genShadowsocksLink', () => {
  const fixtures = fixturesForProtocol('shadowsocks');
  expect(fixtures.length, 'need at least one shadowsocks full-inbound fixture').toBeGreaterThan(0);

  for (const [name, raw] of fixtures) {
    it(`${name}: byte-stable`, () => {
      const typed = InboundSchema.parse(raw);
      const settings = (raw as { settings: { clients?: Array<{ password: string }> } }).settings;
      const client = settings.clients?.[0];

      const link = genShadowsocksLink({
        inbound: typed,
        address: 'example.test',
        port: typed.port,
        forceTls: 'same',
        remark: 'parity-test',
        clientPassword: client?.password ?? '',
        externalProxy: null,
      });
      expect(link).toMatchSnapshot();
    });
  }
});
