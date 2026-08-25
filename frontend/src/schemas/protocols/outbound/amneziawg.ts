import { z } from 'zod';

// Wire format of an "amneziawg" OUTBOUND's settings block, stored verbatim
// inside the panel's Xray template. At config generation the backend swaps
// the whole entry for a loopback socks bridge (internal/web/service/
// xray_amneziawg_outbound.go); this schema is what the form edits and what
// the backend validates (internal/amneziawg.ValidateAmneziaWGOutbound).
export const AmneziaWGOutboundPeerSchema = z.object({
  publicKey: z.string().default(''),
  presharedKey: z.string().default(''),
  allowedIPs: z.array(z.string()).default(['0.0.0.0/0', '::/0']),
  endpoint: z.string().default(''),
  keepAlive: z.number().int().min(0).default(0),
});
export type AmneziaWGOutboundPeer = z.infer<typeof AmneziaWGOutboundPeerSchema>;

export const AmneziaWGOutboundSettingsSchema = z.object({
  mtu: z.number().int().min(0).default(1420),
  secretKey: z.string().default(''),
  address: z.array(z.string()).default([]),
  listenPort: z.number().int().min(0).max(65535).default(0),
  jc: z.number().int().min(0).default(0),
  jmin: z.number().int().min(0).default(40),
  jmax: z.number().int().min(0).default(100),
  s1: z.number().int().min(0).default(15),
  s2: z.number().int().min(0).default(80),
  s3: z.number().int().min(0).max(64).default(12),
  s4: z.number().int().min(0).max(32).default(12),
  h1: z.string().default(''),
  h2: z.string().default(''),
  h3: z.string().default(''),
  h4: z.string().default(''),
  i1: z.string().default(''),
  i2: z.string().default(''),
  i3: z.string().default(''),
  i4: z.string().default(''),
  i5: z.string().default(''),
  headerProtectionKey: z.string().default(''),
  contentPaddingAddition: z.string().default(''),
  rekeyAfterTime: z.string().default(''),
  rekeyTimeout: z.string().default(''),
  rejectAfterTime: z.string().default(''),
  keepaliveTimeout: z.string().default(''),
  maxHandshakeAttempts: z.string().default(''),
  randomTrailers: z.boolean().default(true),
  disableCookies: z.boolean().default(true),
  peers: z.array(AmneziaWGOutboundPeerSchema).default([]),
});
export type AmneziaWGOutboundSettings = z.infer<typeof AmneziaWGOutboundSettingsSchema>;
