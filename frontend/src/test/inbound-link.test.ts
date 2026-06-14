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
  preferPublicHost,
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

  it('emits the UDP hop range as the v2rayN-compatible mport param', () => {
    const [, raw] = fixtures[0];
    const withHop = {
      ...raw,
      settings: { ...(raw.settings as Record<string, unknown>), version: 2 },
      streamSettings: {
        ...(raw.streamSettings as Record<string, unknown>),
        finalmask: { quicParams: { udpHop: { ports: '20000-50000', interval: '5-10' } } },
      },
    };
    const typed = InboundSchema.parse(withHop);
    const client = (raw.settings as { clients: Array<{ auth: string }> }).clients[0];

    const link = genHysteriaLink({
      inbound: typed,
      address: 'example.test',
      port: typed.port,
      remark: 'hop-test',
      clientAuth: client.auth,
    });

    expect(link.startsWith('hysteria2://')).toBe(true);
    expect(link).toContain(`@example.test:${typed.port}`);
    expect(link).toContain('mport=20000-50000');
    expect(link.endsWith('#hop-test')).toBe(true);
  });

  it('normalizes pinSHA256 to hex for base64, raw-hex and colon-hex pins (issue #4818)', () => {
    const [, raw] = fixtures[0];
    const base64Pin = 'yEfdI5XQl4wHgLggHEsomosoFZfUfCdfLXfT+W2N6cQ=';
    const hexPin = '84491c0312d9e70f519ce24659a2ca7d9c4ec59dc86417ece426945e0f939293';
    const colonPin = 'C8:47:DD:23:95:D0:97:8C:07:80:B8:20:1C:4B:28:9A:8B:28:15:97:D4:7C:27:5F:2D:77:D3:F9:6D:8D:E9:C4';
    const stream = raw.streamSettings as Record<string, unknown>;
    const tls = stream.tlsSettings as Record<string, unknown>;
    const tlsClientSettings = tls.settings as Record<string, unknown>;
    const withPins = {
      ...raw,
      streamSettings: {
        ...stream,
        tlsSettings: {
          ...tls,
          settings: { ...tlsClientSettings, pinnedPeerCertSha256: [base64Pin, hexPin, colonPin] },
        },
      },
    };
    const typed = InboundSchema.parse(withPins);
    const client = (raw.settings as { clients: Array<{ auth: string }> }).clients[0];

    const link = genHysteriaLink({
      inbound: typed,
      address: 'example.test',
      port: typed.port,
      remark: 'pin-test',
      clientAuth: client.auth,
    });

    const pin = new URL(link).searchParams.get('pinSHA256');
    expect(pin).toBe(
      'c847dd2395d0978c0780b8201c4b289a8b281597d47c275f2d77d3f96d8de9c4,' +
        '84491c0312d9e70f519ce24659a2ca7d9c4ec59dc86417ece426945e0f939293,' +
        'c847dd2395d0978c0780b8201c4b289a8b281597d47c275f2d77d3f96d8de9c4',
    );
  });

  it('emits an external proxy pin as hex pinSHA256 (not pcs)', () => {
    const [, raw] = fixtures[0];
    const typed = InboundSchema.parse(raw);
    const client = (raw.settings as { clients: Array<{ auth: string }> }).clients[0];

    const link = genHysteriaLink({
      inbound: typed,
      address: 'edge.example.com',
      port: 8443,
      remark: 'ep-pin',
      clientAuth: client.auth,
      externalProxy: {
        forceTls: 'tls',
        dest: 'edge.example.com',
        port: 8443,
        remark: 'ep-pin',
        // base64 SHA-256 — must come out hex-normalized for Hysteria.
        pinnedPeerCertSha256: ['yEfdI5XQl4wHgLggHEsomosoFZfUfCdfLXfT+W2N6cQ='],
      },
    });

    const url = new URL(link);
    expect(url.searchParams.get('pinSHA256')).toBe(
      'c847dd2395d0978c0780b8201c4b289a8b281597d47c275f2d77d3f96d8de9c4',
    );
    expect(url.searchParams.has('pcs')).toBe(false);
  });
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

  it('skips a unix socket path listen and falls through to fallbackHostname', () => {
    expect(resolveAddr(
      { ...baseInbound, listen: '/run/xray/in.sock' } as never,
      '',
      'fallback.test',
    )).toBe('fallback.test');
    expect(resolveAddr(
      { ...baseInbound, listen: '@xray-abstract' } as never,
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

  it('uses listen strategy with a shareable IPv6 listen before node override', () => {
    expect(resolveAddr(
      { ...baseInbound, listen: '[2001:db8::1]', shareAddrStrategy: 'listen', shareAddr: '' } as never,
      'node.example.test',
      'fallback.test',
    )).toBe('[2001:db8::1]');
  });

  it('uses listen strategy to prefer listen and fall back to node override', () => {
    expect(resolveAddr(
      { ...baseInbound, listen: '10.0.0.1', shareAddrStrategy: 'listen', shareAddr: '' } as never,
      'node.example.test',
      'fallback.test',
    )).toBe('10.0.0.1');
    expect(resolveAddr(
      { ...baseInbound, listen: '0.0.0.0', shareAddrStrategy: 'listen', shareAddr: '' } as never,
      'node.example.test',
      'fallback.test',
    )).toBe('node.example.test');
    expect(resolveAddr(
      { ...baseInbound, listen: 'localhost', shareAddrStrategy: 'listen', shareAddr: '' } as never,
      'node.example.test',
      'fallback.test',
    )).toBe('node.example.test');
  });

  it('uses custom strategy address before node override', () => {
    expect(resolveAddr(
      { ...baseInbound, listen: '10.0.0.1', shareAddrStrategy: 'custom', shareAddr: 'edge.example.test' } as never,
      'node.example.test',
      'fallback.test',
    )).toBe('edge.example.test');
  });

  it('normalizes a bare IPv6 custom strategy address', () => {
    expect(resolveAddr(
      { ...baseInbound, listen: '10.0.0.1', shareAddrStrategy: 'custom', shareAddr: '2001:db8::2' } as never,
      'node.example.test',
      'fallback.test',
    )).toBe('[2001:db8::2]');
  });

  it('ignores invalid custom strategy addresses and falls back to node override', () => {
    for (const shareAddr of ['https://edge.example.test', 'edge.example.test:8443', '[2001:db8::2]:8443', 'bad host']) {
      expect(resolveAddr(
        { ...baseInbound, listen: '10.0.0.1', shareAddrStrategy: 'custom', shareAddr } as never,
        'node.example.test',
        'fallback.test',
      )).toBe('node.example.test');
    }
  });
});

// #4829: reaching the panel through an SSH tunnel (127.0.0.1/localhost) must not
// leak the loopback host into share/QR links; a configured public host wins.
describe('preferPublicHost (loopback fallback)', () => {
  it('keeps a routable browser host as-is even when a public host is configured', () => {
    expect(preferPublicHost('panel.example.com', 'sub.example.com')).toBe('panel.example.com');
    expect(preferPublicHost('203.0.113.7', 'sub.example.com')).toBe('203.0.113.7');
  });

  it('substitutes the public host for loopback browser hosts', () => {
    for (const loop of ['127.0.0.1', 'localhost', '::1', '[::1]', '127.5.6.7']) {
      expect(preferPublicHost(loop, 'sub.example.com')).toBe('sub.example.com');
    }
  });

  it('leaves loopback untouched when no public host is configured', () => {
    expect(preferPublicHost('127.0.0.1', '')).toBe('127.0.0.1');
    expect(preferPublicHost('localhost', '')).toBe('localhost');
  });

  it('an explicit per-inbound listen still wins over the loopback fallback', () => {
    const inbound = { listen: '203.0.113.9', port: 443, protocol: 'vless' as const };
    expect(resolveAddr(
      inbound as never,
      '',
      preferPublicHost('127.0.0.1', 'sub.example.com'),
    )).toBe('203.0.113.9');
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

describe('IPv6 bracket wrapping in share-link authority', () => {
  it('genVlessLink brackets a bare IPv6 address', () => {
    const [, raw] = fixturesForProtocol('vless')[0];
    const typed = InboundSchema.parse(raw);
    const clientId = (raw as { settings: { clients: Array<{ id: string }> } }).settings.clients[0].id;

    const link = genVlessLink({
      inbound: typed,
      address: '2001:db8::1',
      port: 443,
      clientId,
    });
    expect(new URL(link).host).toBe('[2001:db8::1]:443');
  });

  it('genTrojanLink brackets a bare IPv6 address', () => {
    const [, raw] = fixturesForProtocol('trojan')[0];
    const typed = InboundSchema.parse(raw);
    const clientPassword = (raw as { settings: { clients: Array<{ password: string }> } }).settings.clients[0].password;

    const link = genTrojanLink({
      inbound: typed,
      address: '2001:db8::1',
      port: 443,
      clientPassword,
    });
    expect(new URL(link).host).toBe('[2001:db8::1]:443');
  });

  it('genShadowsocksLink brackets a bare IPv6 address', () => {
    const [, raw] = fixturesForProtocol('shadowsocks')[0];
    const typed = InboundSchema.parse(raw);
    const clientPassword = (raw as { settings: { clients?: Array<{ password: string }> } }).settings.clients?.[0]?.password ?? '';

    const link = genShadowsocksLink({
      inbound: typed,
      address: '2001:db8::1',
      port: 443,
      clientPassword,
    });
    expect(new URL(link).host).toBe('[2001:db8::1]:443');
  });

  it('genHysteriaLink brackets a bare IPv6 address', () => {
    const [, raw] = fixturesForProtocol('hysteria')[0];
    const typed = InboundSchema.parse(raw);
    const clientAuth = (raw as { settings: { clients: Array<{ auth: string }> } }).settings.clients[0].auth;

    const link = genHysteriaLink({
      inbound: typed,
      address: '2001:db8::1',
      port: 443,
      clientAuth,
    });
    expect(new URL(link).host).toBe('[2001:db8::1]:443');
  });

  it('genWireguardLink brackets a bare IPv6 address', () => {
    const [, raw] = fixturesForProtocol('wireguard')[0];
    const typed = InboundSchema.parse(raw);
    if (typed.protocol !== 'wireguard') throw new Error('not a wireguard fixture');
    const settings = typed.settings as WireguardInboundSettings;

    const link = genWireguardLink({
      settings,
      address: '2001:db8::1',
      port: 443,
      peerIndex: 0,
    });
    expect(new URL(link).host).toBe('[2001:db8::1]:443');
  });

  it('does not bracket IPv4 addresses or hostnames', () => {
    const [, raw] = fixturesForProtocol('vless')[0];
    const typed = InboundSchema.parse(raw);
    const clientId = (raw as { settings: { clients: Array<{ id: string }> } }).settings.clients[0].id;

    const v4 = genVlessLink({ inbound: typed, address: '203.0.113.7', port: 443, clientId });
    expect(new URL(v4).host).toBe('203.0.113.7:443');

    const host = genVlessLink({ inbound: typed, address: 'example.test', port: 443, clientId });
    expect(new URL(host).host).toBe('example.test:443');
  });
});

describe('external proxy pinned cert (pcs)', () => {
  const [, raw] = fixturesForProtocol('vless').find(([name]) => name === 'vless-ws-tls')!;
  const typed = InboundSchema.parse(raw);
  const clientId = (raw as { settings: { clients: Array<{ id: string }> } }).settings.clients[0].id;

  it('emits the external proxy pin list as pcs when forcing TLS', () => {
    const link = genVlessLink({
      inbound: typed,
      address: 'edge.example.com',
      port: 8443,
      forceTls: 'tls',
      remark: 'ep-pin',
      clientId,
      externalProxy: {
        forceTls: 'tls',
        dest: 'edge.example.com',
        port: 8443,
        remark: 'ep-pin',
        pinnedPeerCertSha256: ['aa11', 'bb22'],
      },
    });

    expect(new URL(link).searchParams.get('pcs')).toBe('aa11,bb22');
  });

  it('omits pcs when the external proxy forces security off', () => {
    const link = genVlessLink({
      inbound: typed,
      address: 'edge.example.com',
      port: 8080,
      forceTls: 'none',
      remark: 'ep-none',
      clientId,
      externalProxy: {
        forceTls: 'none',
        dest: 'edge.example.com',
        port: 8080,
        remark: 'ep-none',
        pinnedPeerCertSha256: ['aa11'],
      },
    });

    expect(new URL(link).searchParams.has('pcs')).toBe(false);
  });
});
