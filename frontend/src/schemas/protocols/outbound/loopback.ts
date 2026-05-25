import { z } from 'zod';

// Loopback outbound reinjects traffic back into a named inbound for chained
// routing. The single `inboundTag` field references an inbound tag by name.
export const LoopbackOutboundSettingsSchema = z.object({
  inboundTag: z.string().optional(),
});
export type LoopbackOutboundSettings = z.infer<typeof LoopbackOutboundSettingsSchema>;
