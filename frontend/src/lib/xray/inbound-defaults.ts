import { RandomUtil } from '@/utils';

import type { HysteriaClient } from '@/schemas/protocols/inbound/hysteria';
import type { ShadowsocksClient } from '@/schemas/protocols/inbound/shadowsocks';
import type { TrojanClient } from '@/schemas/protocols/inbound/trojan';
import type { VlessClient } from '@/schemas/protocols/inbound/vless';
import type { VmessClient } from '@/schemas/protocols/inbound/vmess';

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
