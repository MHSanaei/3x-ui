import { describe, expect, it } from 'vitest';

import {
  createDefaultHttpInboundSettings,
  createDefaultHysteriaClient,
  createDefaultHysteriaInboundSettings,
  createDefaultMixedInboundSettings,
  createDefaultShadowsocksClient,
  createDefaultShadowsocksInboundSettings,
  createDefaultTrojanClient,
  createDefaultTrojanInboundSettings,
  createDefaultTunnelInboundSettings,
  createDefaultVlessClient,
  createDefaultVlessInboundSettings,
  createDefaultVmessClient,
  createDefaultVmessInboundSettings,
  createDefaultWireguardInboundSettings,
} from '@/lib/xray/inbound-defaults';
import { createHysteriaTlsSettingsWithDefaultCert } from '@/lib/xray/inbound-tls-defaults';
import { HttpInboundSettingsSchema } from '@/schemas/protocols/inbound/http';
import { HysteriaClientSchema, HysteriaInboundSettingsSchema } from '@/schemas/protocols/inbound/hysteria';
import { MixedInboundSettingsSchema } from '@/schemas/protocols/inbound/mixed';
import { ShadowsocksClientSchema, ShadowsocksInboundSettingsSchema } from '@/schemas/protocols/inbound/shadowsocks';
import { TrojanClientSchema, TrojanInboundSettingsSchema } from '@/schemas/protocols/inbound/trojan';
import { TunnelInboundSettingsSchema } from '@/schemas/protocols/inbound/tunnel';
import { VlessClientSchema, VlessInboundSettingsSchema } from '@/schemas/protocols/inbound/vless';
import { VmessClientSchema, VmessInboundSettingsSchema } from '@/schemas/protocols/inbound/vmess';
import { WireguardInboundSettingsSchema } from '@/schemas/protocols/inbound/wireguard';

// Tests pass explicit seeds for every random field so the assertions don't
// depend on window.crypto (the node test env has no crypto.randomUUID).
// Each factory is verified two ways:
//   1. snapshot — locks the exact shape
//   2. Zod parse round-trip — confirms the factory output is a valid
//      member of the protocol's client schema (no missing defaults, no
//      stray fields)

const seed = {
  email: 'fixture@example.test',
  subId: 'fixed-sub-id-1234',
};

describe('createDefaultVlessClient', () => {
  it('produces a Zod-valid client', () => {
    const c = createDefaultVlessClient({ ...seed, id: '11111111-2222-4333-8444-555555555555' });
    expect(c).toMatchSnapshot();
    expect(VlessClientSchema.parse(c)).toEqual(c);
  });
});

describe('createDefaultVmessClient', () => {
  it('produces a Zod-valid client', () => {
    const c = createDefaultVmessClient({ ...seed, id: 'aaaaaaaa-bbbb-4ccc-9ddd-eeeeeeeeeeee' });
    expect(c).toMatchSnapshot();
    expect(VmessClientSchema.parse(c)).toEqual(c);
  });
});

describe('createDefaultTrojanClient', () => {
  it('produces a Zod-valid client', () => {
    const c = createDefaultTrojanClient({ ...seed, password: 'fixed-trojan-pw' });
    expect(c).toMatchSnapshot();
    expect(TrojanClientSchema.parse(c)).toEqual(c);
  });
});

describe('createDefaultShadowsocksClient', () => {
  it('produces a Zod-valid client', () => {
    const c = createDefaultShadowsocksClient({ ...seed, password: 'ZmFrZS1zcy1wYXNzd29yZA==' });
    expect(c).toMatchSnapshot();
    expect(ShadowsocksClientSchema.parse(c)).toEqual(c);
  });
});

describe('createDefaultHysteriaClient', () => {
  it('produces a Zod-valid client', () => {
    const c = createDefaultHysteriaClient({ ...seed, auth: 'fixed-hyst-auth' });
    expect(c).toMatchSnapshot();
    expect(HysteriaClientSchema.parse(c)).toEqual(c);
  });
});

describe('createDefault*InboundSettings factories', () => {
  it('vless', () => {
    const s = createDefaultVlessInboundSettings();
    expect(s).toMatchSnapshot();
    expect(VlessInboundSettingsSchema.parse(s)).toEqual(s);
  });

  it('vmess', () => {
    const s = createDefaultVmessInboundSettings();
    expect(s).toMatchSnapshot();
    expect(VmessInboundSettingsSchema.parse(s)).toEqual(s);
  });

  it('trojan', () => {
    const s = createDefaultTrojanInboundSettings();
    expect(s).toMatchSnapshot();
    expect(TrojanInboundSettingsSchema.parse(s)).toEqual(s);
  });

  it('shadowsocks', () => {
    const s = createDefaultShadowsocksInboundSettings({ password: 'ZmFrZS1zcy1zZWVk' });
    expect(s).toMatchSnapshot();
    expect(ShadowsocksInboundSettingsSchema.parse(s)).toEqual(s);
  });

  it('hysteria (v1, defaults to v2 wire version)', () => {
    const s = createDefaultHysteriaInboundSettings();
    expect(s).toMatchSnapshot();
    expect(HysteriaInboundSettingsSchema.parse(s)).toEqual(s);
  });

  it('http', () => {
    const s = createDefaultHttpInboundSettings();
    expect(s.allowTransparent).toBe(false);
    const accounts = s.accounts ?? [];
    expect(accounts).toHaveLength(1);
    expect(accounts[0].user.length).toBe(8);
    expect(accounts[0].pass.length).toBe(12);
    expect(HttpInboundSettingsSchema.parse(s)).toEqual(s);
  });

  it('mixed', () => {
    const s = createDefaultMixedInboundSettings();
    expect(s.auth).toBe('password');
    expect(s.udp).toBe(false);
    expect(s.ip).toBe('127.0.0.1');
    const accounts = s.accounts ?? [];
    expect(accounts).toHaveLength(1);
    expect(accounts[0].user.length).toBe(8);
    expect(accounts[0].pass.length).toBe(12);
    expect(MixedInboundSettingsSchema.parse(s)).toEqual(s);
  });

  it('tunnel', () => {
    const s = createDefaultTunnelInboundSettings();
    expect(s).toMatchSnapshot();
    expect(TunnelInboundSettingsSchema.parse(s)).toEqual(s);
  });

  it('wireguard', () => {
    const s = createDefaultWireguardInboundSettings({
      secretKey: 'QGVlb2dXc1ZTWGw0ZXBzZndsWmtMaUM5MUlNYjBHWFdYbz0=',
    });
    expect(s).toMatchSnapshot();
    expect(WireguardInboundSettingsSchema.parse(s)).toEqual(s);
    expect(s.peers).toEqual([]);
    expect(s.clients).toEqual([]);
  });
});

describe('createHysteriaTlsSettingsWithDefaultCert', () => {
  it('defaults Hysteria TLS to uTLS None and h3 ALPN', () => {
    const tls = createHysteriaTlsSettingsWithDefaultCert();
    expect(tls.alpn).toEqual(['h3']);
    expect((tls.settings as Record<string, unknown>).fingerprint).toBe('');
    expect(tls.certificates).toEqual([
      expect.objectContaining({
        useFile: true,
        certificateFile: '',
        keyFile: '',
      }),
    ]);
  });
});
