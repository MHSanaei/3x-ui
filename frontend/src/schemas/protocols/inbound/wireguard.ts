import { z } from 'zod';

export const WireguardDomainStrategySchema = z.enum([
  'ForceIP',
  'ForceIPv4',
  'ForceIPv4v6',
  'ForceIPv6',
  'ForceIPv6v4',
]);
export type WireguardDomainStrategy = z.infer<typeof WireguardDomainStrategySchema>;

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
  keepAlive: z.number().int().min(0).optional(),
});
export type WireguardInboundPeer = z.infer<typeof WireguardInboundPeerSchema>;

export const WireguardInboundSettingsSchema = z.object({
  mtu: z.number().int().min(1).optional(),
  secretKey: z.string().min(1),
  peers: z.array(WireguardInboundPeerSchema).default([]),
  noKernelTun: z.boolean().default(false),
  workers: z.number().int().min(1).optional(),
  domainStrategy: WireguardDomainStrategySchema.optional(),
});
export type WireguardInboundSettings = z.infer<typeof WireguardInboundSettingsSchema>;
