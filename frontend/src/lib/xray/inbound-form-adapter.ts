import type { InboundFormValues, ShareAddrStrategy, TrafficReset } from '@/schemas/forms/inbound-form';
import type { InboundSettings } from '@/schemas/protocols/inbound';
import {
  HysteriaClientSchema,
  ShadowsocksClientSchema,
  TrojanClientSchema,
  VlessClientSchema,
  VmessClientSchema,
} from '@/schemas/protocols/inbound';
import type { StreamSettings } from '@/schemas/api/inbound';
import type { Sniffing } from '@/schemas/primitives';
import type { z } from 'zod';
import { normalizeStreamSettingsForWire } from '@/lib/xray/stream-wire-normalize';
import { canEnableSniffing } from '@/lib/xray/protocol-capabilities';

// Plain-data adapter between the panel's stored inbound row shape and
// the typed InboundFormValues that Form.useForm<T> carries inside
// InboundFormModal. No dependency on the legacy Inbound/DBInbound
// classes — the modal hands the raw row in, takes typed values out, and
// on submit calls formValuesToWirePayload() to get a payload ready to
// POST to /panel/api/inbounds/add or /update/:id.

export interface RawInboundRow {
  port?: number;
  listen?: string;
  protocol?: string;
  tag?: string;
  settings?: unknown;
  streamSettings?: unknown;
  sniffing?: unknown;
  up?: number;
  down?: number;
  total?: number;
  remark?: string;
  enable?: boolean;
  expiryTime?: number;
  trafficReset?: string;
  lastTrafficResetTime?: number;
  nodeId?: number | null;
  shareAddrStrategy?: string;
  shareAddr?: string;
  clientStats?: unknown;
}

// The wire payload — settings/streamSettings/sniffing arrive as JSON
// strings, mirroring what the Go endpoints expect (xray-core wants the
// nested config slices as strings to round-trip through its loader).
export interface WireInboundPayload {
  up: number;
  down: number;
  total: number;
  remark: string;
  enable: boolean;
  expiryTime: number;
  trafficReset: TrafficReset;
  lastTrafficResetTime: number;
  listen: string;
  port: number;
  protocol: string;
  settings: string;
  streamSettings: string;
  sniffing: string;
  tag: string;
  clientStats?: unknown;
  nodeId?: number;
  shareAddrStrategy: ShareAddrStrategy;
  shareAddr: string;
}

function coerceJsonObject(value: unknown): Record<string, unknown> {
  if (value == null) return {};
  if (typeof value === 'object' && !Array.isArray(value)) {
    return value as Record<string, unknown>;
  }
  if (typeof value !== 'string') return {};
  const trimmed = value.trim();
  if (trimmed === '') return {};
  try {
    const parsed = JSON.parse(trimmed);
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
      ? (parsed as Record<string, unknown>)
      : {};
  } catch {
    return {};
  }
}

const TRAFFIC_RESETS: TrafficReset[] = ['never', 'hourly', 'daily', 'weekly', 'monthly'];
const SHARE_ADDR_STRATEGIES: ShareAddrStrategy[] = ['node', 'listen', 'custom'];

function coerceTrafficReset(v: unknown): TrafficReset {
  return typeof v === 'string' && (TRAFFIC_RESETS as string[]).includes(v)
    ? (v as TrafficReset)
    : 'never';
}

function coerceShareAddrStrategy(v: unknown): ShareAddrStrategy {
  return typeof v === 'string' && (SHARE_ADDR_STRATEGIES as string[]).includes(v)
    ? (v as ShareAddrStrategy)
    : 'node';
}

// Network values that map to a required `${network}Settings` key in
// NetworkSettingsSchema. Older saved inbounds may be missing the per-
// network sub-object (the legacy panel sometimes emitted streamSettings
// without it, and an earlier panel-side prune wrongly stripped empty
// `tcpSettings: {}` out of the wire payload). Reseat an empty object
// here so InboundFormSchema.safeParse doesn't blow up at edit time.
const NETWORK_SETTINGS_KEY: Record<string, string> = {
  tcp: 'tcpSettings',
  kcp: 'kcpSettings',
  ws: 'wsSettings',
  grpc: 'grpcSettings',
  httpupgrade: 'httpupgradeSettings',
  xhttp: 'xhttpSettings',
  hysteria: 'hysteriaSettings',
};

