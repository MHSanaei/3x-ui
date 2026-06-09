import { z } from 'zod';

import { FlowSchema, SniffingSchema } from '@/schemas/primitives';

export const VlessFallbackSchema = z.object({
  name: z.string().default(''),
  alpn: z.string().default(''),
  path: z.string().default(''),
  dest: z.union([z.string(), z.number()]).default(''),
  xver: z.number().int().min(0).default(0),
});
export type VlessFallback = z.infer<typeof VlessFallbackSchema>;

export const VlessClientSchema = z.object({
  id: z.string().min(1),
  email: z.string().min(1),
  flow: FlowSchema.default(''),
  limitIp: z.number().int().min(0).default(0),
  totalGB: z.number().int().min(0).default(0),
  expiryTime: z.number().int().default(0),
  enable: z.boolean().default(true),
  tgId: z.union([z.number(), z.string()]).transform((v) => Number(v) || 0).default(0),
  subId: z.string().default(''),
  comment: z.string().default(''),
  reset: z.number().int().min(0).default(0),
  // VLESS simple reverse-proxy: which reverse tag this client routes to,
  // plus an optional sniffing override for that path. Distinct from the
  // inbound-level `fallbacks` feature.
  reverse: z
    .object({
      tag: z.string(),
      sniffing: SniffingSchema.optional(),
    })
    .optional(),
  created_at: z.number().int().optional(),
  updated_at: z.number().int().optional(),
});
export type VlessClient = z.infer<typeof VlessClientSchema>;

export const VlessInboundSettingsSchema = z.object({
  clients: z.array(VlessClientSchema).default([]),
  decryption: z.string().min(1).default('none'),
  encryption: z.string().min(1).default('none'),
  fallbacks: z.array(VlessFallbackSchema).default([]),
  // TODO: narrow to flow === 'xtls-rprx-vision' once a per-flow discriminator
  // exists. 4-positive-int padding seed for xtls-rprx-vision; backend uses
  // safe defaults when omitted.
  testseed: z.array(z.number().int().positive()).length(4).optional(),
});
export type VlessInboundSettings = z.infer<typeof VlessInboundSettingsSchema>;
