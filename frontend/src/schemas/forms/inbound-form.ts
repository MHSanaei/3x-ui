import { z } from 'zod';

import { InboundPortSchema, SniffingSchema } from '@/schemas/primitives';
import { InboundSettingsSchema } from '@/schemas/protocols/inbound';
import { SecuritySettingsSchema } from '@/schemas/protocols/security';
import { NetworkSettingsSchema, StreamExtrasSchema } from '@/schemas/protocols/stream';

// InboundFormValues = the values shape Form.useForm<T>() carries in
// InboundFormModal. Mirrors the wire shape (so submission can hand
// values straight to Schema.parse + POST) plus the DB-side fields that
// the panel's /panel/api/inbounds/add endpoint expects alongside.
//
// Differences from schemas/api/inbound.ts InboundSchema:
//   - settings/streamSettings/sniffing are nested OBJECTS here, not the
//     JSON strings the endpoint accepts. The form holds typed data; the
//     submit handler stringifies right before POSTing.
//   - Adds DB fields not in InboundSchema: up, down, total, trafficReset,
//     lastTrafficResetTime, nodeId. These flow through the DBInbound row,
//     not the xray-config slice.

export const InboundStreamFormSchema = NetworkSettingsSchema
  .and(SecuritySettingsSchema)
  .and(StreamExtrasSchema);
export type InboundStreamFormValues = z.infer<typeof InboundStreamFormSchema>;

export const TrafficResetSchema = z.enum(['never', 'hourly', 'daily', 'weekly', 'monthly']);
export type TrafficReset = z.infer<typeof TrafficResetSchema>;
export const ShareAddrStrategySchema = z.enum(['node', 'listen', 'custom']);
export type ShareAddrStrategy = z.infer<typeof ShareAddrStrategySchema>;

// Db-side fields layered on top of the xray slice. These mirror the
// DBInbound model — they live in the SQL row, not in xray's config.
export const InboundDbFieldsSchema = z.object({
  up: z.number().int().min(0).default(0),
  down: z.number().int().min(0).default(0),
  total: z.number().int().min(0).default(0),
  trafficReset: TrafficResetSchema.default('never'),
  lastTrafficResetTime: z.number().int().default(0),
  nodeId: z.number().int().nullable().optional(),
  shareAddrStrategy: ShareAddrStrategySchema.default('node'),
  shareAddr: z.string().default(''),
});
export type InboundDbFields = z.infer<typeof InboundDbFieldsSchema>;

// Base fields that apply to every inbound regardless of protocol or
// transport. The protocol-specific `settings` and the transport-specific
// `streamSettings` are layered on via intersection below.
export const InboundFormBaseSchema = z.object({
  remark: z.string().default(''),
  enable: z.boolean().default(true),
  port: InboundPortSchema,
  listen: z.string().default(''),
  tag: z.string().default(''),
  expiryTime: z.number().int().default(0),
  clientStats: z.string().optional(),
  sniffing: SniffingSchema.default({
    enabled: false,
    destOverride: ['http', 'tls', 'quic', 'fakedns'],
    metadataOnly: false,
    routeOnly: false,
    ipsExcluded: [],
    domainsExcluded: [],
  }),
  streamSettings: InboundStreamFormSchema.optional(),
});
export type InboundFormBase = z.infer<typeof InboundFormBaseSchema>;

// Full form values = base + db fields + protocol-discriminated settings.
// Consumers narrow on `.protocol` to access the matching settings branch.
export const InboundFormSchema = InboundFormBaseSchema
  .and(InboundDbFieldsSchema)
  .and(InboundSettingsSchema);
export type InboundFormValues = z.infer<typeof InboundFormSchema>;

// Fallback rows ride alongside the inbound submission for VLESS/Trojan
// hosts. They're saved via a separate endpoint after the main inbound
// POST returns, so the schema lives here but is not part of the wire
// inbound payload.
export const FallbackRowSchema = z.object({
  rowKey: z.string(),
  childId: z.number().int().nullable(),
  name: z.string().default(''),
  alpn: z.string().default(''),
  path: z.string().default(''),
  dest: z.string().default(''),
  xver: z.number().int().min(0).max(2).default(0),
});
export type FallbackRow = z.infer<typeof FallbackRowSchema>;
