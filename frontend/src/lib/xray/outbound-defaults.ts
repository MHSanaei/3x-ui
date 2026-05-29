import { RandomUtil, Wireguard } from '@/utils';

import type { BlackholeOutboundSettings } from '@/schemas/protocols/outbound/blackhole';
import type { DNSOutboundSettings } from '@/schemas/protocols/outbound/dns';
import type { FreedomOutboundSettings } from '@/schemas/protocols/outbound/freedom';
import type { HttpOutboundSettings } from '@/schemas/protocols/outbound/http';
import type { HysteriaOutboundSettings } from '@/schemas/protocols/outbound/hysteria';
import type { LoopbackOutboundSettings } from '@/schemas/protocols/outbound/loopback';
import type { ShadowsocksOutboundSettings } from '@/schemas/protocols/outbound/shadowsocks';
import type { SocksOutboundSettings } from '@/schemas/protocols/outbound/socks';
import type { TrojanOutboundSettings } from '@/schemas/protocols/outbound/trojan';
import type { VlessOutboundSettings } from '@/schemas/protocols/outbound/vless';
import type { VmessOutboundSettings } from '@/schemas/protocols/outbound/vmess';
import type { WireguardOutboundSettings } from '@/schemas/protocols/outbound/wireguard';

// Plain-object factories mirroring `new Outbound.<X>Settings()` from the
// legacy class hierarchy, then `.toJson()`. The output matches the wire
// shape — the same starting state the OutboundFormModal's `ob.settings`
// holds the first time the user picks a protocol.
//
// Required-by-schema fields the legacy class leaves undefined (address,
// port, user-supplied ids/passwords) become empty stubs here. Zod will
// reject the default output until the user fills them in via the form;
// this is intentional and matches the legacy "scaffold object" behavior.

export function createDefaultFreedomOutboundSettings(): FreedomOutboundSettings {
  return {};
}

export function createDefaultBlackholeOutboundSettings(): BlackholeOutboundSettings {
  return {};
}

export function createDefaultLoopbackOutboundSettings(): LoopbackOutboundSettings {
  return { inboundTag: '' };
}

export function createDefaultDNSOutboundSettings(): DNSOutboundSettings {
  return {
    rewriteNetwork: '',
    rewriteAddress: '',
    rewritePort: 53,
    userLevel: 0,
    rules: [],
  };
}

export function createDefaultVmessOutboundSettings(): VmessOutboundSettings {
  return {
    vnext: [{
      address: '',
      port: 443,
      users: [{ id: '', security: 'auto' }],
    }],
  };
}

export function createDefaultVlessOutboundSettings(): VlessOutboundSettings {
  return {
    address: '',
    port: 443,
    id: '',
    flow: '',
    encryption: 'none',
  };
}

export function createDefaultTrojanOutboundSettings(): TrojanOutboundSettings {
  return {
    servers: [{ address: '', port: 443, password: '' }],
  };
}

// Why: legacy constructor leaves method undefined; the form's Select
// snaps to the first option when the user opens it. We pick the same
// modern default the inbound shadowsocks factory uses
// (2022-blake3-aes-128-gcm) so the OutboundFormModal renders a coherent
// initial state instead of an empty Select.
export function createDefaultShadowsocksOutboundSettings(): ShadowsocksOutboundSettings {
  return {
    servers: [{
      address: '',
      port: 443,
      password: '',
      method: '2022-blake3-aes-128-gcm',
    }],
  };
}

export function createDefaultSocksOutboundSettings(): SocksOutboundSettings {
  return {
    servers: [{ address: '', port: 1080, users: [] }],
  };
}

export function createDefaultHttpOutboundSettings(): HttpOutboundSettings {
  return {
    servers: [{ address: '', port: 8080, users: [] }],
  };
}

interface WireguardOutboundSeed {
  secretKey?: string;
}

export function createDefaultWireguardOutboundSettings(
  seed: WireguardOutboundSeed = {},
): WireguardOutboundSettings {
  const secretKey = seed.secretKey ?? Wireguard.generateKeypair().privateKey;
  return {
    mtu: 1420,
    secretKey,
    address: [],
    workers: 2,
    peers: [{
      publicKey: '',
      allowedIPs: ['0.0.0.0/0', '::/0'],
      endpoint: '',
    }],
    noKernelTun: false,
  };
}

export function createDefaultHysteriaOutboundSettings(): HysteriaOutboundSettings {
  return { address: '', port: 443, version: 2 };
}

export type AnyOutboundSettings =
  | BlackholeOutboundSettings
  | DNSOutboundSettings
  | FreedomOutboundSettings
  | HttpOutboundSettings
  | HysteriaOutboundSettings
  | LoopbackOutboundSettings
  | ShadowsocksOutboundSettings
  | SocksOutboundSettings
  | TrojanOutboundSettings
  | VlessOutboundSettings
  | VmessOutboundSettings
  | WireguardOutboundSettings;

// Protocol-aware dispatch. Mirrors the legacy
// `Outbound.Settings.getSettings(protocol)` switch. Note: the inbound
// dispatcher returns `null` for unknown protocols and so does this one,
// keeping the contract identical so callers can stay protocol-agnostic.
//
// The `RandomUtil` reference is held to silence unused-import warnings
// when no per-call randomization happens at the dispatcher level —
// individual factories may pull from it via their own seeds.
export function createDefaultOutboundSettings(protocol: string): AnyOutboundSettings | null {
  void RandomUtil;
  switch (protocol) {
    case 'freedom':     return createDefaultFreedomOutboundSettings();
    case 'blackhole':   return createDefaultBlackholeOutboundSettings();
    case 'dns':         return createDefaultDNSOutboundSettings();
    case 'vmess':       return createDefaultVmessOutboundSettings();
    case 'vless':       return createDefaultVlessOutboundSettings();
    case 'trojan':      return createDefaultTrojanOutboundSettings();
    case 'shadowsocks': return createDefaultShadowsocksOutboundSettings();
    case 'socks':       return createDefaultSocksOutboundSettings();
    case 'http':        return createDefaultHttpOutboundSettings();
    case 'wireguard':   return createDefaultWireguardOutboundSettings();
    case 'hysteria':    return createDefaultHysteriaOutboundSettings();
    case 'loopback':    return createDefaultLoopbackOutboundSettings();
    default:            return null;
  }
}
