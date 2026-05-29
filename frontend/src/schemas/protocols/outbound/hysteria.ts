import { z } from 'zod';

import { PortSchema } from '@/schemas/primitives';

// Hysteria outbound is a thin connect-target descriptor — the actual auth and
// transport knobs live on the stream/transport layer, not in settings.
export const HysteriaOutboundSettingsSchema = z.object({
  address: z.string().min(1),
  port: PortSchema,
  version: z.number().int().min(1).default(2),
});
export type HysteriaOutboundSettings = z.infer<typeof HysteriaOutboundSettingsSchema>;
