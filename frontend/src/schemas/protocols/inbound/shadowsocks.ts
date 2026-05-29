import { z } from 'zod';

export const SSMethodSchema = z.enum([
  'aes-256-gcm',
  'chacha20-poly1305',
  'chacha20-ietf-poly1305',
  'xchacha20-ietf-poly1305',
  '2022-blake3-aes-128-gcm',
  '2022-blake3-aes-256-gcm',
  '2022-blake3-chacha20-poly1305',
]);
export type SSMethod = z.infer<typeof SSMethodSchema>;

export const SSNetworkSchema = z.enum(['tcp', 'udp', 'tcp,udp']);
export type SSNetwork = z.infer<typeof SSNetworkSchema>;

// On a single-user shadowsocks inbound the client carries no method/password
// of its own — the inbound-level method+password are authoritative. On a
// 2022-blake3 multi-user setup each client provides its own password (and
// optionally a per-client method).
export const ShadowsocksClientSchema = z.object({
  method: z.string().default(''),
  password: z.string().default(''),
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
export type ShadowsocksClient = z.infer<typeof ShadowsocksClientSchema>;

export const ShadowsocksInboundSettingsSchema = z.object({
  method: SSMethodSchema.default('2022-blake3-aes-256-gcm'),
  password: z.string().default(''),
  network: SSNetworkSchema.default('tcp'),
  clients: z.array(ShadowsocksClientSchema).default([]),
  ivCheck: z.boolean().default(false),
});
export type ShadowsocksInboundSettings = z.infer<typeof ShadowsocksInboundSettingsSchema>;
