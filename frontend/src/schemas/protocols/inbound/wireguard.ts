import { z } from 'zod';

export const WireguardDomainStrategySchema = z.enum([
  'ForceIP',
  'ForceIPv4',
  'ForceIPv4v6',
  'ForceIPv6',
  'ForceIPv6v4',
]);
export type WireguardDomainStrategy = z.infer<typeof WireguardDomainStrategySchema>;

// AntD InputNumber emits null (not undefined) when the user clears it, and
// the form store hands that null straight to safeParse on submit — a bare
// .optional() would reject it and block the save.
const optionalClearedInt = (schema: z.ZodNumber) =>
  z.preprocess((v) => (v == null ? undefined : v), schema.optional());

// Wireguard inbound is peer-based (no clients). Each peer is a client device
// the server accepts; secretKey is the server-side private key and pubKey is
// derived from it at runtime (not persisted on the wire). Inbound peers
// optionally store the client's privateKey so the panel can render configs
// for the user — outbound peers never have a privateKey.
export const WireguardInboundPeerSchema = z.object({
  privateKey: z.string().optional(),
  publicKey: z.string().min(1),
  preSharedKey: z.string().optional(),
  allowedIPs: z.array(z.string()).default([]),
  keepAlive: optionalClearedInt(z.number().int().min(0)),
  // Panel-only annotation (#5168): which client/device this peer belongs to.
  // Rides along in the settings JSON like privateKey does; xray-core ignores
  // unknown peer fields.
  comment: z.string().optional(),
});
export type WireguardInboundPeer = z.infer<typeof WireguardInboundPeerSchema>;

// A WireGuard inbound client (multi-client model). Each client is one peer the
// server accepts: the panel stores its keypair so it can render a full .conf/QR,
// and allowedIPs is the client's unique tunnel address (allocated server-side
// when left blank). Keys are optional on the wire — the backend generates them
// when absent.
export const WireguardClientSchema = z.object({
  privateKey: z.string().optional(),
  publicKey: z.string().optional(),
  preSharedKey: z.string().optional(),
  allowedIPs: z.array(z.string()).default([]),
  keepAlive: optionalClearedInt(z.number().int().min(0)),
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
export type WireguardClient = z.infer<typeof WireguardClientSchema>;

export const WireguardInboundSettingsSchema = z.object({
  mtu: optionalClearedInt(z.number().int().min(1)),
  secretKey: z.string().min(1),
  peers: z.array(WireguardInboundPeerSchema).default([]),
  clients: z.array(WireguardClientSchema).default([]),
  noKernelTun: z.boolean().default(false),
  domainStrategy: WireguardDomainStrategySchema.optional(),
});
export type WireguardInboundSettings = z.infer<typeof WireguardInboundSettingsSchema>;
