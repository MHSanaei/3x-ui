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

export const HysteriaStreamSettingsSchema = z.object({
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
});
export type HysteriaStreamSettings = z.infer<typeof HysteriaStreamSettingsSchema>;
