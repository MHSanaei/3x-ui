import { z } from 'zod';

import { GrpcStreamSettingsSchema } from './grpc';
import { HttpUpgradeStreamSettingsSchema } from './httpupgrade';
import { KcpStreamSettingsSchema } from './kcp';
import { TcpStreamSettingsSchema } from './tcp';
import { WsStreamSettingsSchema } from './ws';
import { XHttpStreamSettingsSchema } from './xhttp';

export * from './grpc';
export * from './httpupgrade';
export * from './kcp';
export * from './tcp';
export * from './ws';
export * from './xhttp';

export const NetworkSchema = z.enum(['tcp', 'kcp', 'ws', 'grpc', 'httpupgrade', 'xhttp']);
export type Network = z.infer<typeof NetworkSchema>;

// Tagged-wrapper DU on `network`. The wire shape uses an asymmetric per-
// network key (`tcpSettings`, `wsSettings`, ...) rather than a single
// `settings` object — same pattern Xray ships and the panel's StreamSettings
// class flattens via toJson. Each branch carries only the matching key so
// fixtures round-trip byte-identical.
export const NetworkSettingsSchema = z.discriminatedUnion('network', [
  z.object({ network: z.literal('tcp'),         tcpSettings:         TcpStreamSettingsSchema }),
  z.object({ network: z.literal('kcp'),         kcpSettings:         KcpStreamSettingsSchema }),
  z.object({ network: z.literal('ws'),          wsSettings:          WsStreamSettingsSchema }),
  z.object({ network: z.literal('grpc'),        grpcSettings:        GrpcStreamSettingsSchema }),
  z.object({ network: z.literal('httpupgrade'), httpupgradeSettings: HttpUpgradeStreamSettingsSchema }),
  z.object({ network: z.literal('xhttp'),       xhttpSettings:       XHttpStreamSettingsSchema }),
]);
export type NetworkSettings = z.infer<typeof NetworkSettingsSchema>;
