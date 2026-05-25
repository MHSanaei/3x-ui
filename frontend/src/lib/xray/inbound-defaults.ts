import { RandomUtil, Wireguard } from '@/utils';

import type { HttpInboundSettings } from '@/schemas/protocols/inbound/http';
import type { Hysteria2InboundSettings } from '@/schemas/protocols/inbound/hysteria2';
import type { HysteriaClient, HysteriaInboundSettings } from '@/schemas/protocols/inbound/hysteria';
import type { MixedInboundSettings } from '@/schemas/protocols/inbound/mixed';
import type { ShadowsocksClient, ShadowsocksInboundSettings } from '@/schemas/protocols/inbound/shadowsocks';
import type { TrojanClient, TrojanInboundSettings } from '@/schemas/protocols/inbound/trojan';
import type { TunnelInboundSettings } from '@/schemas/protocols/inbound/tunnel';
import type { VlessClient, VlessInboundSettings } from '@/schemas/protocols/inbound/vless';
import type { VmessClient, VmessInboundSettings } from '@/schemas/protocols/inbound/vmess';
import type { WireguardInboundSettings } from '@/schemas/protocols/inbound/wireguard';

// Plain-object factories for protocol clients. Each returns a Zod-parsable
// object matching the wire shape. Random fields (id, password, auth,
// email, subId) call RandomUtil at invocation time — pass them in
// `overrides` for deterministic tests or for forms that pre-seed values.
//
// These replace the legacy `new Inbound.<Settings>.<Client>()` constructors
// and the Inbound.ClientBase machinery. Callers no longer carry the
// XrayCommonClass dependency once the swap lands.

interface ClientBaseSeed {
  email?: string;
  subId?: string;
  limitIp?: number;
  totalGB?: number;
  expiryTime?: number;
  enable?: boolean;
  tgId?: number;
  comment?: string;
  reset?: number;
}

interface ClientBase {
  email: string;
  limitIp: number;
  totalGB: number;
  expiryTime: number;
  enable: boolean;
  tgId: number;
  subId: string;
  comment: string;
  reset: number;
}

function clientBase(seed: ClientBaseSeed = {}): ClientBase {
  return {
    email: seed.email ?? RandomUtil.randomLowerAndNum(8),
    limitIp: seed.limitIp ?? 0,
    totalGB: seed.totalGB ?? 0,
    expiryTime: seed.expiryTime ?? 0,
    enable: seed.enable ?? true,
    tgId: seed.tgId ?? 0,
    subId: seed.subId ?? RandomUtil.randomLowerAndNum(16),
    comment: seed.comment ?? '',
    reset: seed.reset ?? 0,
  };
}

export interface VlessClientSeed extends ClientBaseSeed {
  id?: string;
  flow?: VlessClient['flow'];
}

export function createDefaultVlessClient(seed: VlessClientSeed = {}): VlessClient {
  return {
    id: seed.id ?? RandomUtil.randomUUID(),
    flow: seed.flow ?? '',
    ...clientBase(seed),
  };
}

export interface VmessClientSeed extends ClientBaseSeed {
  id?: string;
  security?: VmessClient['security'];
}

export function createDefaultVmessClient(seed: VmessClientSeed = {}): VmessClient {
  return {
    id: seed.id ?? RandomUtil.randomUUID(),
    security: seed.security ?? 'auto',
    ...clientBase(seed),
  };
}

export interface TrojanClientSeed extends ClientBaseSeed {
  password?: string;
}

export function createDefaultTrojanClient(seed: TrojanClientSeed = {}): TrojanClient {
  return {
    password: seed.password ?? RandomUtil.randomSeq(10),
    ...clientBase(seed),
  };
}

export interface ShadowsocksClientSeed extends ClientBaseSeed {
  method?: string;
  password?: string;
  ssMethod?: string;
}

