import type { InboundFormValues, TrafficReset } from '@/schemas/forms/inbound-form';
import type { InboundSettings } from '@/schemas/protocols/inbound';
import type { StreamSettings } from '@/schemas/api/inbound';
import type { Sniffing } from '@/schemas/primitives';

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

function coerceTrafficReset(v: unknown): TrafficReset {
  return typeof v === 'string' && (TRAFFIC_RESETS as string[]).includes(v)
    ? (v as TrafficReset)
    : 'never';
}

// Map a raw DB row (settings/streamSettings/sniffing as string OR object)
// into the typed InboundFormValues. Does NOT validate against the schema —
// callers that want a hard guarantee should follow up with
// InboundFormSchema.safeParse(...).
export function rawInboundToFormValues(row: RawInboundRow): InboundFormValues {
  const protocol = (row.protocol || 'vless') as InboundSettings['protocol'];
  const settings = coerceJsonObject(row.settings) as InboundSettings['settings'];
  const rawStream = coerceJsonObject(row.streamSettings);
  const streamSettings = Object.keys(rawStream).length > 0
    ? (rawStream as StreamSettings)
    : undefined;
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
    protocol,
    settings,
  } as InboundFormValues;
}

export function formValuesToWirePayload(values: InboundFormValues): WireInboundPayload {
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
    settings: JSON.stringify(values.settings ?? {}),
    streamSettings: values.streamSettings ? JSON.stringify(values.streamSettings) : '',
    sniffing: JSON.stringify(values.sniffing ?? {}),
    tag: values.tag,
  };
  if (values.nodeId != null) payload.nodeId = values.nodeId;
  return payload;
}
