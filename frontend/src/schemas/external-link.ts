import { z } from 'zod';

import { parseOutboundLink } from '@/lib/xray/outbound-link-parser';

export const ExternalShareLinkValueSchema = z
  .string()
  .trim()
  .refine(
    (value) => value === '' || parseOutboundLink(value) !== null,
    'pages.clients.externalLinkInvalid',
  );
export const ExternalSubscriptionValueSchema = z
  .string()
  .trim()
  .refine((value) => {
    if (value === '') return true;
    try {
      const url = new URL(value);
      return (url.protocol === 'http:' || url.protocol === 'https:') && url.hostname !== '';
    } catch {
      return false;
    }
  }, 'pages.clients.externalSubscriptionInvalid');

export const ExternalLinkInputSchema = z
  .object({
    kind: z.enum(['link', 'subscription']),
    value: z.string().trim(),
    remark: z.string().trim(),
    enable: z.boolean(),
    expiryTime: z.number().int().min(0),
    namePrefix: z.string(),
  })
  .superRefine((row, ctx) => {
    const schema =
      row.kind === 'subscription' ? ExternalSubscriptionValueSchema : ExternalShareLinkValueSchema;
    const result = schema.safeParse(row.value);
    if (!result.success)
      ctx.addIssue({ code: 'custom', path: ['value'], message: result.error.issues[0].message });
  });

export const ExternalLinkFormSchema = ExternalLinkInputSchema.safeExtend({
  lastFetchAt: z.number(),
  lastFetchError: z.string(),
});
export const ExternalLinksFormSchema = z.object({ externalLinks: z.array(ExternalLinkFormSchema) });
export const ExternalLinksAppendSchema = z.object({
  externalLinks: z
    .array(ExternalLinkInputSchema)
    .refine(
      (rows) => rows.some((row) => row.value !== ''),
      'pages.clients.bulkAppendLinksRequired',
    ),
});

export type ExternalLinkInput = z.infer<typeof ExternalLinkInputSchema>;
export type ExternalLinkRow = z.infer<typeof ExternalLinkFormSchema>;
export type ExternalLinksFormValues = z.infer<typeof ExternalLinksFormSchema>;
