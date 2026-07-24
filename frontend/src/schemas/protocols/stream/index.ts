import { z } from 'zod';

import { ExternalProxyEntrySchema } from './external-proxy';
import { FinalMaskStreamSettingsSchema } from './finalmask';
import { GrpcStreamSettingsSchema } from './grpc';
import { HttpUpgradeStreamSettingsSchema } from './httpupgrade';
import { HysteriaStreamSettingsSchema } from './hysteria';
import { KcpStreamSettingsSchema } from './kcp';
import { SockoptStreamSettingsSchema } from './sockopt';
import { TcpStreamSettingsSchema } from './tcp';
import { WsStreamSettingsSchema } from './ws';
import { XHttpStreamSettingsSchema } from './xhttp';

export * from './external-proxy';
export * from './finalmask';
export * from './grpc';
export * from './httpupgrade';
export * from './hysteria';
export * from './kcp';
export * from './sockopt';
export * from './tcp';
export * from './ws';
export * from './xhttp';

export const NetworkSchema = z.enum([
  'tcp', 'kcp', 'ws', 'grpc', 'httpupgrade', 'xhttp', 'hysteria',
]);
export type Network = z.infer<typeof NetworkSchema>;

// Tagged-wrapper DU on `network`. The wire shape uses an asymmetric per-
// network key (`tcpSettings`, `wsSettings`, ...) rather than a single
// `settings` object — same pattern Xray ships and the panel's StreamSettings
// class flattens via toJson. Each branch carries only the matching key so
// fixtures round-trip byte-identical.
//
// `hysteria` is only valid when the parent protocol is hysteria — the
// network selector hides it for other protocols. xray-core enforces
// the constraint server-side too.
const TransportNetworkSettingsSchema = z.discriminatedUnion('network', [
  z.object({ network: z.literal('tcp'),         tcpSettings:         TcpStreamSettingsSchema }),
  z.object({ network: z.literal('kcp'),         kcpSettings:         KcpStreamSettingsSchema }),
  z.object({ network: z.literal('ws'),          wsSettings:          WsStreamSettingsSchema }),
  z.object({ network: z.literal('grpc'),        grpcSettings:        GrpcStreamSettingsSchema }),
  z.object({ network: z.literal('httpupgrade'), httpupgradeSettings: HttpUpgradeStreamSettingsSchema }),
  z.object({ network: z.literal('xhttp'),       xhttpSettings:       XHttpStreamSettingsSchema }),
  z.object({ network: z.literal('hysteria'),    hysteriaSettings:    HysteriaStreamSettingsSchema }),
]);

// Wireguard (always a UDP listener) and Tunnel (dokodemo-door) expose no
// user-selectable transport: their streamSettings carries no `network` key —
// only security/sockopt, and Tunnel relies on `sockopt.tproxy` for its TProxy
// mode. The transportless branch accepts that shape (network absent), while a
// present-but-invalid network still fails both branches so a typo can't slip
// through. `network: never().optional()` reads as "this key must be absent".
//
// The preprocess folds `method` — xray-core v26.7.11's preferred alias for
// `network`, which wins over `network` when both are present — back into the
// panel-canonical `network` key, so imported/pasted configs keyed on the
// alias don't silently match the transportless branch and lose their
// transport.
export const NetworkSettingsSchema = z.preprocess(
  (val) => {
    if (val && typeof val === 'object' && 'method' in val) {
      const { method, ...rest } = val as Record<string, unknown>;
      if (typeof method === 'string' && method !== '') {
        return { ...rest, network: method };
      }
      return rest;
    }
    return val;
  },
  z.union([
    TransportNetworkSettingsSchema,
    z.object({ network: z.never().optional() }),
  ]),
);
export type NetworkSettings = z.infer<typeof NetworkSettingsSchema>;

// Orthogonal extras that ride alongside the network and security branches.
// All optional on the wire — legacy toJson() omits any field whose value
// is empty. The shadow harness treats absent and empty-array as the same
// canonical state.
export const StreamExtrasSchema = z.object({
  externalProxy: z.array(ExternalProxyEntrySchema).optional(),
  finalmask: FinalMaskStreamSettingsSchema.optional(),
  sockopt: SockoptStreamSettingsSchema.optional(),
});
export type StreamExtras = z.infer<typeof StreamExtrasSchema>;
