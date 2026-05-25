import { z } from 'zod';

import { PortSchema } from '@/schemas/primitives';

// Outbound counterpart to hysteria2 — same {address, port} connect descriptor
// as hysteria, but version locked to 2.
export const Hysteria2OutboundSettingsSchema = z.object({
  address: z.string().min(1),
  port: PortSchema,
  version: z.literal(2).default(2),
});
export type Hysteria2OutboundSettings = z.infer<typeof Hysteria2OutboundSettingsSchema>;
