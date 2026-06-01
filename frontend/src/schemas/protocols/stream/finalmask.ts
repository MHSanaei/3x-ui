import { z } from 'zod';

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
