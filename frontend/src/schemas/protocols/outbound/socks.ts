import { z } from 'zod';

import { PortSchema } from '@/schemas/primitives';

// SOCKS outbound persists in Xray's `servers[].users[]` shape — wire-identical
// to HTTP outbound but with `socks` as the parent protocol literal. The panel
// only supports a single server with at most one user.
export const SocksOutboundUserSchema = z.object({
  user: z.string().min(1),
  pass: z.string().min(1),
});
export type SocksOutboundUser = z.infer<typeof SocksOutboundUserSchema>;

export const SocksOutboundServerSchema = z.object({
  address: z.string().min(1),
  port: PortSchema,
  users: z.array(SocksOutboundUserSchema).default([]),
});
export type SocksOutboundServer = z.infer<typeof SocksOutboundServerSchema>;

export const SocksOutboundSettingsSchema = z.object({
  servers: z.array(SocksOutboundServerSchema).min(1),
});
export type SocksOutboundSettings = z.infer<typeof SocksOutboundSettingsSchema>;
