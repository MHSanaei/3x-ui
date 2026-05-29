import { z } from 'zod';

// HTTP proxy inbound — a classic forward proxy. Accounts are user/pass pairs;
// `allowTransparent` exposes Xray's option to forward requests with the
// original Host header. No client tracking (no email/limits) at the Xray
// settings level — the panel doesn't model HTTP users as billable clients.
export const HttpAccountSchema = z.object({
  user: z.string().min(1),
  pass: z.string().min(1),
});
export type HttpAccount = z.infer<typeof HttpAccountSchema>;

export const HttpInboundSettingsSchema = z.object({
  accounts: z.array(HttpAccountSchema).default([]),
  allowTransparent: z.boolean().default(false),
});
export type HttpInboundSettings = z.infer<typeof HttpInboundSettingsSchema>;
