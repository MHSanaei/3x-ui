import { z } from 'zod';

import { FlowSchema, SniffingSchema } from '@/schemas/primitives';

export const VlessOutboundSettingsSchema = z.object({
  address: z.string(),
  port: z.number().int().min(1).max(65535),
  id: z.uuid(),
  flow: FlowSchema.default(''),
  encryption: z.string().min(1).default('none'),
  reverse: z
    .object({
      tag: z.string(),
      sniffing: SniffingSchema.optional(),
    })
    .optional(),
  testpre: z.number().int().min(0).optional(),
  // TODO: narrow to flow === 'xtls-rprx-vision' once a per-flow discriminator
  // exists.
  testseed: z.array(z.number().int().positive()).length(4).optional(),
});
export type VlessOutboundSettings = z.infer<typeof VlessOutboundSettingsSchema>;
