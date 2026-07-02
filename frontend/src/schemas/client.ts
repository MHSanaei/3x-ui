import { z } from 'zod';

const nullableStringArray = z.array(z.string()).nullable().transform((v) => v ?? []);
const nullableNumberArray = z.array(z.number()).nullable().transform((v) => v ?? []);

export const ClientTrafficSchema = z.object({
  up: z.number().optional(),
  down: z.number().optional(),
  total: z.number().optional(),
  expiryTime: z.number().optional(),
  enable: z.boolean().optional(),
  lastOnline: z.number().optional(),
});

export const ClientRecordSchema = z.object({
  id: z.number().optional(),
  email: z.string(),
  subId: z.string().optional(),
  uuid: z.string().optional(),
  password: z.string().optional(),
  auth: z.string().optional(),
  flow: z.string().optional(),
  security: z.string().optional(),
  totalGB: z.number().optional(),
  expiryTime: z.number().optional(),
  limitIp: z.number().optional(),
  tgId: z.union([z.number(), z.string()]).optional(),
  group: z.string().optional(),
  comment: z.string().optional(),
  enable: z.boolean().optional(),
  reset: z.number().optional(),
  inboundIds: nullableNumberArray.optional(),
  traffic: ClientTrafficSchema.nullable().optional(),
  reverse: z.object({ tag: z.string().optional() }).loose().nullable().optional(),
  privateKey: z.string().optional(),
  publicKey: z.string().optional(),
  allowedIPs: z.string().optional(),
  preSharedKey: z.string().optional(),
  keepAlive: z.number().optional(),
  createdAt: z.number().optional(),
  updatedAt: z.number().optional(),
}).loose();

export const InboundOptionSchema = z.object({
  id: z.number(),
  remark: z.string().optional(),
  tag: z.string().optional(),
  protocol: z.string().optional(),
  port: z.number().optional(),
  tlsFlowCapable: z.boolean().optional(),
  ssMethod: z.string().optional(),
  wgPublicKey: z.string().optional(),
  wgMtu: z.number().optional(),
  wgDns: z.string().optional(),
  // Hosting node id; absent/null for this panel's own inbounds (#4997).
  nodeId: z.number().nullable().optional(),
  // Share-host resolution inputs, mirroring the backend resolveInboundAddress so
  // the clients page picks the same WireGuard endpoint host as the subscription:
  // the hosting node address, the inbound listen, and its share-address strategy.
  nodeAddress: z.string().optional(),
  listen: z.string().optional(),
  shareAddr: z.string().optional(),
  shareAddrStrategy: z.string().optional(),
}).loose();

export const InboundOptionsSchema = z.array(InboundOptionSchema);

export const ClientsSummarySchema = z.object({
  total: z.number(),
  active: z.number(),
  online: nullableStringArray,
  depleted: nullableStringArray,
  expiring: nullableStringArray,
  deactive: nullableStringArray,
});

const nullableClientArray = z.array(ClientRecordSchema).nullable().transform((v) => v ?? []);

export const ClientPageResponseSchema = z.object({
  items: nullableClientArray,
  total: z.number(),
  filtered: z.number(),
  page: z.number(),
  pageSize: z.number(),
  summary: ClientsSummarySchema.nullable().optional(),
  groups: nullableStringArray.optional(),
});

// A per-client external link surfaced in the client's subscription:
// kind=link is a single share link, kind=subscription is a remote sub URL.
export const ExternalLinkSchema = z.object({
  kind: z.enum(['link', 'subscription']).default('link'),
  value: z.string(),
  remark: z.string().optional().default(''),
}).loose();

export const ExternalLinkListSchema = z.array(ExternalLinkSchema).nullable().transform((v) => v ?? []);

export const ClientHydrateSchema = z.object({
  client: ClientRecordSchema,
  inboundIds: nullableNumberArray,
  externalLinks: ExternalLinkListSchema.optional(),
});

export const BulkAdjustResultSchema = z.object({
  adjusted: z.number(),
  skipped: z
    .array(z.object({ email: z.string(), reason: z.string() }))
    .optional(),
});

export const BulkDeleteResultSchema = z.object({
  deleted: z.number(),
  skipped: z
    .array(z.object({ email: z.string(), reason: z.string() }))
    .optional(),
});

export const BulkSetEnableResultSchema = z.object({
  changed: z.number(),
  skipped: z
    .array(z.object({ email: z.string(), reason: z.string() }))
    .optional(),
});

export const BulkCreateResultSchema = z.object({
  created: z.number(),
  skipped: z
    .array(z.object({ email: z.string(), reason: z.string() }))
    .optional(),
});

export const DelDepletedResultSchema = z.object({
  deleted: z.number().optional(),
});

export const BulkAttachResultSchema = z.object({
  attached: z.array(z.string()).nullable().transform((v) => v ?? []),
  skipped: z.array(z.string()).nullable().transform((v) => v ?? []),
  errors: z.array(z.string()).nullable().transform((v) => v ?? []),
});

