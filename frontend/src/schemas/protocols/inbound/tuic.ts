import { z } from 'zod';

const optionalClearedInt = (schema: z.ZodNumber) =>
  z.preprocess((v) => (v == null ? undefined : v), schema.optional());

const clearedToDefault = <T extends z.ZodType>(schema: T) =>
  z.preprocess((v) => (v == null ? undefined : v), schema);

export const TuicClientSchema = z.object({
  uuid: z.string().optional(),
  id: z.string().optional(),
  password: z.string().default(''),
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
export type TuicClient = z.infer<typeof TuicClientSchema>;

export const TuicServerSchema = z.object({
  certificate: z.string().default(''),
  private_key: z.string().default(''),
  congestion_control: z.enum(['bbr', 'cubic', 'new_reno']).default('bbr'),
  alpn: z.array(z.string()).default(['h3', 'spdy/3.1']),
  udp_relay_mode: z.enum(['native', 'quic']).default('native'),
  zero_rtt_handshake: z.boolean().default(true),
  log_level: z.enum(['info', 'warn', 'error', 'debug']).default('info'),
  max_idle_time: clearedToDefault(z.number().int().min(1).default(15)),
  authentication_timeout: clearedToDefault(z.number().int().min(1).default(3)),
  max_udp_relay_packet_size: clearedToDefault(z.number().int().min(1).default(1500)),
  sni: z.string().default(''),
});
export type TuicServer = z.infer<typeof TuicServerSchema>;

export const TuicInboundSettingsSchema = z.object({
  server: TuicServerSchema.optional(),
  certificate: z.string().optional(),
  private_key: z.string().optional(),
  congestion_control: z.string().optional(),
  alpn: z.array(z.string()).optional(),
  udp_relay_mode: z.string().optional(),
  zero_rtt_handshake: z.boolean().optional(),
  log_level: z.string().optional(),
  max_idle_time: optionalClearedInt(z.number().int().min(1)),
  authentication_timeout: optionalClearedInt(z.number().int().min(1)),
  max_udp_relay_packet_size: optionalClearedInt(z.number().int().min(1)),
  sni: z.string().optional(),
  clients: z.array(TuicClientSchema).default([]),
});
export type TuicInboundSettings = z.infer<typeof TuicInboundSettingsSchema>;
