import { z } from 'zod';

// AntD InputNumber emits null (not undefined) when the user clears it, and
// the form store hands that null straight to safeParse on submit — a bare
// .optional() would reject it and block the save.
const optionalClearedInt = (schema: z.ZodNumber) =>
  z.preprocess((v) => (v == null ? undefined : v), schema.optional());

// An AmneziaWG client (multi-client model). Same key/address fields as
// WireguardClientSchema — the panel's generic ClientRecord already has those
// exact keys (privateKey/publicKey/preSharedKey/allowedIPs/keepAlive), so
// bulk operations, the QR modal and subscriptions all work unmodified — plus
// one AmneziaWG-only addition, forwardedPorts (WireGuard's Xray-native
// inbound has no host-level iptables layer to hang per-client DNAT off of).
// Keys are optional on the wire — the backend generates them when absent.
export const AmneziawgClientSchema = z.object({
  privateKey: z.string().optional(),
  publicKey: z.string().optional(),
  preSharedKey: z.string().optional(),
  allowedIPs: z.array(z.string()).default([]),
  keepAlive: optionalClearedInt(z.number().int().min(0)),
  forwardedPorts: z.string().default(''),
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
export type AmneziawgClient = z.infer<typeof AmneziawgClientSchema>;

// Server-wide AmneziaWG 3.1 obfuscation parameters and tunnel identity,
// mirroring internal/amneziawg.ServerSettings on the Go side exactly (same
// field names) — the listen port is not duplicated here, it's the inbound's
// own port like every other protocol. H1-H4 blank falls back to the classic
// 1/2/3/4 magic header on save; blank optional fields omit their feature
// from the rendered config.
export const AmneziawgServerSchema = z.object({
  privateKey: z.string().optional(),
  publicKey: z.string().optional(),
  subnetIp: z.string().default('10.8.1.0'),
  subnetCidr: z.number().int().min(1).max(32).default(24),
  mtu: optionalClearedInt(z.number().int().min(1)),
  primaryDns: z.string().default('8.8.8.8'),
  secondaryDns: z.string().default('8.8.4.4'),
  externalInterface: z.string().default(''),
  ipv6Enabled: z.boolean().default(false),
  ipv6Subnet: z.string().default(''),
  ipv6ExternalInterface: z.string().default(''),
  routeThroughXray: z.boolean().default(false),
  jc: z.number().int().min(0).default(5),
  jmin: z.number().int().min(0).default(10),
  jmax: z.number().int().min(0).default(50),
  s1: z.number().int().min(0).default(30),
  s2: z.number().int().min(0).default(45),
  s3: z.number().int().min(0).max(64).default(10),
  s4: z.number().int().min(0).max(32).default(5),
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
  randomTrailers: z.boolean().default(false),
  disableCookies: z.boolean().default(false),
});
export type AmneziawgServer = z.infer<typeof AmneziawgServerSchema>;

export const AmneziawgInboundSettingsSchema = z.object({
  server: AmneziawgServerSchema,
  clients: z.array(AmneziawgClientSchema).default([]),
});
export type AmneziawgInboundSettings = z.infer<typeof AmneziawgInboundSettingsSchema>;
