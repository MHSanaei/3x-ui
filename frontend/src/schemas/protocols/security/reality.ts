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

// Reality stream payload. `serverNames` and `shortIds` are stored as
// comma-joined strings in the panel class but ship as string[] on the wire
// — fixtures round-trip through the array form. `target` is the dest host
// Reality piggybacks on; the panel auto-generates random target+SNI when
// blank.
export const RealityStreamSettingsSchema = z.object({
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
  settings: RealityClientSettingsSchema.default({
    publicKey: '',
    fingerprint: 'chrome',
    serverName: '',
    spiderX: '/',
    mldsa65Verify: '',
  }),
});
export type RealityStreamSettings = z.infer<typeof RealityStreamSettingsSchema>;
