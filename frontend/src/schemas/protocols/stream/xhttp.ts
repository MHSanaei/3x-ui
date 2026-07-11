import { z } from 'zod';

import { WsHeaderMapSchema } from '@/schemas/protocols/stream/ws';

export const XHttpModeSchema = z.enum(['auto', 'packet-up', 'stream-up', 'stream-one']);
export type XHttpMode = z.infer<typeof XHttpModeSchema>;

// xHTTP (SplitHTTPConfig) is xray-core's modern stream-multiplexed transport.
// The field set is large because the schema mirrors what the server-side
// listener reads — plus a few client-only fields (`uplinkHTTPMethod`,
// `headers`) the panel embeds into share-link `extra` blobs even though the
// server ignores them at runtime. Outbound has additional fields (uplinkChunk
// sizes, noGRPCHeader, scMinPostsIntervalMs, xmux, downloadSettings) which
// belong on the outbound class instead, not modeled here.
// XMUX is the connection-multiplexing layer xHTTP uses to fan out
// parallel requests over a small pool of upstream connections. Fields
// are strings because they accept dash-range values like '16-32'.
// maxConcurrency and maxConnections are mutually exclusive strategies
// (xray-core rejects a config that sets both), so the bare schema
// default keeps only one of them non-zero — a non-zero maxConnections
// default resurrected on load made every re-save silently delete the
// user's maxConcurrency.
export const XHttpXmuxSchema = z.object({
  maxConcurrency: z.string().default('16-32'),
  maxConnections: z.union([z.string(), z.number()]).default(0),
  cMaxReuseTimes: z.union([z.string(), z.number()]).default(0),
  hMaxRequestTimes: z.string().default('600-900'),
  hMaxReusableSecs: z.string().default('1800-3000'),
  hKeepAlivePeriod: z.number().int().min(0).default(0),
});
export type XHttpXmux = z.infer<typeof XHttpXmuxSchema>;

// Seed for freshly enabling XMUX on a config that had no xmux block:
// mirrors xray-core v26.6.27's own anti-RKN maxConnections=6 fallback
// rather than the concurrency strategy.
export const XMUX_FRESH_DEFAULTS: XHttpXmux = {
  ...XHttpXmuxSchema.parse({}),
  maxConcurrency: '',
  maxConnections: 6,
};

// Predefined sessionIDTable names xray-core accepts as a shorthand for a
// charset (splithttp.PredefinedTable, xray-core #6258). A literal ASCII
// charset string is also accepted.
export const XHTTP_SESSION_ID_TABLES = [
  'ALPHABET', 'Alphabet', 'BASE36', 'Base62', 'HEX',
  'alphabet', 'base36', 'hex', 'number',
] as const;

// xray-core #6258 renamed sessionPlacement/sessionKey to
// sessionIDPlacement/sessionIDKey (no fallback kept in core) and added
// sessionIDTable/sessionIDLength. Lift any legacy keys persisted by an older
// panel onto the new names so a saved inbound/outbound never silently loses
// its session setting, then drop the legacy keys so we never emit both.
function migrateLegacyXhttp(v: unknown): unknown {
  if (v == null || typeof v !== 'object' || Array.isArray(v)) return v;
  const o = { ...(v as Record<string, unknown>) };
  if (o.sessionIDPlacement === undefined && o.sessionPlacement !== undefined) {
    o.sessionIDPlacement = o.sessionPlacement;
  }
  if (o.sessionIDKey === undefined && o.sessionKey !== undefined) {
    o.sessionIDKey = o.sessionKey;
  }
  delete o.sessionPlacement;
  delete o.sessionKey;
  return o;
}

export const XHttpStreamSettingsSchema = z.preprocess(migrateLegacyXhttp, z.object({
  path: z.string().default('/'),
  host: z.string().default(''),
  mode: XHttpModeSchema.default('auto'),
  xPaddingBytes: z.string().default('100-1000'),
  xPaddingObfsMode: z.boolean().default(false),
  xPaddingKey: z.string().default(''),
  xPaddingHeader: z.string().default(''),
  xPaddingPlacement: z.string().default(''),
  xPaddingMethod: z.string().default(''),
  sessionIDPlacement: z.string().default(''),
  sessionIDKey: z.string().default(''),
  // sessionIDTable: a predefined name (XHTTP_SESSION_ID_TABLES) or a literal
  // ASCII charset. sessionIDLength: dash-range string (e.g. '8-16'); only
  // honored when a table is set. xray-core enforces the room-size minimum.
  sessionIDTable: z.string().default(''),
  sessionIDLength: z.string().default(''),
  seqPlacement: z.string().default(''),
  seqKey: z.string().default(''),
  uplinkDataPlacement: z.string().default(''),
  uplinkDataKey: z.string().default(''),
  // Empty default on purpose: xray-core already defaults to 1MB/30ms, and
  // baking the literal values into every config and share link gives DPI a
  // stable fingerprint (#5141 — TSPU keys on scMinPostsIntervalMs=30).
  scMaxEachPostBytes: z.string().default(''),
  noSSEHeader: z.boolean().default(false),
  scMaxBufferedPosts: z.number().int().min(0).default(30),
  scStreamUpServerSecs: z.string().default('20-80'),
  serverMaxHeaderBytes: z.number().int().min(0).default(0),
  uplinkHTTPMethod: z.string().default(''),
  headers: WsHeaderMapSchema.default({}),
  // Client-side fields stored on inbound for subscription propagation.
  // The server listener ignores them at runtime, but the panel embeds
  // them in share-link `extra` blobs so the same xhttp config can
  // round-trip on both sides.
  // - scMinPostsIntervalMs: preserved when non-default (stripped at '' or '30')
  // - uplinkChunkSize & noGRPCHeader: outbound-only; stripped from inbound wire
  scMinPostsIntervalMs: z.string().default(''),
  uplinkChunkSize: z.number().int().min(0).default(0),
  noGRPCHeader: z.boolean().default(false),
  xmux: XHttpXmuxSchema.optional(),
  // UI-only toggle controlling whether the XMUX sub-form is expanded.
  // Never present on the wire — outbound modal strips it via the
  // form-to-wire adapter.
  enableXmux: z.boolean().default(false),
}));
export type XHttpStreamSettings = z.infer<typeof XHttpStreamSettingsSchema>;
