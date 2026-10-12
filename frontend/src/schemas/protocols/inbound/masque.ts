import { z } from 'zod';

// MASQUE clients log in with email and password over Basic auth; the panel stores
// `password` and the backend hands it to xray-core as `pass`.
export const MasqueClientSchema = z.object({
  password: z.string().min(1),
  email: z.string().min(1),
  limitIp: z.number().int().min(0).default(0),
  totalGB: z.number().int().min(0).default(0),
  expiryTime: z.number().int().default(0),
  enable: z.boolean().default(true),
  tgId: z
    .union([z.number(), z.string()])
    .transform((v) => Number(v) || 0)
    .default(0),
  subId: z.string().default(''),
  comment: z.string().default(''),
  reset: z.number().int().min(0).default(0),
  created_at: z.number().int().optional(),
  updated_at: z.number().int().optional(),
});
export type MasqueClient = z.infer<typeof MasqueClientSchema>;

export const MASQUE_DEFAULT_ADDRESS_POOL = ['10.14.0.1/24', 'fd14::1/64'];

export const MasqueInboundSettingsSchema = z.object({
  clients: z.array(MasqueClientSchema).default([]),
  // Tunnel addresses are leased from these prefixes: at most one IPv4 and one IPv6.
  address: z.array(z.string()).default(MASQUE_DEFAULT_ADDRESS_POOL),
  mtu: z.number().int().min(1280).max(65535).optional(),
});
export type MasqueInboundSettings = z.infer<typeof MasqueInboundSettingsSchema>;
