import { z } from 'zod';

// WebSocket stream uses the flat V1-style header map (string values only,
// not arrays — the panel calls toV2Headers with arr=false). `path` and
// `host` are the WS request line / Host header overrides. `heartbeatPeriod`
// in seconds; 0 disables heartbeats.
export const WsHeaderMapSchema = z.record(z.string(), z.string());
export type WsHeaderMap = z.infer<typeof WsHeaderMapSchema>;

export const WsStreamSettingsSchema = z.object({
  acceptProxyProtocol: z.boolean().default(false),
  path: z.string().default('/'),
  host: z.string().default(''),
  headers: WsHeaderMapSchema.default({}),
  heartbeatPeriod: z.number().int().min(0).default(0),
});
export type WsStreamSettings = z.infer<typeof WsStreamSettingsSchema>;
