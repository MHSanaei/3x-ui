import { z } from 'zod';

// Extensions belong to the Happ client and must survive a trip through this editor.
export const HappRoutingProfileSchema = z
  .object({
    // The backend treats null lists as absent; preserve them until explicitly edited.
    DirectSites: z.array(z.string()).nullish(),
    DirectIp: z.array(z.string()).nullish(),
    ProxySites: z.array(z.string()).nullish(),
    ProxyIp: z.array(z.string()).nullish(),
    BlockSites: z.array(z.string()).nullish(),
    BlockIp: z.array(z.string()).nullish(),
  })
  .catchall(z.unknown());

export type HappRoutingProfile = z.infer<typeof HappRoutingProfileSchema>;
