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

// Panel-only auto-rotation policy for a Reality inbound. Stripped before the
// config reaches xray (see internal/web/service/xray.go). Intervals are in
// days; 0 disables that rotation. The lastRotation timestamps (unix seconds)
// are managed by the backend RealityRotationJob and round-trip through the form
// so the cadence survives manual edits.
export const RealityRotationSchema = z.object({
  shortIdDays: z.number().int().min(0).default(0),
  publicKeyDays: z.number().int().min(0).default(0),
  lastShortIdRotation: z.number().int().min(0).default(0),
  lastPublicKeyRotation: z.number().int().min(0).default(0),
});
export type RealityRotation = z.infer<typeof RealityRotationSchema>;

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
    rotation: RealityRotationSchema.default({
      shortIdDays: 0,
      publicKeyDays: 0,
      lastShortIdRotation: 0,
      lastPublicKeyRotation: 0,
    }),
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
