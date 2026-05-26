import { z } from 'zod';

// Hysteria stream transport — the hysteria-specific knobs that ride
// alongside the connect target on outbound (and the inbound side too,
// where the listening peer needs matching auth / congestion / obfs).
// Wire shape mirrors xray-core's HysteriaConfig, with udphop nested
// when port-hopping is on and omitted otherwise.

export const HysteriaUdphopSchema = z.object({
  port: z.string().default(''),
  intervalMin: z.number().int().min(1).default(30),
  intervalMax: z.number().int().min(1).default(30),
});
export type HysteriaUdphop = z.infer<typeof HysteriaUdphopSchema>;

// `congestion` is `''` (BBR, the default) or `'brutal'`. Both empty and
// missing are equivalent on the wire so we accept either.
export const HysteriaCongestionSchema = z.union([z.literal(''), z.literal('brutal')]);

// Inbound-only masquerade sub-object. Xray's hysteria inbound can disguise
// itself as an HTTP server by serving static files (`type: 'file'`),
// reverse-proxying upstream traffic (`type: 'proxy'`), or returning a
// fixed string body (`type: 'string'`). Fields are loose-typed strings
// because the panel writes them as free-form input.
export const HysteriaMasqueradeSchema = z.object({
  type: z.enum(['proxy', 'file', 'string']).default('proxy'),
  dir: z.string().default(''),
  url: z.string().default(''),
  rewriteHost: z.boolean().default(false),
  insecure: z.boolean().default(false),
  content: z.string().default(''),
  headers: z.record(z.string(), z.string()).default({}),
  statusCode: z.number().int().min(0).default(0),
});
export type HysteriaMasquerade = z.infer<typeof HysteriaMasqueradeSchema>;

export const HysteriaStreamSettingsSchema = z.object({
  // Outbound-side fields. The version field is shared with inbound and
  // typically locked to 2.
  version: z.literal(2).default(2),
  auth: z.string().default(''),
  congestion: HysteriaCongestionSchema.default(''),
  // up / down are dash-separated bandwidth strings like '100 mbps' / '1 gbps'.
  // The panel stores them as free-form strings and Xray parses on the
  // server side; no client-side validation.
  up: z.string().default('0'),
  down: z.string().default('0'),
  udphop: HysteriaUdphopSchema.optional(),
  initStreamReceiveWindow: z.number().int().min(0).default(8388608),
  maxStreamReceiveWindow: z.number().int().min(0).default(8388608),
  initConnectionReceiveWindow: z.number().int().min(0).default(20971520),
  maxConnectionReceiveWindow: z.number().int().min(0).default(20971520),
  maxIdleTimeout: z.number().int().min(1).default(30),
  keepAlivePeriod: z.number().int().min(1).default(2),
  disablePathMTUDiscovery: z.boolean().default(false),
  // Inbound-side fields. xray-core's HysteriaConfig accepts both sets in
  // the same struct; outbound emits the bandwidth/udphop block, inbound
  // emits the protocol/udpIdleTimeout/masquerade block. The panel can
  // round-trip both shapes through this single schema.
  protocol: z.string().optional(),
  udpIdleTimeout: z.number().int().min(1).optional(),
  masquerade: HysteriaMasqueradeSchema.optional(),
});
export type HysteriaStreamSettings = z.infer<typeof HysteriaStreamSettingsSchema>;
