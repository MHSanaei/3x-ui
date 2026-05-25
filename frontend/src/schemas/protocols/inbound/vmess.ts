import { z } from 'zod';

export const VmessSecuritySchema = z.enum([
  'aes-128-gcm',
  'chacha20-poly1305',
  'auto',
  'none',
  'zero',
]);
export type VmessSecurity = z.infer<typeof VmessSecuritySchema>;

export const VmessClientSchema = z.object({
  id: z.uuid(),
  security: VmessSecuritySchema.default('auto'),
  email: z.string().min(1),
  limitIp: z.number().int().min(0).default(0),
  totalGB: z.number().int().min(0).default(0),
  expiryTime: z.number().int().default(0),
  enable: z.boolean().default(true),
  tgId: z.number().int().default(0),
  subId: z.string().default(''),
  comment: z.string().default(''),
  reset: z.number().int().min(0).default(0),
  created_at: z.number().int().optional(),
  updated_at: z.number().int().optional(),
});
export type VmessClient = z.infer<typeof VmessClientSchema>;

export const VmessInboundSettingsSchema = z.object({
  clients: z.array(VmessClientSchema).default([]),
});
export type VmessInboundSettings = z.infer<typeof VmessInboundSettingsSchema>;
