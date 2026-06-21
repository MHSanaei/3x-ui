import { z } from 'zod';

import { SockoptStreamSettingsSchema } from '@/schemas/protocols/stream/sockopt';

export const TlsVersionSchema = z.enum(['1.0', '1.1', '1.2', '1.3']);
export type TlsVersion = z.infer<typeof TlsVersionSchema>;

// Xray's uTLS fingerprints — used both for TLS and Reality. Kept here (not
// in primitives/) because the only consumer is security/tls.ts and
// security/reality.ts via re-import.
export const UtlsFingerprintSchema = z.enum([
  'chrome',
  'firefox',
  'safari',
  'ios',
  'android',
  'edge',
  '360',
  'qq',
  'random',
  'randomized',
  'randomizednoalpn',
  'unsafe',
]);
export type UtlsFingerprint = z.infer<typeof UtlsFingerprintSchema>;

export const TlsFingerprintSchema = z.union([UtlsFingerprintSchema, z.literal('')]);
export type TlsFingerprint = z.infer<typeof TlsFingerprintSchema>;

export const AlpnSchema = z.enum(['h3', 'h2', 'http/1.1']);
export type Alpn = z.infer<typeof AlpnSchema>;

export const TlsCertUsageSchema = z.enum(['encipherment', 'verify', 'issue']);
export type TlsCertUsage = z.infer<typeof TlsCertUsageSchema>;

// TLS certs on the wire come in two shapes — file-backed or inline. The
// panel class collapses them into one with a `useFile` boolean; we model
// the wire shape as a DU so saves round-trip without the boolean leaking.
export const TlsCertFileSchema = z.object({
  certificateFile: z.string().min(1),
  keyFile: z.string().min(1),
  ocspStapling: z.number().default(0),
  oneTimeLoading: z.boolean().default(false),
  usage: TlsCertUsageSchema.default('encipherment'),
  buildChain: z.boolean().default(false),
});
export const TlsCertInlineSchema = z.object({
  certificate: z.array(z.string()),
  key: z.array(z.string()),
  ocspStapling: z.number().default(0),
  oneTimeLoading: z.boolean().default(false),
  usage: TlsCertUsageSchema.default('encipherment'),
  buildChain: z.boolean().default(false),
});
export const TlsCertSchema = z.union([TlsCertFileSchema, TlsCertInlineSchema]);
export type TlsCert = z.infer<typeof TlsCertSchema>;

export const TlsClientSettingsSchema = z.object({
  fingerprint: TlsFingerprintSchema.default('chrome'),
  echConfigList: z.string().default(''),
  pinnedPeerCertSha256: z.array(z.string()).default([]),
  // Panel-only client directive (v2rayN `vcn`): verify the server certificate
  // against this name instead of the SNI. Comma-separated names. Shipped in
  // share links / subscriptions; the modern replacement for `allowInsecure`,
  // which xray-core removed after 2026-06-01.
  verifyPeerCertByName: z.string().default(''),
});
export type TlsClientSettings = z.infer<typeof TlsClientSettingsSchema>;

// `serverName` is the SNI; the class field is `sni` internally but on the
// wire stays `serverName` to match Xray's config schema.
export const TlsStreamSettingsSchema = z.object({
  serverName: z.string().default(''),
  minVersion: TlsVersionSchema.default('1.2'),
  maxVersion: TlsVersionSchema.default('1.3'),
  cipherSuites: z.string().default(''),
  rejectUnknownSni: z.boolean().default(false),
  disableSystemRoot: z.boolean().default(false),
  enableSessionResumption: z.boolean().default(false),
  certificates: z.array(TlsCertSchema).default([]),
  alpn: z.array(AlpnSchema).default(['h2', 'http/1.1']),
  echServerKeys: z.string().default(''),
  // Server-side TLS fields (xray-core TLSConfig top-level): survive the
  // panel-only `settings` strip and reach the runtime config. Optional so
  // existing inbounds round-trip unchanged.
  curvePreferences: z.array(z.string()).optional(),
  masterKeyLog: z.string().optional(),
  echSockopt: SockoptStreamSettingsSchema.optional(),
  settings: TlsClientSettingsSchema.default({ fingerprint: 'chrome', echConfigList: '', pinnedPeerCertSha256: [], verifyPeerCertByName: '' }),
});
export type TlsStreamSettings = z.infer<typeof TlsStreamSettingsSchema>;
