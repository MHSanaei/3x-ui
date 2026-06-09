import { z } from 'zod';

import { VmessSecuritySchema } from '../shared/vmess';

export const VmessClientSchema = z.object({
  id: z.string().min(1),
  security: VmessSecuritySchema.default('auto'),
  alterId: z.number().int().min(0).default(0),
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
export type VmessClient = z.infer<typeof VmessClientSchema>;

export const VmessInboundSettingsSchema = z.object({
  clients: z.array(VmessClientSchema).default([]),
});
export type VmessInboundSettings = z.infer<typeof VmessInboundSettingsSchema>;
