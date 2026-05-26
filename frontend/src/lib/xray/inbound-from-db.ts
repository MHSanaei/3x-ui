import type { Inbound } from '@/schemas/api/inbound';
import { InboundSettingsSchema } from '@/schemas/protocols/inbound';
import {
  GrpcStreamSettingsSchema,
  HttpUpgradeStreamSettingsSchema,
  HysteriaStreamSettingsSchema,
  KcpStreamSettingsSchema,
  TcpStreamSettingsSchema,
  WsStreamSettingsSchema,
  XHttpStreamSettingsSchema,
} from '@/schemas/protocols/stream';
import {
  RealityStreamSettingsSchema,
  TlsStreamSettingsSchema,
} from '@/schemas/protocols/security';
import { coerceInboundJsonField } from '@/models/dbinbound';

export interface DbInboundLike {
  port: number;
  listen: string;
  protocol: string;
  settings: unknown;
  streamSettings: unknown;
  sniffing: unknown;
  tag?: string;
  remark?: string;
  enable?: boolean;
  expiryTime?: number;
  up?: number;
  down?: number;
  total?: number;
}

const NETWORK_KEY_MAP = {
  tcp: 'tcpSettings',
  kcp: 'kcpSettings',
  ws: 'wsSettings',
  grpc: 'grpcSettings',
  httpupgrade: 'httpupgradeSettings',
  xhttp: 'xhttpSettings',
  hysteria: 'hysteriaSettings',
} as const;

type SchemaWithParse = { safeParse: (v: unknown) => { success: boolean; data?: unknown } };

function parseOrDefault(schema: SchemaWithParse, value: unknown): unknown {
  const parsed = schema.safeParse(value ?? {});
  if (parsed.success) return parsed.data;
  const fallback = schema.safeParse({});
  return fallback.success ? fallback.data : value;
}

function networkSchemaFor(network: string): SchemaWithParse | null {
  switch (network) {
    case 'tcp': return TcpStreamSettingsSchema;
    case 'kcp': return KcpStreamSettingsSchema;
    case 'ws': return WsStreamSettingsSchema;
    case 'grpc': return GrpcStreamSettingsSchema;
    case 'httpupgrade': return HttpUpgradeStreamSettingsSchema;
    case 'xhttp': return XHttpStreamSettingsSchema;
    case 'hysteria': return HysteriaStreamSettingsSchema;
    default: return null;
  }
}

function securitySchemaFor(security: string): { key: string; schema: SchemaWithParse } | null {
  switch (security) {
    case 'tls': return { key: 'tlsSettings', schema: TlsStreamSettingsSchema };
    case 'reality': return { key: 'realitySettings', schema: RealityStreamSettingsSchema };
    default: return null;
  }
}

function fillStreamDefaults(stream: Record<string, unknown>): Record<string, unknown> {
  const network = (stream.network as string | undefined) ?? 'tcp';
  const security = (stream.security as string | undefined) ?? 'none';
  const out: Record<string, unknown> = { ...stream, network, security };
  const subKey = NETWORK_KEY_MAP[network as keyof typeof NETWORK_KEY_MAP];
  const netSchema = networkSchemaFor(network);
  if (subKey && netSchema) {
    out[subKey] = parseOrDefault(netSchema, out[subKey]);
  }
  const sec = securitySchemaFor(security);
  if (sec) {
    out[sec.key] = parseOrDefault(sec.schema, out[sec.key]);
  }
  return out;
}

function fillProtocolSettingsDefaults(protocol: string, settings: Record<string, unknown>): Record<string, unknown> {
  const parsed = InboundSettingsSchema.safeParse({ protocol, settings });
  if (parsed.success) {
    const tagged = parsed.data as { settings: Record<string, unknown> };
    return { ...tagged.settings };
  }
  return settings;
}

export function inboundFromDb(raw: DbInboundLike): Inbound {
  const rawSettings = coerceInboundJsonField(raw.settings);
  const settings = fillProtocolSettingsDefaults(raw.protocol, rawSettings);
  const streamSettingsRaw = coerceInboundJsonField(raw.streamSettings);
  const sniffing = coerceInboundJsonField(raw.sniffing);
  const streamSettings = Object.keys(streamSettingsRaw).length === 0
    ? streamSettingsRaw
    : fillStreamDefaults(streamSettingsRaw);
  return {
    protocol: raw.protocol,
    port: raw.port,
    listen: raw.listen ?? '',
    tag: raw.tag ?? '',
    remark: raw.remark ?? '',
    enable: raw.enable ?? true,
    expiryTime: raw.expiryTime ?? 0,
    up: raw.up ?? 0,
    down: raw.down ?? 0,
    total: raw.total ?? 0,
    settings,
    streamSettings,
    sniffing,
  } as unknown as Inbound;
}
