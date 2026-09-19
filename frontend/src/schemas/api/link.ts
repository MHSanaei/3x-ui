import { z } from 'zod';

import {
  ExternalLinkSchema as GeneratedExternalLinkSchema,
  ExternalLinkTargetViewSchema,
  ExternalLinkViewSchema,
} from '@/generated/zod';

export const LinkKindSchema = z.enum(['link', 'subscription']);
export type LinkKind = z.infer<typeof LinkKindSchema>;

export const LinkScopeSchema = z.enum(['client', 'group', 'inbound', 'global', 'new_clients']);
export type LinkScope = z.infer<typeof LinkScopeSchema>;

// Headers arrive as null on every row that never set one, so the generated
// record schema would reject the whole list on the first such row.
export const LinkRecordSchema = GeneratedExternalLinkSchema.extend({
  kind: LinkKindSchema,
  headers: z
    .record(z.string(), z.string())
    .nullish()
    .transform((v) => v ?? {}),
});
export type LinkRecord = z.infer<typeof LinkRecordSchema>;

export const LinkListSchema = z
  .array(LinkRecordSchema)
  .nullish()
  .transform((v) => v ?? []);

export const LinkTargetSchema = ExternalLinkTargetViewSchema.extend({
  targetType: LinkScopeSchema,
});
export type LinkTarget = z.infer<typeof LinkTargetSchema>;

export const LinkTargetListSchema = z
  .array(LinkTargetSchema)
  .nullish()
  .transform((v) => v ?? []);

// One row of a client's Links tab that the client does not own itself: the
// scope names whoever granted it, which is what makes it read-only here.
export const LinkViewSchema = ExternalLinkViewSchema.extend({
  kind: LinkKindSchema,
  scope: LinkScopeSchema,
});
export type LinkView = z.infer<typeof LinkViewSchema>;

export const LinkViewListSchema = z
  .array(LinkViewSchema)
  .nullish()
  .transform((v) => v ?? []);

export const LinkSaveSchema = z.object({
  id: z.number().int().optional(),
  kind: LinkKindSchema.default('link'),
  value: z.string().trim().min(1, 'pages.links.valueRequired'),
  remark: z.string().trim().max(256).default(''),
  namePrefix: z.string().trim().max(64).default(''),
  enable: z.boolean().default(true),
  expiryTime: z.number().int().min(0).default(0),
  userAgent: z.string().trim().max(256).default(''),
  cacheTtl: z.number().int().min(0).default(0),
});
export type LinkSaveValues = z.infer<typeof LinkSaveSchema>;

export const LinkAssignSchema = z.object({
  scope: LinkScopeSchema.default('client'),
  emails: z.array(z.string()).default([]),
  group: z.string().default(''),
  inboundId: z.number().int().default(0),
});
export type LinkAssignValues = z.infer<typeof LinkAssignSchema>;

// The save endpoint writes user_agent and cache_ttl only when they carry a
// value, so a blank field must travel as an absent key: sending "" would look
// like a change the endpoint silently drops.
export function toLinkSaveRequest(values: LinkSaveValues): Record<string, unknown> {
  const payload: Record<string, unknown> = { ...values };
  if (!values.userAgent.trim()) delete payload.userAgent;
  if (values.cacheTtl <= 0) delete payload.cacheTtl;
  return payload;
}

// The assign endpoint takes one flag per scope rather than a scope name, so a
// payload that only names the scope has to be narrowed down here.
export function toAssignRequest(values: LinkAssignValues): Record<string, unknown> {
  return {
    emails: values.scope === 'client' ? values.emails : [],
    group: values.scope === 'group' ? values.group : '',
    inboundId: values.scope === 'inbound' ? values.inboundId : 0,
    global: values.scope === 'global',
    newClients: values.scope === 'new_clients',
  };
}
