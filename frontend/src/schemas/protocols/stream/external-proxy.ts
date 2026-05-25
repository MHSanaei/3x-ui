import { z } from 'zod';

import { PortSchema } from '@/schemas/primitives';

import { AlpnSchema, UtlsFingerprintSchema } from '@/schemas/protocols/security/tls';

export const ExternalProxyForceTlsSchema = z.enum(['same', 'tls', 'none']);
export type ExternalProxyForceTls = z.infer<typeof ExternalProxyForceTlsSchema>;

// An inbound can advertise external proxy fronts (CDN edges, mirror nodes)
// that share its config but vary the dest+port+SNI for the share link. The
// panel form ships rows of this shape; link generators iterate them when
// stream.externalProxy is non-empty.
export const ExternalProxyEntrySchema = z.object({
  forceTls: ExternalProxyForceTlsSchema.default('same'),
  dest: z.string().default(''),
  port: PortSchema.default(443),
  remark: z.string().default(''),
  sni: z.string().optional(),
  fingerprint: UtlsFingerprintSchema.optional(),
  alpn: z.array(AlpnSchema).optional(),
});
export type ExternalProxyEntry = z.infer<typeof ExternalProxyEntrySchema>;
