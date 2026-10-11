import { z } from 'zod';

// The path xray-core's MASQUE transport uses when none is set; the server matches
// a request on it after expanding {target}/{ipproto} to "*".
export const MASQUE_DEFAULT_PATH = '/.well-known/masque/ip/*/*/';

// WARP over MASQUE: an enrolled ECDSA P-256 key (PEM or base64 DER), the endpoint's
// public key, and the tunnel addresses Cloudflare assigned (one IPv4, one IPv6).
export const MasqueWarpSchema = z.object({
  privateKey: z.string().default(''),
  publicKey: z.string().default(''),
  address: z.array(z.string()).default([]),
});
export type MasqueWarp = z.infer<typeof MasqueWarpSchema>;

// A server reads only `path`; host, user/pass (Basic auth), headers and warp are the
// client side. Every key is optional so a stored stream round-trips byte-stably.
export const MasqueStreamSettingsSchema = z.object({
  host: z.string().optional(),
  path: z.string().optional(),
  user: z.string().optional(),
  pass: z.string().optional(),
  headers: z.record(z.string(), z.string()).optional(),
  warp: MasqueWarpSchema.optional(),
});
export type MasqueStreamSettings = z.infer<typeof MasqueStreamSettingsSchema>;
