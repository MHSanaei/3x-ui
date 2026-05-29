import { z } from 'zod';

import { PortSchema } from '@/schemas/primitives';

export const TunnelNetworkSchema = z.enum(['tcp', 'udp', 'tcp,udp']);
export type TunnelNetwork = z.infer<typeof TunnelNetworkSchema>;

// Tunnel inbound (Xray's `dokodemo-door`-style transparent forwarder).
// `portMap` is persisted as Record<string, string> on the wire — the panel
// flattens an internal array-of-{name,value} into that map via toV2Headers
// with arr=false.
export const TunnelInboundSettingsSchema = z.object({
  rewriteAddress: z.string().optional(),
  rewritePort: PortSchema.optional(),
  portMap: z.record(z.string(), z.string()).default({}),
  allowedNetwork: TunnelNetworkSchema.default('tcp,udp'),
  followRedirect: z.boolean().default(false),
});
export type TunnelInboundSettings = z.infer<typeof TunnelInboundSettingsSchema>;
