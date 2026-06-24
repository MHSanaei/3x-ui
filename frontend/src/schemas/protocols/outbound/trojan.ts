import { z } from 'zod';

import { PortSchema } from '@/schemas/primitives';

export const TrojanOutboundServerSchema = z.object({
  address: z.string().min(1),
  port: PortSchema,
  password: z.string().min(1),
});
export type TrojanOutboundServer = z.infer<typeof TrojanOutboundServerSchema>;

export const TrojanOutboundSettingsSchema = z.object({
  servers: z.array(TrojanOutboundServerSchema).min(1),
});
export type TrojanOutboundSettings = z.infer<typeof TrojanOutboundSettingsSchema>;
