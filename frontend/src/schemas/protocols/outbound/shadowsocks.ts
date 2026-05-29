import { z } from 'zod';

import { PortSchema } from '@/schemas/primitives';
import { SSMethodSchema } from '@/schemas/protocols/inbound/shadowsocks';

// Shadowsocks outbound persists as { servers: [{ ... }] }, with UDP-over-TCP
// knobs (uot, UoTVersion) attached per-server when the user enabled them.
export const ShadowsocksOutboundServerSchema = z.object({
  address: z.string().min(1),
  port: PortSchema,
  password: z.string().min(1),
  method: SSMethodSchema,
  uot: z.boolean().optional(),
  UoTVersion: z.number().int().min(1).max(2).optional(),
});
export type ShadowsocksOutboundServer = z.infer<typeof ShadowsocksOutboundServerSchema>;

export const ShadowsocksOutboundSettingsSchema = z.object({
  servers: z.array(ShadowsocksOutboundServerSchema).min(1),
});
export type ShadowsocksOutboundSettings = z.infer<typeof ShadowsocksOutboundSettingsSchema>;
