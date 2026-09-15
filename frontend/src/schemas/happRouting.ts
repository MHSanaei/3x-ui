import { z } from 'zod';

// Extensions belong to the Happ client and must survive a trip through this editor.
export const HappRoutingProfileSchema = z
  .object({
    DirectSites: z.array(z.string()).optional(),
    DirectIp: z.array(z.string()).optional(),
    ProxySites: z.array(z.string()).optional(),
    ProxyIp: z.array(z.string()).optional(),
    BlockSites: z.array(z.string()).optional(),
    BlockIp: z.array(z.string()).optional(),
  })
  .catchall(z.unknown());

export type HappRoutingProfile = z.infer<typeof HappRoutingProfileSchema>;
