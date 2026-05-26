import { z } from 'zod';

// Hysteria stream transport. Per Xray docs (transports/hysteria.html), the
// Xray implementation of Hysteria2's underlying QUIC transport keeps only
// the essentials — version, auth, udpIdleTimeout, and masquerade. The
// extended bandwidth/window/udphop knobs that earlier hysteria builds
// exposed are not part of this transport's wire shape.

// Inbound masquerade — Xray's hysteria inbound can disguise itself as an
// HTTP/3 server. `type` is the empty string by default (serves the default
// 404 page), and per-type config keys are only honored when their type is
// active.
export const HysteriaMasqueradeSchema = z.object({
  type: z.enum(['', 'proxy', 'file', 'string']).default(''),
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
  version: z.literal(2).default(2),
  auth: z.string().default(''),
  udpIdleTimeout: z.number().int().min(1).default(60),
  masquerade: HysteriaMasqueradeSchema.optional(),
});
export type HysteriaStreamSettings = z.infer<typeof HysteriaStreamSettingsSchema>;