function healStreamNetworkKey(stream: Record<string, unknown>): void {
  const network = typeof stream.network === 'string' ? stream.network : '';
  const key = NETWORK_SETTINGS_KEY[network];
  if (!key) return;
  if (stream[key] == null || typeof stream[key] !== 'object') {
    stream[key] = {};
  }
}

function tlsCerts(stream: Record<string, unknown>): Record<string, unknown>[] {
  const tls = stream.tlsSettings as { certificates?: unknown } | undefined;
  return Array.isArray(tls?.certificates) ? tls.certificates as Record<string, unknown>[] : [];
}

function synthesizeTlsCertUseFile(stream: Record<string, unknown>): void {
  for (const c of tlsCerts(stream)) {
    if (typeof c.useFile === 'boolean') continue;
    const hasFile = !!c.certificateFile || !!c.keyFile;
    const hasInline =
      (Array.isArray(c.certificate) && c.certificate.length > 0) ||
      (Array.isArray(c.key) && c.key.length > 0);
    c.useFile = hasFile || !hasInline;
  }
}

function stripTlsCertUseFile(stream: Record<string, unknown>): void {
  for (const c of tlsCerts(stream)) delete c.useFile;
}

export function rawInboundToFormValues(row: RawInboundRow): InboundFormValues {
  const protocol = (row.protocol || 'vless') as InboundSettings['protocol'];
  const settings = coerceJsonObject(row.settings) as InboundSettings['settings'];
  const rawStream = coerceJsonObject(row.streamSettings);
  const streamSettings = Object.keys(rawStream).length > 0
    ? (rawStream as StreamSettings)
    : undefined;
  if (streamSettings) {
    healStreamNetworkKey(streamSettings as unknown as Record<string, unknown>);
    synthesizeTlsCertUseFile(streamSettings as unknown as Record<string, unknown>);
  }
  const sniffing = coerceJsonObject(row.sniffing) as unknown as Sniffing;

  return {
    remark: row.remark ?? '',
    enable: row.enable ?? true,
    port: row.port ?? 0,
    listen: row.listen ?? '',
    tag: row.tag ?? '',
    expiryTime: row.expiryTime ?? 0,
    sniffing,
    streamSettings,
    up: row.up ?? 0,
    down: row.down ?? 0,
    total: row.total ?? 0,
    trafficReset: coerceTrafficReset(row.trafficReset),
    lastTrafficResetTime: row.lastTrafficResetTime ?? 0,
    nodeId: row.nodeId ?? null,
    shareAddrStrategy: coerceShareAddrStrategy(row.shareAddrStrategy),
    shareAddr: row.shareAddr ?? '',
    protocol,
    settings,
  } as InboundFormValues;
}

// Recursively strip undefined leaves from the wire payload. Empty arrays
// and empty objects are PRESERVED — legacy XrayCommonClass.toJson() kept
// shells like `tcpSettings: {}` so xray-core picks up its built-in
// defaults, and stripping them led the FE to lose required-but-empty
// arrays (vless clients, wireguard peers, etc.) which the Go side then
// serialized back as `null`. Primitive values (including 0, false, '')
// are kept verbatim.
export function pruneEmpty(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(pruneEmpty);
  }
  if (value !== null && typeof value === 'object') {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      const p = pruneEmpty(v);
      if (p === undefined) continue;
      out[k] = p;
    }
    return out;
  }
  return value;
}

// Per-protocol client field whitelist — the Zod schemas in
// schemas/protocols/inbound/<proto>.ts define which keys a given
// protocol's clients accept on the wire. When a global client is created
// the panel may persist cross-protocol fields on the same row (`auth` for
// hysteria, `password` for trojan, `security` for vmess, etc.); rendering
// those inside a vless inbound's settings.clients is confusing and rides
// dead weight in the wire payload. Parsing through the protocol's schema
// gives us the canonical projection.
function clientSchemaForProtocol(protocol: string): z.ZodType | null {
  switch (protocol) {
    case 'vless': return VlessClientSchema;
    case 'vmess': return VmessClientSchema;
    case 'trojan': return TrojanClientSchema;
    case 'shadowsocks': return ShadowsocksClientSchema;
    case 'hysteria': return HysteriaClientSchema;
    default: return null;
  }
}

export function normalizeClients(protocol: string, clients: unknown): unknown {
  const schema = clientSchemaForProtocol(protocol);
  if (!schema || !Array.isArray(clients)) return clients;
  return clients.map((c) => {
    const parsed = schema.safeParse(c);
    return parsed.success ? parsed.data : c;
  });
}

