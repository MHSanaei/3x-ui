import { describe, expect, it } from 'vitest';

import {
  createDefaultBlackholeOutboundSettings,
  createDefaultDNSOutboundSettings,
  createDefaultFreedomOutboundSettings,
  createDefaultHttpOutboundSettings,
  createDefaultHysteriaOutboundSettings,
  createDefaultLoopbackOutboundSettings,
  createDefaultShadowsocksOutboundSettings,
  createDefaultSocksOutboundSettings,
  createDefaultTrojanOutboundSettings,
  createDefaultVlessOutboundSettings,
  createDefaultVmessOutboundSettings,
  createDefaultWireguardOutboundSettings,
  createDefaultOutboundSettings,
} from '@/lib/xray/outbound-defaults';
import {
  BlackholeOutboundSettingsSchema,
  DNSOutboundSettingsSchema,
  FreedomOutboundSettingsSchema,
  HttpOutboundSettingsSchema,
  HysteriaOutboundSettingsSchema,
  LoopbackOutboundSettingsSchema,
  ShadowsocksOutboundSettingsSchema,
  SocksOutboundSettingsSchema,
  TrojanOutboundSettingsSchema,
  VlessOutboundSettingsSchema,
  VmessOutboundSettingsSchema,
  WireguardOutboundSettingsSchema,
} from '@/schemas/protocols/outbound';

// Snapshot + Zod round-trip for each createDefault*OutboundSettings factory.
// The factory output mirrors the legacy `new Outbound.<X>Settings()` start
// state, so most required fields are empty stubs (address, port, password,
// id). Zod parsing happens AFTER patching the stubs with sensible values —
// this catches schema/factory drift without forcing the factory to invent
// data it shouldn't.

const SAMPLE_ID = '11111111-2222-4333-8444-555555555555';
const SAMPLE_ADDRESS = '1.2.3.4';
const SAMPLE_PORT = 443;
const SAMPLE_SECRET = 'abc123def456ghi789';

describe('outbound default factories: shape snapshots', () => {
  it('freedom is the empty object', () => {
    expect(createDefaultFreedomOutboundSettings()).toEqual({});
  });

  it('blackhole is the empty object', () => {
    expect(createDefaultBlackholeOutboundSettings()).toEqual({});
  });

  it('loopback has an empty inboundTag', () => {
    expect(createDefaultLoopbackOutboundSettings()).toEqual({ inboundTag: '' });
  });

  it('dns has the legacy constructor defaults', () => {
    expect(createDefaultDNSOutboundSettings()).toEqual({
      rewriteNetwork: '',
      rewriteAddress: '',
      rewritePort: 53,
      userLevel: 0,
      rules: [],
    });
  });

  it('vmess wraps a single vnext server with one user', () => {
    expect(createDefaultVmessOutboundSettings()).toEqual({
      vnext: [{ address: '', port: 443, users: [{ id: '', security: 'auto' }] }],
    });
  });

  it('vless lays the connect target flat', () => {
    expect(createDefaultVlessOutboundSettings()).toEqual({
      address: '',
      port: 443,
      id: '',
      flow: '',
      encryption: 'none',
    });
  });

  it('trojan wraps a single server', () => {
    expect(createDefaultTrojanOutboundSettings()).toEqual({
      servers: [{ address: '', port: 443, password: '' }],
    });
  });

  it('shadowsocks defaults to 2022-blake3-aes-128-gcm', () => {
    expect(createDefaultShadowsocksOutboundSettings()).toEqual({
      servers: [{
        address: '', port: 443, password: '', method: '2022-blake3-aes-128-gcm',
      }],
    });
  });

  it('socks defaults to port 1080 with no users', () => {
    expect(createDefaultSocksOutboundSettings()).toEqual({
      servers: [{ address: '', port: 1080, users: [] }],
    });
  });

  it('http defaults to port 8080 with no users', () => {
    expect(createDefaultHttpOutboundSettings()).toEqual({
      servers: [{ address: '', port: 8080, users: [] }],
    });
  });

  it('wireguard seeds secretKey deterministically when given', () => {
    const out = createDefaultWireguardOutboundSettings({ secretKey: SAMPLE_SECRET });
    expect(out.secretKey).toBe(SAMPLE_SECRET);
    expect(out.mtu).toBe(1420);
    expect(out.address).toEqual([]);
    expect(out.noKernelTun).toBe(false);
    expect(out.peers).toEqual([{
      publicKey: '', allowedIPs: ['0.0.0.0/0', '::/0'], endpoint: '',
    }]);
  });

  it('wireguard generates a secretKey when none is seeded', () => {
    const out = createDefaultWireguardOutboundSettings();
    expect(out.secretKey).toMatch(/^[A-Za-z0-9+/=]+$/);
    expect(out.secretKey.length).toBeGreaterThan(8);
  });

  it('hysteria defaults to port 443 version 2', () => {
    expect(createDefaultHysteriaOutboundSettings()).toEqual({
      address: '', port: 443, version: 2,
    });
  });
});

