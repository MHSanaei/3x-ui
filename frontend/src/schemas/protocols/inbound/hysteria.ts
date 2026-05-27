import { z } from 'zod';

// Hysteria v1 inbound (legacy — upstream xray-core kept v1 support but the
// panel defaults to v2). Each client supplies an `auth` token instead of a
// UUID/password.
export const HysteriaClientSchema = z.object({
  auth: z.string().min(1),
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
export type HysteriaClient = z.infer<typeof HysteriaClientSchema>;

export const HysteriaInboundSettingsSchema = z.object({
  version: z.number().int().min(1).default(2),
  clients: z.array(HysteriaClientSchema).default([]),
});
export type HysteriaInboundSettings = z.infer<typeof HysteriaInboundSettingsSchema>;
