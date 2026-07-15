import { z } from 'zod';

export const NaiveOutboundSettingsSchema = z.object({
  proxy: z.string().default(''),
  insecureConcurrency: z.number().int().min(1).max(8).optional(),
  tunnelTimeout: z.number().int().min(0).optional(),
  idleTimeout: z.number().int().min(0).optional(),
  extraHeaders: z.string().optional(),
  hostResolverRules: z.string().optional(),
  resolverRange: z.string().optional(),
  noPostQuantum: z.boolean().optional(),
});

export type NaiveOutboundSettings = z.infer<typeof NaiveOutboundSettingsSchema>;