describe('outbound default factories: schema acceptance after stub fill-in', () => {
  it('freedom default parses (no required fields)', () => {
    expect(FreedomOutboundSettingsSchema.safeParse(
      createDefaultFreedomOutboundSettings(),
    ).success).toBe(true);
  });

  it('blackhole default parses (no required fields)', () => {
    expect(BlackholeOutboundSettingsSchema.safeParse(
      createDefaultBlackholeOutboundSettings(),
    ).success).toBe(true);
  });

  it('loopback default parses (no required fields)', () => {
    expect(LoopbackOutboundSettingsSchema.safeParse(
      createDefaultLoopbackOutboundSettings(),
    ).success).toBe(true);
  });

  it('dns default parses', () => {
    expect(DNSOutboundSettingsSchema.safeParse(
      createDefaultDNSOutboundSettings(),
    ).success).toBe(true);
  });

  it('vmess parses once vnext fields are filled', () => {
    const def = createDefaultVmessOutboundSettings();
    def.vnext[0].address = SAMPLE_ADDRESS;
    def.vnext[0].port = SAMPLE_PORT;
    def.vnext[0].users[0].id = SAMPLE_ID;
    expect(VmessOutboundSettingsSchema.safeParse(def).success).toBe(true);
  });

  it('vless parses once address/port/id are filled', () => {
    const def = createDefaultVlessOutboundSettings();
    def.address = SAMPLE_ADDRESS;
    def.port = SAMPLE_PORT;
    def.id = SAMPLE_ID;
    expect(VlessOutboundSettingsSchema.safeParse(def).success).toBe(true);
  });

  it('trojan parses once server fields are filled', () => {
    const def = createDefaultTrojanOutboundSettings();
    def.servers[0].address = SAMPLE_ADDRESS;
    def.servers[0].password = 'secret';
    expect(TrojanOutboundSettingsSchema.safeParse(def).success).toBe(true);
  });

  it('shadowsocks parses once server fields are filled', () => {
    const def = createDefaultShadowsocksOutboundSettings();
    def.servers[0].address = SAMPLE_ADDRESS;
    def.servers[0].password = 'secret';
    expect(ShadowsocksOutboundSettingsSchema.safeParse(def).success).toBe(true);
  });

  it('socks parses once address is filled', () => {
    const def = createDefaultSocksOutboundSettings();
    def.servers[0].address = SAMPLE_ADDRESS;
    expect(SocksOutboundSettingsSchema.safeParse(def).success).toBe(true);
  });

  it('http parses once address is filled', () => {
    const def = createDefaultHttpOutboundSettings();
    def.servers[0].address = SAMPLE_ADDRESS;
    expect(HttpOutboundSettingsSchema.safeParse(def).success).toBe(true);
  });

  it('wireguard parses once peer + secretKey are filled', () => {
    const def = createDefaultWireguardOutboundSettings({ secretKey: SAMPLE_SECRET });
    def.peers[0].publicKey = 'pk';
    def.peers[0].endpoint = `${SAMPLE_ADDRESS}:51820`;
    expect(WireguardOutboundSettingsSchema.safeParse(def).success).toBe(true);
  });

  it('hysteria parses once address is filled', () => {
    const def = createDefaultHysteriaOutboundSettings();
    def.address = SAMPLE_ADDRESS;
    expect(HysteriaOutboundSettingsSchema.safeParse(def).success).toBe(true);
  });
});

describe('createDefaultOutboundSettings dispatcher', () => {
  const PROTOCOLS = [
    'freedom', 'blackhole', 'dns', 'vmess', 'vless', 'trojan', 'shadowsocks',
    'socks', 'http', 'wireguard', 'hysteria', 'loopback',
  ];

  for (const protocol of PROTOCOLS) {
    it(`returns non-null for ${protocol}`, () => {
      expect(createDefaultOutboundSettings(protocol)).not.toBeNull();
    });
  }

  it('returns null for an unknown protocol', () => {
    expect(createDefaultOutboundSettings('mysterious')).toBeNull();
  });
});
