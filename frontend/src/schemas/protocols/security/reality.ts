import { z } from 'zod';

import { UtlsFingerprintSchema } from '@/schemas/protocols/security/tls';

// Reality client-side handshake config (sits under the inbound's
// realitySettings.settings on the wire — the panel's class names the field
// `settings` even though it's the "client" half of Reality).
export const RealityClientSettingsSchema = z.object({
  publicKey: z.string().default(''),
  fingerprint: UtlsFingerprintSchema.default('chrome'),
  serverName: z.string().default(''),
  spiderX: z.string().default('/'),
  mldsa65Verify: z.string().default(''),
});
export type RealityClientSettings = z.infer<typeof RealityClientSettingsSchema>;

// REALITY fallback rate-limit (xray-core reality.LimitFallback): throttles the
// fallback stream after `afterBytes`, then caps it at `bytesPerSec` with an
// optional `burstBytesPerSec`. Optional so existing inbounds round-trip
// unchanged — the object is only emitted once a user sets a non-zero value.
export const RealityLimitFallbackSchema = z.object({
  afterBytes: z.number().int().min(0).default(0),
  bytesPerSec: z.number().int().min(0).default(0),
  burstBytesPerSec: z.number().int().min(0).default(0),
});
export type RealityLimitFallback = z.infer<typeof RealityLimitFallbackSchema>;

// xray-core accepts both `target` and `dest` as the REALITY destination —
// they are aliases (infra/conf/transport_internet.go: REALITYConfig has
// `json:"target"` and `json:"dest"`). The panel writes `target`, but configs
// produced by older panel builds, external tools, or the panel's own
// `/panel/api/inbounds` API commonly use `dest`. Map `dest` -> `target` on
// parse when `target` is absent/empty: otherwise such an inbound loads with
// an empty (required) Target field even though it runs fine, and re-saving
// it serializes the blank `target` and drops the working `dest` — silently
// breaking REALITY on the next xray restart.
const aliasRealityDest = (value: unknown): unknown => {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    const obj = value as Record<string, unknown>;
    const hasTarget = typeof obj.target === 'string' && obj.target !== '';
    if (!hasTarget && typeof obj.dest === 'string' && obj.dest !== '') {
      return { ...obj, target: obj.dest };
    }
  }
  return value;
};

// Reality stream payload. `serverNames` and `shortIds` are stored as
// comma-joined strings in the panel class but ship as string[] on the wire
// — fixtures round-trip through the array form. `target` is the dest host
// Reality piggybacks on; the panel auto-generates random target+SNI when
// blank.
export const RealityStreamSettingsSchema = z.preprocess(
  aliasRealityDest,
  z.object({
    show: z.boolean().default(false),
    xver: z.number().int().min(0).default(0),
    target: z.string().default(''),
    serverNames: z.array(z.string()).default([]),
    privateKey: z.string().default(''),
    minClientVer: z.string().default(''),
    maxClientVer: z.string().default(''),
    maxTimediff: z.number().int().min(0).default(0),
    shortIds: z.array(z.string()).default([]),
    mldsa65Seed: z.string().default(''),
    // Server-side TLS master-key log path (xray-core reality.Config). Optional
    // so existing inbounds round-trip unchanged.
    masterKeyLog: z.string().optional(),
    limitFallbackUpload: RealityLimitFallbackSchema.optional(),
    limitFallbackDownload: RealityLimitFallbackSchema.optional(),
    settings: RealityClientSettingsSchema.default({
      publicKey: '',
      fingerprint: 'chrome',
      serverName: '',
      spiderX: '/',
      mldsa65Verify: '',
    }),
  }),
);
export type RealityStreamSettings = z.infer<typeof RealityStreamSettingsSchema>;
