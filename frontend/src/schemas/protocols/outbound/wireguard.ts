import { z } from 'zod';

export const WireguardDomainStrategySchema = z.enum([
  'ForceIP',
  'ForceIPv4',
  'ForceIPv4v6',
  'ForceIPv6',
  'ForceIPv6v4',
]);
export type WireguardDomainStrategy = z.infer<typeof WireguardDomainStrategySchema>;

export const WireguardOutboundPeerSchema = z.object({
  publicKey: z.string().min(1),
  preSharedKey: z.string().optional(),
  allowedIPs: z.array(z.string()).default(['0.0.0.0/0', '::/0']),
  endpoint: z.string().min(1),
  keepAlive: z.number().int().min(0).optional(),
});
export type WireguardOutboundPeer = z.infer<typeof WireguardOutboundPeerSchema>;

export const WireguardOutboundSettingsSchema = z.object({
  mtu: z.number().int().min(1).optional(),
  secretKey: z.string().min(1),
  address: z.array(z.string()).default([]),
  domainStrategy: WireguardDomainStrategySchema.optional(),
  reserved: z.array(z.number().int()).optional(),
  peers: z.array(WireguardOutboundPeerSchema).min(1),
  noKernelTun: z.boolean().default(false),
});
export type WireguardOutboundSettings = z.infer<typeof WireguardOutboundSettingsSchema>;
