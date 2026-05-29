import { z } from 'zod';

export const SniffingDestSchema = z.enum(['http', 'tls', 'quic', 'fakedns']);
export type SniffingDest = z.infer<typeof SniffingDestSchema>;

export const SniffingSchema = z.object({
  enabled: z.boolean().default(false),
  destOverride: z
    .array(SniffingDestSchema)
    .default(['http', 'tls', 'quic', 'fakedns']),
  metadataOnly: z.boolean().default(false),
  routeOnly: z.boolean().default(false),
  ipsExcluded: z.array(z.string()).default([]),
  domainsExcluded: z.array(z.string()).default([]),
});
export type Sniffing = z.infer<typeof SniffingSchema>;
