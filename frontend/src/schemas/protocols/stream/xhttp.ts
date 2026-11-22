import { z } from 'zod';

import { WsHeaderMapSchema } from '@/schemas/protocols/stream/ws';

export const XHttpModeSchema = z.enum(['auto', 'packet-up', 'stream-up', 'stream-one']);
export type XHttpMode = z.infer<typeof XHttpModeSchema>;

export function xhttpUplinkMethods(mode?: string) {
  return [
    { value: '', label: 'Default (POST)' },
    ...['POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS'].map((value) => ({ value, label: value })),
    { value: 'GET', label: 'GET (packet-up only)', disabled: mode !== 'packet-up' },
    // OpenStream drains the upload response before closing its request body.
    // HEAD has no response body to keep a streaming upload open.
    { value: 'HEAD', label: 'HEAD (packet-up only)', disabled: mode !== 'packet-up' },
  ];
}

// Xray accepts an int32 or a range string. Text controls use the string form.
const rangeText = z.union([z.string(), z.int32().transform(String)]);

// A stored XMUX object is sparse. Xray chooses its own fallback when the
// block is empty. Preserve explicit zeros, numeric ranges and omitted keys.
const xmuxRange = z.union([z.int32(), z.string()]).optional();
export const XHttpXmuxSchema = z.looseObject({
  maxConcurrency: xmuxRange,
  maxConnections: xmuxRange,
  cMaxReuseTimes: xmuxRange,
  hMaxRequestTimes: xmuxRange,
  hMaxReusableSecs: xmuxRange,
  hKeepAlivePeriod: z.number().int().min(0).optional(),
});
export type XHttpXmux = z.infer<typeof XHttpXmuxSchema>;

// Seed only a newly enabled block. Select one connection-limit strategy.
export const XMUX_FRESH_DEFAULTS: XHttpXmux = {
  maxConcurrency: '',
  maxConnections: 3,
  cMaxReuseTimes: 0,
  hMaxRequestTimes: '600-900',
  hMaxReusableSecs: '1800-3000',
  hKeepAlivePeriod: 0,
};

// Predefined sessionIDTable names xray-core accepts as a shorthand for a
// charset (splithttp.PredefinedTable, xray-core #6258). A literal ASCII
// charset string is also accepted.
export const XHTTP_SESSION_ID_TABLES = [
  'ALPHABET',
  'Alphabet',
  'BASE36',
  'Base62',
  'HEX',
  'alphabet',
  'base36',
  'hex',
  'number',
] as const;

// xray-core #6258 renamed sessionPlacement/sessionKey to
// sessionIDPlacement/sessionIDKey (no fallback kept in core) and added
// sessionIDTable/sessionIDLength. Lift any legacy keys persisted by an older
// panel onto the new names so a saved inbound/outbound never silently loses
// its session setting, then drop the legacy keys so we never emit both.
// In core's wrapped syntax only host, path and mode come from the outer
// object. The panel edits the equivalent flat syntax.
export function flattenXhttpExtra(raw: Record<string, unknown>): Record<string, unknown> {
  const { extra, host, path, mode } = raw;
  const flat: Record<string, unknown> =
    extra && typeof extra === 'object' && !Array.isArray(extra)
      ? { ...extra, host, path, mode }
      : { ...raw };
  if (flat.xmux === null) delete flat.xmux;
  if (flat.downloadSettings === null) delete flat.downloadSettings;
  return flat;
}

function migrateLegacyXhttp(v: unknown): unknown {
  if (v == null || typeof v !== 'object' || Array.isArray(v)) return v;
  const o = flattenXhttpExtra(v as Record<string, unknown>);
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

const maxChunkSize = 0x7fffffff;
const chunkSizeMessage =
  'Enter a nonnegative integer or byte range, e.g. 512-1024 (maximum 2147483647).';
export const XHttpChunkSizeSchema = z
  .union(
    [
      z.number().int().min(0).max(maxChunkSize),
      z
        .string()
        .refine(
          (value) =>
            value === '' ||
            (/^\d+(?:-\d+)?$/.test(value) &&
              value.split('-').every((part) => Number(part) <= maxChunkSize)),
          chunkSizeMessage,
        ),
    ],
    { error: chunkSizeMessage },
  )
  .default(0);

export const XHttpStreamSettingsSchema = z.preprocess(
  migrateLegacyXhttp,
  z.looseObject({
    downloadSettings: z.record(z.string(), z.unknown()).optional(),
    path: z.string().default('/'),
    host: z.string().default(''),
    mode: XHttpModeSchema.default('auto'),
    xPaddingBytes: rangeText.default('100-1000'),
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
    sessionIDLength: rangeText.default(''),
    seqPlacement: z.string().default(''),
    seqKey: z.string().default(''),
    uplinkDataPlacement: z.string().default(''),
    uplinkDataKey: z.string().default(''),
    // Empty default on purpose: xray-core already defaults to 1MB/30ms, and
    // baking the literal values into every config and share link gives DPI a
    // stable fingerprint (#5141 — TSPU keys on scMinPostsIntervalMs=30).
    scMaxEachPostBytes: z
      .union([z.string(), z.number().int().nonnegative().transform(String)])
      .default(''),
    noSSEHeader: z.boolean().default(false),
    scMaxBufferedPosts: z.number().int().min(0).default(30),
    scStreamUpServerSecs: rangeText.default('20-80'),
    serverMaxHeaderBytes: z.number().int().min(0).default(0),
    uplinkHTTPMethod: z.string().default(''),
    headers: WsHeaderMapSchema.default({}),
    // Server-only Go net/http HTTP2Config (xray-core 26.10.31); never put into share links.
    h2Params: z.record(z.string(), z.unknown()).optional(),
    // Client-side fields stored on inbound for subscription propagation.
    // The server listener ignores them at runtime, but the panel embeds
    // them in share-link `extra` blobs so the same xhttp config can
    // round-trip on both sides.
    // - scMinPostsIntervalMs: preserved when non-default (stripped at '' or '30')
    scMinPostsIntervalMs: rangeText.default(''),
    uplinkChunkSize: XHttpChunkSizeSchema,
    noGRPCHeader: z.boolean().default(false),
    xmux: XHttpXmuxSchema.optional(),
    // UI-only toggle controlling whether the XMUX sub-form is expanded.
    // Never present on the wire — outbound modal strips it via the
    // form-to-wire adapter.
    enableXmux: z.boolean().default(false),
  }),
);
export type XHttpStreamSettings = z.infer<typeof XHttpStreamSettingsSchema>;
