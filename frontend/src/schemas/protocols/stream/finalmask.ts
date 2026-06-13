import { z } from 'zod';

import { AlpnSchema, UtlsFingerprintSchema } from '@/schemas/protocols/security/tls';

// FinalMask is xray-core's late-layer obfuscation wrapper applied AFTER
// the network/security layers. It models per-type masks on TCP and UDP
// plus optional QUIC tuning. The `settings` sub-object is polymorphic on
// `type`; we model the wire-faithful shape with a permissive
// record-of-unknown for `settings` and leave per-type tightening to
// Step 6 — there are 8 UDP mask types plus 3 TCP mask types, each with
// distinct setting fields, and modeling them all as discriminated unions
// here would dwarf the rest of the stream module without buying anything
// the safety net doesn't already cover.

export const TcpMaskTypeSchema = z.enum(['fragment', 'sudoku', 'header-custom']);
export type TcpMaskType = z.infer<typeof TcpMaskTypeSchema>;

export const TcpMaskSchema = z.object({
  type: TcpMaskTypeSchema,
  settings: z.record(z.string(), z.unknown()).optional(),
});
export type TcpMask = z.infer<typeof TcpMaskSchema>;

export const UdpMaskTypeSchema = z.enum([
  'salamander',
  'mkcp-legacy',
  'header-custom',
  'xdns',
  'xicmp',
  'noise',
  'sudoku',
  'realm',
]);
export type UdpMaskType = z.infer<typeof UdpMaskTypeSchema>;

export const UdpMaskSchema = z.object({
  type: UdpMaskTypeSchema,
  settings: z.record(z.string(), z.unknown()).optional(),
});
export type UdpMask = z.infer<typeof UdpMaskSchema>;

export const QuicCongestionSchema = z.enum(['reno', 'bbr', 'brutal', 'force-brutal']);
export type QuicCongestion = z.infer<typeof QuicCongestionSchema>;

export const BbrProfileSchema = z.enum(['conservative', 'standard', 'aggressive']);
export type BbrProfile = z.infer<typeof BbrProfileSchema>;

// udpHop randomizes the QUIC port between a range every `interval` seconds
// to dodge port-based blocking. Both fields are dash-range strings on the
// wire (e.g. '20000-50000', '5-10'). preprocess coerces legacy DB rows
// where interval was stored as a number (UI bug — see B19 in commit history).
const StringRangeSchema = z.preprocess(
  (v) => (typeof v === 'number' ? String(v) : v),
  z.string(),
);

// Salamander UDP mask. `password` is the obfuscation secret; the optional
// `packetSize` dash-range turns on Gecko (Hysteria v2.9.2) — xray-core builds
// a GeckoConfig from MinPacketSize/MaxPacketSize when it is present and plain
// Salamander when it is absent, so leaving it empty preserves prior behaviour.
// Marshalled as an Int32Range, which accepts a "min-max" string or plain int.
export const SalamanderSettingsSchema = z.object({
  password: z.string().default(''),
  packetSize: StringRangeSchema.optional(),
});
export type SalamanderSettings = z.infer<typeof SalamanderSettingsSchema>;

// Realm UDP hole-punching mask (Hysteria v2.9.1). `url` is a
// realm://token@host:port/id (or realm+http://) endpoint and `stunServers`
// are host:port pairs. The optional `tlsConfig` is xray-core's flat TLSConfig
// for the connection to the realm server — note `fingerprint` is a top-level
// key here, unlike the panel's own TLS stream settings which nest it under
// `settings`.
export const RealmTlsConfigSchema = z.object({
  serverName: z.string().optional(),
  allowInsecure: z.boolean().optional(),
  alpn: z.array(AlpnSchema).optional(),
  fingerprint: UtlsFingerprintSchema.optional(),
});
export type RealmTlsConfig = z.infer<typeof RealmTlsConfigSchema>;

export const RealmSettingsSchema = z.object({
  url: z.string().default(''),
  stunServers: z.array(z.string()).default([]),
  tlsConfig: RealmTlsConfigSchema.optional(),
});
export type RealmSettings = z.infer<typeof RealmSettingsSchema>;

export const QuicUdpHopSchema = z.object({
  ports: StringRangeSchema.default('20000-50000'),
  interval: StringRangeSchema.default('5-10'),
});
export type QuicUdpHop = z.infer<typeof QuicUdpHopSchema>;

export const QuicParamsSchema = z.object({
  congestion: QuicCongestionSchema.default('bbr'),
  bbrProfile: BbrProfileSchema.optional(),
  debug: z.boolean().optional(),
  brutalUp: z.string().optional(),
  brutalDown: z.string().optional(),
  udpHop: QuicUdpHopSchema.optional(),
  initStreamReceiveWindow: z.number().int().min(0).optional(),
  maxStreamReceiveWindow: z.number().int().min(0).optional(),
  initConnectionReceiveWindow: z.number().int().min(0).optional(),
  maxConnectionReceiveWindow: z.number().int().min(0).optional(),
  maxIdleTimeout: z.number().int().min(4).max(120).optional(),
  keepAlivePeriod: z.number().int().min(2).max(60).optional(),
  disablePathMTUDiscovery: z.boolean().optional(),
  maxIncomingStreams: z.number().int().min(8).optional(),
});
export type QuicParams = z.infer<typeof QuicParamsSchema>;

// `tcp` and `udp` are omitted from the wire entirely when their arrays
// are empty (legacy toJson() drops them). Our default([]) here mirrors
// the parsed-in shape; the shadow harness already treats empty arrays as
// equivalent to absence so both pipelines converge.
export const FinalMaskStreamSettingsSchema = z.object({
  tcp: z.array(TcpMaskSchema).default([]),
  udp: z.array(UdpMaskSchema).default([]),
  quicParams: QuicParamsSchema.optional(),
});
export type FinalMaskStreamSettings = z.infer<typeof FinalMaskStreamSettingsSchema>;