export const BulkDetachResultSchema = z.object({
  detached: z.array(z.string()).nullable().transform((v) => v ?? []),
  skipped: z.array(z.string()).nullable().transform((v) => v ?? []),
  errors: z.array(z.string()).nullable().transform((v) => v ?? []),
});

export const OnlinesSchema = nullableStringArray;

export const OnlineByNodeSchema = z
  .record(z.string(), nullableStringArray)
  .nullable()
  .transform((v) => v ?? {});

export const ActiveInboundsByNodeSchema = z
  .record(z.string(), nullableStringArray)
  .nullable()
  .transform((v) => v ?? {});

export const GroupSummarySchema = z.object({
  name: z.string(),
  clientCount: z.number(),
  trafficUsed: z.number().nullable().transform((v) => v ?? 0),
  up: z.number().nullable().transform((v) => v ?? 0),
  down: z.number().nullable().transform((v) => v ?? 0),
});

export const GroupSummaryListSchema = z.array(GroupSummarySchema).nullable().transform((v) => v ?? []);

export function hasForbiddenClientChars(value: string): boolean {
  if (value.includes('/') || value.includes('\\') || value.includes(' ')) return true;
  for (let i = 0; i < value.length; i++) {
    const code = value.charCodeAt(i);
    if (code < 0x20 || code === 0x7f) return true;
  }
  return false;
}

export const ClientFormSchema = z.object({
  email: z
    .string()
    .trim()
    .min(1, 'pages.clients.email')
    .refine((v) => !hasForbiddenClientChars(v), 'pages.clients.emailInvalidChars'),
  subId: z.string().refine((v) => !hasForbiddenClientChars(v), 'pages.clients.subIdInvalidChars'),
  uuid: z.string(),
  password: z.string(),
  auth: z.string(),
  flow: z.string(),
  security: z.string(),
  reverseTag: z.string(),
  totalGB: z.number().min(0),
  delayedStart: z.boolean(),
  delayedDays: z.number().int().min(0),
  reset: z.number().int().min(0),
  limitIp: z.number().int().min(0),
  tgId: z.number().int().min(0),
  group: z.string(),
  comment: z.string(),
  enable: z.boolean(),
  inboundIds: z.array(z.number()),
});

export const ClientCreateFormSchema = ClientFormSchema.extend({
  inboundIds: z.array(z.number()).min(1, 'pages.clients.selectInbound'),
});

export const ClientBulkAdjustFormSchema = z
  .object({
    addDays: z.number().int(),
    addGB: z.number(),
    flow: z.string().optional().default(''),
  })
  .refine((v) => v.addDays !== 0 || v.addGB !== 0 || v.flow !== '', {
    message: 'pages.clients.bulkAdjustNothing',
  });

export const ClientBulkAddFormSchema = z.object({
  emailMethod: z.number().int().min(0).max(4),
  firstNum: z.number().int().min(1),
  lastNum: z.number().int().min(1),
  emailPrefix: z.string(),
  emailPostfix: z.string(),
  quantity: z.number().int().min(1).max(1000),
  subId: z.string(),
  group: z.string(),
  comment: z.string(),
  flow: z.string(),
  limitIp: z.number().int().min(0),
  totalGB: z.number().min(0),
  expiryTime: z.number(),
  reset: z.number().int().min(0),
  inboundIds: z.array(z.number()).min(1, 'pages.clients.selectInbound'),
});

export type ClientRecord = z.infer<typeof ClientRecordSchema>;
export type ClientTraffic = z.infer<typeof ClientTrafficSchema>;
export type InboundOption = z.infer<typeof InboundOptionSchema>;
export type ExternalLink = z.infer<typeof ExternalLinkSchema>;
export type ClientsSummary = z.infer<typeof ClientsSummarySchema>;
export type ClientPageResponse = z.infer<typeof ClientPageResponseSchema>;
export type ClientHydrate = z.infer<typeof ClientHydrateSchema>;
export type BulkAdjustResult = z.infer<typeof BulkAdjustResultSchema>;
export type BulkDeleteResult = z.infer<typeof BulkDeleteResultSchema>;
export type BulkSetEnableResult = z.infer<typeof BulkSetEnableResultSchema>;
export type BulkCreateResult = z.infer<typeof BulkCreateResultSchema>;
export type BulkAttachResult = z.infer<typeof BulkAttachResultSchema>;
export type BulkDetachResult = z.infer<typeof BulkDetachResultSchema>;
export type ClientBulkAddFormValues = z.infer<typeof ClientBulkAddFormSchema>;
export type ClientBulkAdjustFormValues = z.infer<typeof ClientBulkAdjustFormSchema>;
export type ClientFormValues = z.infer<typeof ClientFormSchema>;
export type GroupSummary = z.infer<typeof GroupSummarySchema>;
