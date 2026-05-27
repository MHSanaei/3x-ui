import { describe, expect, it } from 'vitest';

import { inboundFromDb } from '@/lib/xray/inbound-from-db';
import {
  genAllLinks,
  genInboundLinks,
  genWireguardConfigs,
  genWireguardLinks,
  getInboundClients,
} from '@/lib/xray/inbound-link';
import {
  canEnableTlsFlow,
  isSS2022,
  isSSMultiUser,
} from '@/lib/xray/protocol-capabilities';

const FALLBACK_HOST = 'panel.example.test';

const BASE_DB_FIELDS = {
  port: 12345,
  listen: '',
  tag: '',
  remark: 'unit',
  enable: true,
  expiryTime: 0,
  up: 0,
  down: 0,
  total: 0,
  sniffing: '',
};

describe('inboundFromDb', () => {
  it('coerces JSON-string settings into a parsed object', () => {
    const raw = {
      ...BASE_DB_FIELDS,
      protocol: 'vless',
      settings: JSON.stringify({
        clients: [{ id: 'abc', email: 'a@test', flow: '' }],
        decryption: 'none',
      }),
      streamSettings: JSON.stringify({ network: 'tcp', security: 'none' }),
    };
    const inbound = inboundFromDb(raw);
    expect(inbound.protocol).toBe('vless');
    expect(inbound.port).toBe(12345);
    expect((inbound.settings as { decryption?: string }).decryption).toBe('none');
    expect((inbound.streamSettings as { network?: string })?.network).toBe('tcp');
  });

  it('fills schema defaults onto partial object settings', () => {
    const settings = { clients: [], decryption: 'none' };
    const raw = {
      ...BASE_DB_FIELDS,
      protocol: 'vless',
      settings,
      streamSettings: { network: 'ws', security: 'tls' },
    };
    const inbound = inboundFromDb(raw);
    // encryption/fallbacks defaulted by schema, original settings ref not preserved
    expect(inbound.settings).not.toBe(settings);
    expect((inbound.settings as { encryption?: string }).encryption).toBe('none');
    expect((inbound.streamSettings as { security?: string })?.security).toBe('tls');
  });

  it('returns schema-default settings for missing/empty fields without throwing', () => {
    const raw = {
      ...BASE_DB_FIELDS,
      protocol: 'http',
      settings: '',
      streamSettings: '',
      sniffing: '',
    };
    const inbound = inboundFromDb(raw);
    // http settings has its own schema defaults (accounts: [], allowTransparent: false)
    expect(inbound.settings).toEqual(expect.objectContaining({ accounts: [] }));
    expect(inbound.streamSettings).toEqual({});
    expect(inbound.sniffing).toEqual({});
  });

  it('feeds genInboundLinks for vless without throwing', () => {
    const raw = {
      ...BASE_DB_FIELDS,
      protocol: 'vless',
      settings: {
        clients: [{ id: '8c14d6f7-2e3b-4a91-9d24-3f7a6b8c1e02', email: 'alice@test', flow: '' }],
        decryption: 'none',
      },
      streamSettings: { network: 'tcp', security: 'none' },
    };
    const inbound = inboundFromDb(raw);
    const links = genInboundLinks({
      inbound,
      remark: 'unit',
      fallbackHostname: FALLBACK_HOST,
    });
    expect(links).toContain('vless://');
    expect(links).toContain('encryption=none');
  });

  it('feeds genWireguardConfigs + genWireguardLinks for wireguard peers', () => {
    const raw = {
      ...BASE_DB_FIELDS,
      protocol: 'wireguard',
      settings: {
        mtu: 1420,
        secretKey: 'QGVlb2dXc1ZTWGw0ZXBzZndsWmtMaUM5MUlNYjBHWFdYbz0=',
        peers: [
          {
            privateKey: 'iJ2cBkrSGqRwIfYIDIxk7hr5RXfdR93MfJUL7yqkkH8=',
            publicKey: 'DGSYIcEKAUkA7HhzGSjxLZuV67BR3LeyU0BMLJzNVHQ=',
            allowedIPs: ['10.0.0.2/32'],
            keepAlive: 25,
          },
        ],
        noKernelTun: false,
      },
      streamSettings: '',
    };
    const inbound = inboundFromDb(raw);
    const configs = genWireguardConfigs({ inbound, remark: 'wg', fallbackHostname: FALLBACK_HOST });
    expect(configs).toContain('[Interface]');
    expect(configs).toContain('[Peer]');
    const links = genWireguardLinks({ inbound, remark: 'wg', fallbackHostname: FALLBACK_HOST });
    expect(links).toMatch(/^wireguard:\/\//);
  });

  it('feeds genAllLinks per client', () => {
    const raw = {
      ...BASE_DB_FIELDS,
      protocol: 'trojan',
      settings: {
        clients: [
          { password: 'pw1', email: 'one@test' },
          { password: 'pw2', email: 'two@test' },
        ],
      },
      streamSettings: { network: 'tcp', security: 'tls', tlsSettings: { serverName: 'example.test' } },
    };
    const inbound = inboundFromDb(raw);
    const entries = genAllLinks({
      inbound,
      remark: 'trojan',
      client: { password: 'pw1', email: 'one@test' },
      fallbackHostname: FALLBACK_HOST,
    });
    expect(entries.length).toBeGreaterThan(0);
    expect(entries[0].link).toContain('trojan://');
  });
});

describe('protocol-capability helpers with raw coerced shapes', () => {
  it('isSSMultiUser returns true for legacy SS methods', () => {
    expect(isSSMultiUser({ protocol: 'shadowsocks', settings: { method: 'aes-256-gcm' } })).toBe(true);
    expect(isSSMultiUser({ protocol: 'shadowsocks', settings: { method: '2022-blake3-aes-128-gcm' } })).toBe(true);
  });

  it('isSSMultiUser returns false for single-user blake3-chacha20 method', () => {
    expect(isSSMultiUser({
      protocol: 'shadowsocks',
      settings: { method: '2022-blake3-chacha20-poly1305' },
    })).toBe(false);
  });

  it('isSS2022 detects 2022-blake3 family', () => {
    expect(isSS2022({ protocol: 'shadowsocks', settings: { method: '2022-blake3-aes-128-gcm' } })).toBe(true);
    expect(isSS2022({ protocol: 'shadowsocks', settings: { method: 'aes-256-gcm' } })).toBe(false);
  });

  it('canEnableTlsFlow gates on vless + tcp + tls/reality', () => {
    expect(canEnableTlsFlow({
      protocol: 'vless',
      streamSettings: { network: 'tcp', security: 'tls' },
    })).toBe(true);
    expect(canEnableTlsFlow({
      protocol: 'vless',
      streamSettings: { network: 'ws', security: 'tls' },
    })).toBe(false);
    expect(canEnableTlsFlow({
      protocol: 'vmess',
      streamSettings: { network: 'tcp', security: 'tls' },
    })).toBe(false);
  });
});

describe('getInboundClients with schema-shaped inbound', () => {
  it('returns clients array for vless/vmess/trojan/hysteria', () => {
    const inbound = inboundFromDb({
      ...BASE_DB_FIELDS,
      protocol: 'vless',
      settings: { clients: [{ id: 'x', email: 'e@test' }], decryption: 'none' },
      streamSettings: { network: 'tcp', security: 'none' },
    });
    expect(getInboundClients(inbound)).toHaveLength(1);
  });

  it('returns null for SS single-user', () => {
    const inbound = inboundFromDb({
      ...BASE_DB_FIELDS,
      protocol: 'shadowsocks',
      settings: { method: '2022-blake3-chacha20-poly1305', password: 'pw', clients: [] },
      streamSettings: { network: 'tcp', security: 'none' },
    });
    expect(getInboundClients(inbound)).toBeNull();
  });

  it('returns null for non-client protocols (http/mixed/tun/tunnel)', () => {
    for (const protocol of ['http', 'mixed', 'tun', 'tunnel']) {
      const inbound = inboundFromDb({
        ...BASE_DB_FIELDS,
        protocol,
        settings: {},
        streamSettings: '',
      });
      expect(getInboundClients(inbound)).toBeNull();
    }
  });
});
