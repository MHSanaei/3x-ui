import { z } from 'zod';

export const AmneziaWgClientSchema = z.object({
  email: z.string().min(1),
  privateKey: z.string().optional(),
  publicKey: z.string().optional(),
  presharedKey: z.string().optional(),
  assignedIp: z.string().optional(),
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
export type AmneziaWgClient = z.infer<typeof AmneziaWgClientSchema>;

export const AmneziaWgServerSchema = z.object({
  privateKey: z.string().min(1),
  publicKey: z.string().min(1),
  psk: z.string().min(1),
  jc: z.number().int().default(5),
  jmin: z.number().int().default(10),
  jmax: z.number().int().default(50),
  s1: z.number().int().default(30),
  s2: z.number().int().default(45),
  s3: z.number().int().default(10),
  s4: z.number().int().default(5),
  h1: z.string().default(''),
  h2: z.string().default(''),
  h3: z.string().default(''),
  h4: z.string().default(''),
  subnetIp: z.string().default('10.8.1.0'),
  subnetCidr: z.number().int().default(24),
  serverPort: z.number().int().default(55424),
  primaryDns: z.string().default('8.8.8.8'),
  secondaryDns: z.string().default('8.8.4.4'),
});
export type AmneziaWgServer = z.infer<typeof AmneziaWgServerSchema>;

export const AmneziaWgInboundSettingsSchema = z.object({
  server: AmneziaWgServerSchema,
  clients: z.array(AmneziaWgClientSchema).default([]),
});
export type AmneziaWgInboundSettings = z.infer<typeof AmneziaWgInboundSettingsSchema>;
