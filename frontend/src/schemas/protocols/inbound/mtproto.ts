import { z } from 'zod';

// MTProto (Telegram) inbound. Served by an mtg sidecar process, not Xray, so
// it has no clients and no stream settings. `secret` is the FakeTLS secret
// (ee-prefixed); the backend rebuilds it to match `fakeTlsDomain` on save.
export const MtprotoInboundSettingsSchema = z.object({
  fakeTlsDomain: z.string().default('www.cloudflare.com'),
  secret: z.string().default(''),
});
export type MtprotoInboundSettings = z.infer<typeof MtprotoInboundSettingsSchema>;
