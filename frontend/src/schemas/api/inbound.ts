import { z } from 'zod';

import { InboundPortSchema, SniffingSchema } from '@/schemas/primitives';
import { InboundSettingsSchema } from '@/schemas/protocols/inbound';
import { SecuritySettingsSchema } from '@/schemas/protocols/security';
import { NetworkSettingsSchema, StreamExtrasSchema } from '@/schemas/protocols/stream';

// Top-level inbound shape on the wire. Composes:
//   - Per-protocol settings via the InboundSettingsSchema discriminated
//     union (10 protocols, tagged-wrapper {protocol, settings}).
//   - StreamSettings as an intersection of the network DU (6 branches),
//     security DU (3 branches), and the orthogonal extras (finalmask,
//     sockopt, externalProxy). Zod 4 supports DU intersection — each
//     branch validates its slice of the same input object.
//
// The id/up/down/total/expiryTime fields are int64 on the Go side but
// the panel ships them as JS numbers. Numbers above Number.MAX_SAFE_INTEGER
// (~9e15) lose precision; the panel works around this for the traffic
// counters by stringifying them at the API edge. Not modeled here.

export const StreamSettingsSchema = NetworkSettingsSchema
  .and(SecuritySettingsSchema)
  .and(StreamExtrasSchema);
export type StreamSettings = z.infer<typeof StreamSettingsSchema>;

export const InboundCoreSchema = z.object({
  id: z.number().int().optional(),
  up: z.number().int().min(0).default(0),
  down: z.number().int().min(0).default(0),
  total: z.number().int().min(0).default(0),
  remark: z.string().default(''),
  enable: z.boolean().default(true),
  expiryTime: z.number().int().default(0),
  listen: z.string().default(''),
  port: InboundPortSchema,
  tag: z.string().default(''),
  sniffing: SniffingSchema.default({
    enabled: false,
    destOverride: ['http', 'tls', 'quic', 'fakedns'],
    metadataOnly: false,
    routeOnly: false,
    ipsExcluded: [],
    domainsExcluded: [],
  }),
  streamSettings: StreamSettingsSchema.optional(),
  clientStats: z.string().optional(),
});
export type InboundCore = z.infer<typeof InboundCoreSchema>;

// Full Inbound = core fields + the protocol/settings discriminated union.
// Consumers narrow on `.protocol` to access the matching `.settings`
// branch with full type safety.
export const InboundSchema = InboundCoreSchema.and(InboundSettingsSchema);
export type Inbound = z.infer<typeof InboundSchema>;

// SlimInbound is the list-view projection — same shape minus settings
// and streamSettings (the list endpoint omits both to keep payload
// small). Used by InboundsPage list rendering.
export const SlimInboundSchema = InboundCoreSchema.omit({
  streamSettings: true,
}).extend({
  protocol: z.string(),
});
export type SlimInbound = z.infer<typeof SlimInboundSchema>;
