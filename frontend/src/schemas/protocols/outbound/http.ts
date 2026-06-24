import { z } from 'zod';

import { PortSchema } from '@/schemas/primitives';

// HTTP outbound persists in Xray's `servers[].users[]` shape. The panel only
// supports a single server with at most one user (the constructor reads
// servers[0] / users[0]). We model the wire shape rather than the panel's
// flattened class fields so saves round-trip exactly.
export const HttpOutboundUserSchema = z.object({
  user: z.string().min(1),
  pass: z.string().min(1),
});
export type HttpOutboundUser = z.infer<typeof HttpOutboundUserSchema>;

export const HttpOutboundServerSchema = z.object({
  address: z.string().min(1),
  port: PortSchema,
  users: z.array(HttpOutboundUserSchema).default([]),
});
export type HttpOutboundServer = z.infer<typeof HttpOutboundServerSchema>;

export const HttpOutboundSettingsSchema = z.object({
  servers: z.array(HttpOutboundServerSchema).min(1),
  headers: z.record(z.string(), z.string()).optional(),
});
export type HttpOutboundSettings = z.infer<typeof HttpOutboundSettingsSchema>;
