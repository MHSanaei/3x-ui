import { z } from 'zod';

import { WsHeaderMapSchema } from '@/schemas/protocols/stream/ws';

// HTTP Upgrade transport reuses the flat WS-style header map (string values,
// not arrays — toV2Headers with arr=false). No heartbeat field — that's
// websocket-specific.
export const HttpUpgradeStreamSettingsSchema = z.object({
  acceptProxyProtocol: z.boolean().default(false),
  path: z.string().default('/'),
  host: z.string().default(''),
  headers: WsHeaderMapSchema.default({}),
});
export type HttpUpgradeStreamSettings = z.infer<typeof HttpUpgradeStreamSettingsSchema>;
