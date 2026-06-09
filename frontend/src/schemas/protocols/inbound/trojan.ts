import { z } from 'zod';

export const TrojanFallbackSchema = z.object({
  name: z.string().default(''),
  alpn: z.string().default(''),
  path: z.string().default(''),
  dest: z.union([z.string(), z.number()]).default(''),
  xver: z.number().int().min(0).default(0),
});
export type TrojanFallback = z.infer<typeof TrojanFallbackSchema>;

export const TrojanClientSchema = z.object({
  password: z.string().min(1),
  email: z.string().min(1),
  limitIp: z.number().int().min(0).default(0),
  totalGB: z.number().int().min(0).default(0),
  expiryTime: z.number().int().default(0),
  enable: z.boolean().default(true),
  tgId: z.union([z.number(), z.string()]).transform((v) => Number(v) || 0).default(0),
  subId: z.string().default(''),
  comment: z.string().default(''),
  reset: z.number().int().min(0).default(0),
  created_at: z.number().int().optional(),
  updated_at: z.number().int().optional(),
});
export type TrojanClient = z.infer<typeof TrojanClientSchema>;

export const TrojanInboundSettingsSchema = z.object({
  clients: z.array(TrojanClientSchema).default([]),
  fallbacks: z.array(TrojanFallbackSchema).default([]),
});
export type TrojanInboundSettings = z.infer<typeof TrojanInboundSettingsSchema>;
