import { z } from 'zod';

import { PortSchema } from '@/schemas/primitives';

export const DNSRuleActionSchema = z.enum(['direct', 'reject', 'rejectIPv4', 'rejectIPv6']);

// On the wire `qtype` is either a number (DNS type code) or a string like
// "A"/"AAAA"/"TXT"; the panel normalizes numeric strings to numbers in
// toJson. `domain` is a string[] (split from a comma-joined input).
export const DNSRuleSchema = z.object({
  action: DNSRuleActionSchema.default('direct'),
  qtype: z.union([z.string(), z.number().int()]).optional(),
  domain: z.array(z.string()).optional(),
});
export type DNSRule = z.infer<typeof DNSRuleSchema>;

// DNS outbound rewrites DNS queries onto a different transport. All five
// fields are emitted conditionally — empty/zero values are omitted from the
// wire payload entirely (handled at the caller, not here).
export const DNSOutboundSettingsSchema = z.object({
  rewriteNetwork: z.string().optional(),
  rewriteAddress: z.string().optional(),
  rewritePort: PortSchema.optional(),
  userLevel: z.number().int().min(0).optional(),
  rules: z.array(DNSRuleSchema).optional(),
});
export type DNSOutboundSettings = z.infer<typeof DNSOutboundSettingsSchema>;
