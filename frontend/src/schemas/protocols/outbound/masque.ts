import { z } from 'zod';

import { PortSchema } from '@/schemas/primitives';

// MASQUE outbound names its server only; credentials, path and WARP ride on the
// masque transport. remoteDNS lists IPs queried inside the CONNECT-IP tunnel.
export const MasqueOutboundSettingsSchema = z.object({
  address: z.string().min(1),
  port: PortSchema,
  remoteDNS: z.array(z.string()).optional(),
});
export type MasqueOutboundSettings = z.infer<typeof MasqueOutboundSettingsSchema>;
