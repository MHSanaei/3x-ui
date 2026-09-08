import { z } from 'zod';

export const BlackholeResponseTypeSchema = z.enum(['none', 'http', 'custom']);
export type BlackholeResponseType = z.infer<typeof BlackholeResponseTypeSchema>;

// `response.type` picks Xray's reply before closing: none (silent), http
// (canned 403) or custom (base64 customResponseData). Omitted when empty.
export const BlackholeOutboundSettingsSchema = z.object({
  response: z
    .object({ type: BlackholeResponseTypeSchema, customResponseData: z.string().optional() })
    .optional(),
});
export type BlackholeOutboundSettings = z.infer<typeof BlackholeOutboundSettingsSchema>;
