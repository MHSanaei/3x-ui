import { z } from 'zod';

import { SniffingSchema } from '@/schemas/primitives';

export const LoopbackOutboundSettingsSchema = z.object({
  inboundTag: z.string().optional(),
  sniffing: SniffingSchema.optional(),
});
export type LoopbackOutboundSettings = z.infer<typeof LoopbackOutboundSettingsSchema>;
