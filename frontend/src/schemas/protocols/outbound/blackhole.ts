import { z } from 'zod';

export const BlackholeResponseTypeSchema = z.enum(['none', 'http']);
export type BlackholeResponseType = z.infer<typeof BlackholeResponseTypeSchema>;

// Blackhole drops traffic. `response.type` is the only knob — when set, Xray
// returns the canned 403 HTTP response before closing; when omitted it
// silently drops. The panel stores it as { response: { type } } or omits the
// whole `response` key when type is empty.
export const BlackholeOutboundSettingsSchema = z.object({
  response: z.object({ type: BlackholeResponseTypeSchema }).optional(),
});
export type BlackholeOutboundSettings = z.infer<typeof BlackholeOutboundSettingsSchema>;