// Shadowsocks clients ship with an empty `method` on single-user inbounds
// (the parent inbound's method is authoritative); only 2022-blake3 multi-
// user inbounds use the per-client method. Callers pass `ssMethod` to seed
// a method-specific password length when creating a multi-user client.
export function createDefaultShadowsocksClient(seed: ShadowsocksClientSeed = {}): ShadowsocksClient {
  const method = seed.method ?? '';
  const password = seed.password ?? RandomUtil.randomShadowsocksPassword(seed.ssMethod ?? '2022-blake3-aes-256-gcm');
  return {
    method,
    password,
    ...clientBase(seed),
  };
}

export interface HysteriaClientSeed extends ClientBaseSeed {
  auth?: string;
}

export function createDefaultHysteriaClient(seed: HysteriaClientSeed = {}): HysteriaClient {
  return {
    auth: seed.auth ?? RandomUtil.randomSeq(10),
    ...clientBase(seed),
  };
}

// Inbound-settings factories. Each returns a Zod-parsable wire-shape with
// schema defaults already applied — no class instance, no XrayCommonClass.
// Callers (form modals via Step 4, InboundsPage clone via Step 5) call
// these instead of the legacy `Inbound.Settings.getSettings(protocol)`.

export function createDefaultVlessInboundSettings(): VlessInboundSettings {
  return {
    clients: [],
    decryption: 'none',
    encryption: 'none',
    fallbacks: [],
  };
}

export function createDefaultVmessInboundSettings(): VmessInboundSettings {
  return { clients: [] };
}

export function createDefaultTrojanInboundSettings(): TrojanInboundSettings {
  return { clients: [], fallbacks: [] };
}

export interface ShadowsocksInboundSeed {
  method?: ShadowsocksInboundSettings['method'];
  password?: string;
  network?: ShadowsocksInboundSettings['network'];
  ivCheck?: boolean;
}

export function createDefaultShadowsocksInboundSettings(
  seed: ShadowsocksInboundSeed = {},
): ShadowsocksInboundSettings {
  const method = seed.method ?? '2022-blake3-aes-256-gcm';
  return {
    method,
    password: seed.password ?? RandomUtil.randomShadowsocksPassword(method),
    network: seed.network ?? 'tcp',
    clients: [],
    ivCheck: seed.ivCheck ?? false,
  };
}

// Hysteria v1 defaults still emit `version: 2` to match the legacy panel
// constructor — the field discriminates v1 vs v2 inside the same settings
// shape. Callers that explicitly want v1 pass `{ version: 1 }`.
export interface HysteriaInboundSeed {
  version?: number;
}

export function createDefaultHysteriaInboundSettings(
  seed: HysteriaInboundSeed = {},
): HysteriaInboundSettings {
  return {
    version: seed.version ?? 2,
    clients: [],
  };
}

export function createDefaultHysteria2InboundSettings(): Hysteria2InboundSettings {
  return { version: 2, clients: [] };
}

export function createDefaultHttpInboundSettings(): HttpInboundSettings {
  return { accounts: [], allowTransparent: false };
}

export function createDefaultMixedInboundSettings(): MixedInboundSettings {
  return {
    auth: 'password',
    accounts: [],
    udp: false,
    ip: '127.0.0.1',
  };
}

export function createDefaultTunnelInboundSettings(): TunnelInboundSettings {
  return {
    portMap: {},
    allowedNetwork: 'tcp,udp',
    followRedirect: false,
  };
}

export interface WireguardInboundSeed {
  mtu?: number;
  secretKey?: string;
  noKernelTun?: boolean;
}

export function createDefaultWireguardInboundSettings(
  seed: WireguardInboundSeed = {},
): WireguardInboundSettings {
  return {
    mtu: seed.mtu ?? 1420,
    secretKey: seed.secretKey ?? Wireguard.generateKeypair().privateKey,
    peers: [],
    noKernelTun: seed.noKernelTun ?? false,
  };
}
