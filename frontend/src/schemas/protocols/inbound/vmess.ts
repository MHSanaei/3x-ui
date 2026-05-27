import { z } from 'zod';

const VmessSecurityEnum = z.enum([
  'aes-128-gcm',
  'chacha20-poly1305',
  'auto',
  'none',
  'zero',
]);

// Legacy rows persisted `security: ""` (especially on VMess inbounds
// created before the enum was nailed down). Preprocess maps the empty
// string back to the documented default so existing data parses cleanly
// — subsequent writes serialize the normalized value.
export const VmessSecuritySchema = z.preprocess(
  (val) => (val === '' ? 'auto' : val),
  VmessSecurityEnum,
);
export type VmessSecurity = z.infer<typeof VmessSecurityEnum>;

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
