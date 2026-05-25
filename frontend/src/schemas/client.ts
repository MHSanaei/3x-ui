import { z } from 'zod';

export const ClientTrafficSchema = z.object({
  up: z.number().optional(),
  down: z.number().optional(),
  total: z.number().optional(),
  expiryTime: z.number().optional(),
  enable: z.boolean().optional(),
  lastOnline: z.number().optional(),
});

export const ClientRecordSchema = z.object({
  email: z.string(),
  subId: z.string().optional(),
  uuid: z.string().optional(),
  password: z.string().optional(),
  auth: z.string().optional(),
  flow: z.string().optional(),
  totalGB: z.number().optional(),
  expiryTime: z.number().optional(),
  limitIp: z.number().optional(),
  tgId: z.union([z.number(), z.string()]).optional(),
  comment: z.string().optional(),
  enable: z.boolean().optional(),
  inboundIds: z.array(z.number()).optional(),
  traffic: ClientTrafficSchema.optional(),
  reverse: z.object({ tag: z.string().optional() }).loose().optional(),
  createdAt: z.number().optional(),
  updatedAt: z.number().optional(),
}).loose();

export const InboundOptionSchema = z.object({
  id: z.number(),
  remark: z.string().optional(),
  protocol: z.string().optional(),
  port: z.number().optional(),
  tlsFlowCapable: z.boolean().optional(),
}).loose();

export const InboundOptionsSchema = z.array(InboundOptionSchema);

export const ClientsSummarySchema = z.object({
  total: z.number(),
  active: z.number(),
  online: z.array(z.string()),
  depleted: z.array(z.string()),
  expiring: z.array(z.string()),
  deactive: z.array(z.string()),
});

export const ClientPageResponseSchema = z.object({
  items: z.array(ClientRecordSchema),
  total: z.number(),
  filtered: z.number(),
  page: z.number(),
  pageSize: z.number(),
  summary: ClientsSummarySchema.optional(),
});

export const ClientHydrateSchema = z.object({
  client: ClientRecordSchema,
  inboundIds: z.array(z.number()),
});

export const BulkAdjustResultSchema = z.object({
  adjusted: z.number(),
  skipped: z
    .array(z.object({ email: z.string(), reason: z.string() }))
    .optional(),
});

export const DelDepletedResultSchema = z.object({
  deleted: z.number().optional(),
});

export const OnlinesSchema = z.array(z.string());

export type ClientRecord = z.infer<typeof ClientRecordSchema>;
export type ClientTraffic = z.infer<typeof ClientTrafficSchema>;
export type InboundOption = z.infer<typeof InboundOptionSchema>;
export type ClientsSummary = z.infer<typeof ClientsSummarySchema>;
export type ClientPageResponse = z.infer<typeof ClientPageResponseSchema>;
export type ClientHydrate = z.infer<typeof ClientHydrateSchema>;
export type BulkAdjustResult = z.infer<typeof BulkAdjustResultSchema>;
