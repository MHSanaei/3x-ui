import { z } from 'zod';

export const ManagedLinkKindSchema = z.enum(['link', 'subscription']);
export type ManagedLinkKind = z.infer<typeof ManagedLinkKindSchema>;

export const ManagedLinkRecordSchema = z.object({
  id: z.number(),
  kind: ManagedLinkKindSchema,
  value: z.string(),
  remark: z.string().optional().default(''),
  isDisabled: z.boolean().optional().default(false),
  sortIndex: z.number().optional().default(0),
  createdAt: z.number().optional(),
  updatedAt: z.number().optional(),
}).loose();
export type ManagedLinkRecord = z.infer<typeof ManagedLinkRecordSchema>;

export const ManagedLinkListSchema = z.array(ManagedLinkRecordSchema);

export const ManagedLinkFormSchema = z.object({
  kind: ManagedLinkKindSchema,
  value: z.string().trim().min(1),
  remark: z.string().max(256).default(''),
  isDisabled: z.boolean().default(false),
});
export type ManagedLinkFormValues = z.infer<typeof ManagedLinkFormSchema>;

export const LinkAssignResultSchema = z.object({
  clients: z.number().optional().default(0),
  links: z.number().optional().default(0),
  attached: z.number().optional().default(0),
  skipped: z.number().optional().default(0),
  missing: z.array(z.string()).optional().default([]),
});
export type LinkAssignResult = z.infer<typeof LinkAssignResultSchema>;
