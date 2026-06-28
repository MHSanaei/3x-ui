import { RandomUtil, Wireguard } from '@/utils';

import type { HttpInboundSettings } from '@/schemas/protocols/inbound/http';
import type { HysteriaClient, HysteriaInboundSettings } from '@/schemas/protocols/inbound/hysteria';
import type { MixedInboundSettings } from '@/schemas/protocols/inbound/mixed';
import type { MtprotoInboundSettings } from '@/schemas/protocols/inbound/mtproto';
import type { ShadowsocksClient, ShadowsocksInboundSettings } from '@/schemas/protocols/inbound/shadowsocks';
import type { TrojanClient, TrojanInboundSettings } from '@/schemas/protocols/inbound/trojan';
import type { TunInboundSettings } from '@/schemas/protocols/inbound/tun';
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
    email: seed.email ?? RandomUtil.randomLowerAndNum(10),
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
    alterId: 0,
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
    network: seed.network ?? 'tcp,udp',
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

export function createDefaultHttpInboundSettings(): HttpInboundSettings {
  return {
    accounts: [{ user: RandomUtil.randomLowerAndNum(8), pass: RandomUtil.randomLowerAndNum(12) }],
    allowTransparent: false,
  };
}

export function createDefaultMixedInboundSettings(): MixedInboundSettings {
  return {
    auth: 'password',
    accounts: [{ user: RandomUtil.randomLowerAndNum(8), pass: RandomUtil.randomLowerAndNum(12) }],
    udp: false,
    ip: '127.0.0.1',
  };
}

function domainToHex(domain: string): string {
  return Array.from(new TextEncoder().encode(domain))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}

// generateMtprotoSecret builds an "ee" FakeTLS secret: the marker, 16 random
// bytes (32 hex chars), then the domain encoded as hex. Mirrors the Go
// model.GenerateFakeTLSSecret; the backend re-derives it on save so this is
// only for immediate display in the form.
export function generateMtprotoSecret(domain: string): string {
  return `ee${RandomUtil.randomSeq(32, { type: 'hex' })}${domainToHex(domain)}`;
}

// mtprotoSecretForDomain rewrites only the domain suffix of an existing secret,
// preserving its 16-byte random middle when valid (generating one otherwise).
// Mirrors the Go model.HealMtprotoSecret so editing the FakeTLS domain doesn't
// needlessly rotate the secret's identity.
export function mtprotoSecretForDomain(currentSecret: string, domain: string): string {
  let body = currentSecret;
  if (body.startsWith('ee') || body.startsWith('dd')) {
    body = body.slice(2);
  }
  const middle = /^[0-9a-f]{32}/i.test(body)
    ? body.slice(0, 32)
    : RandomUtil.randomSeq(32, { type: 'hex' });
  return `ee${middle}${domainToHex(domain)}`;
}

export function createDefaultMtprotoInboundSettings(): MtprotoInboundSettings {
  const fakeTlsDomain = 'www.cloudflare.com';
  return {
    fakeTlsDomain,
    secret: generateMtprotoSecret(fakeTlsDomain),
  };
}

export function createDefaultTunnelInboundSettings(): TunnelInboundSettings {
  return {
    portMap: {},
    allowedNetwork: 'tcp,udp',
    followRedirect: false,
  };
}

export function createDefaultTunInboundSettings(): TunInboundSettings {
  return {
    name: 'xray0',
    mtu: 1500,
    gateway: [],
    dns: [],
    userLevel: 0,
    autoSystemRoutingTable: [],
    autoOutboundsInterface: 'auto',
  };
}

export interface WireguardInboundSeed {
  mtu?: number;
  secretKey?: string;
  noKernelTun?: boolean;
}

// WireGuard is multi-client now: a new inbound holds only the server identity
// (secretKey/mtu) and starts with no clients. Clients (peers) are added later
// through the client modal, which generates each one's keypair and a unique
// tunnel address. peers stays empty for backward-compatible parsing.
export function createDefaultWireguardInboundSettings(
  seed: WireguardInboundSeed = {},
): WireguardInboundSettings {
  return {
    mtu: seed.mtu ?? 1420,
    secretKey: seed.secretKey ?? Wireguard.generateKeypair().privateKey,
    peers: [],
    clients: [],
    noKernelTun: seed.noKernelTun ?? false,
  };
}

// Protocol-aware dispatch over every inbound-settings factory. Mirrors
// the legacy `Inbound.Settings.getSettings(protocol)` dispatcher, but
// returns a plain Zod-parsable object instead of a class instance.
// Callers swapping off the class hierarchy use this in place of
// `getSettings(p)` + `.toJson()`.
export type AnyInboundSettings =
  | VlessInboundSettings
  | VmessInboundSettings
  | TrojanInboundSettings
  | ShadowsocksInboundSettings
  | HysteriaInboundSettings
  | HttpInboundSettings
  | MixedInboundSettings
  | TunInboundSettings
  | TunnelInboundSettings
  | WireguardInboundSettings
  | MtprotoInboundSettings;

export function createDefaultInboundSettings(protocol: string): AnyInboundSettings | null {
  switch (protocol) {
    case 'vless':       return createDefaultVlessInboundSettings();
    case 'vmess':       return createDefaultVmessInboundSettings();
    case 'trojan':      return createDefaultTrojanInboundSettings();
    case 'shadowsocks': return createDefaultShadowsocksInboundSettings();
    case 'hysteria':    return createDefaultHysteriaInboundSettings();
    case 'http':        return createDefaultHttpInboundSettings();
    case 'mixed':       return createDefaultMixedInboundSettings();
    case 'tunnel':      return createDefaultTunnelInboundSettings();
    case 'tun':         return createDefaultTunInboundSettings();
    case 'wireguard':   return createDefaultWireguardInboundSettings();
    case 'mtproto':     return createDefaultMtprotoInboundSettings();
    default:            return null;
  }
}