// Sniffing normalizer matching the legacy Sniffing.toJson(): when
// disabled the payload is the bare `{ enabled: false }` regardless of
// what the form holds; when enabled, only non-default fields ride.
export function normalizeSniffing(s: Sniffing | undefined): Record<string, unknown> {
  if (!s || !s.enabled) return { enabled: false };
  const out: Record<string, unknown> = {
    enabled: true,
    destOverride: s.destOverride,
  };
  if (s.metadataOnly) out.metadataOnly = true;
  if (s.routeOnly) out.routeOnly = true;
  if (s.ipsExcluded?.length) out.ipsExcluded = s.ipsExcluded;
  if (s.domainsExcluded?.length) out.domainsExcluded = s.domainsExcluded;
  return out;
}

// Drops cosmetic empty-array keys that legacy XrayCommonClass.toJson()
// explicitly skipped (fallbacks/finalmask). Mutates the pruned settings
// objects in place; called AFTER pruneEmpty so we can lean on the
// already-shallow shape.
export function dropLegacyOptionalEmpties(
  settings: Record<string, unknown>,
  stream: Record<string, unknown> | undefined,
): void {
  // VLESS/Trojan emit `fallbacks` only when non-empty.
  const fb = settings.fallbacks;
  if (Array.isArray(fb) && fb.length === 0) delete settings.fallbacks;

  if (stream) {
    // StreamSettings emits `finalmask` only when at least one transport
    // mask exists (legacy `hasFinalMask`). Drop the whole block when all
    // sub-fields are empty; otherwise drop only the empty sub-arrays so
    // the wire payload doesn't carry a stray `"tcp": []` next to a
    // populated UDP mask list (and vice versa).
    const fm = stream.finalmask as { tcp?: unknown[]; udp?: unknown[]; quicParams?: unknown } | undefined;
    if (fm && typeof fm === 'object') {
      const hasTcp = Array.isArray(fm.tcp) && fm.tcp.length > 0;
      const hasUdp = Array.isArray(fm.udp) && fm.udp.length > 0;
      const hasQuic = fm.quicParams != null;
      if (!hasTcp && !hasUdp && !hasQuic) {
        delete stream.finalmask;
      } else {
        if (!hasTcp) delete fm.tcp;
        if (!hasUdp) delete fm.udp;
      }
    }

    // Hysteria's per-client auth lives in settings.clients[*].auth; the
    // streamSettings.hysteriaSettings.auth slot is a holdover from older
    // hysteria builds and serves no purpose on the inbound side, so an
    // empty value shouldn't ride along in the JSON payload.
    const hs = stream.hysteriaSettings as { auth?: string } | undefined;
    if (hs && typeof hs === 'object' && (hs.auth === '' || hs.auth == null)) {
      delete hs.auth;
    }
  }
}

export function formValuesToWirePayload(values: InboundFormValues): WireInboundPayload {
  const settingsPruned = (pruneEmpty(values.settings ?? {}) ?? {}) as Record<string, unknown>;
  if (Array.isArray(settingsPruned.clients)) {
    settingsPruned.clients = normalizeClients(values.protocol, settingsPruned.clients);
  }
  let streamPruned = values.streamSettings
    ? ((pruneEmpty(values.streamSettings) ?? {}) as Record<string, unknown>)
    : undefined;
  if (streamPruned) {
    streamPruned = normalizeStreamSettingsForWire(streamPruned, { side: 'inbound' });
    stripTlsCertUseFile(streamPruned);
  }
  dropLegacyOptionalEmpties(settingsPruned, streamPruned);
  const payload: WireInboundPayload = {
    up: values.up,
    down: values.down,
    total: values.total,
    remark: values.remark,
    enable: values.enable,
    expiryTime: values.expiryTime,
    trafficReset: values.trafficReset,
    lastTrafficResetTime: values.lastTrafficResetTime,
    listen: values.listen,
    port: values.port,
    protocol: values.protocol,
    settings: JSON.stringify(settingsPruned),
    streamSettings: streamPruned ? JSON.stringify(streamPruned) : '',
    // mtproto is mtg-served, not Xray, so sniffing never applies — emit empty
    // rather than the default { enabled: false } so the row carries no sniffing.
    sniffing: canEnableSniffing({ protocol: values.protocol }) ? JSON.stringify(normalizeSniffing(values.sniffing)) : '',
    tag: values.tag,
    shareAddrStrategy: values.shareAddrStrategy,
    shareAddr: values.shareAddr,
  };
  if (values.nodeId != null) payload.nodeId = values.nodeId;
  return payload;
}
