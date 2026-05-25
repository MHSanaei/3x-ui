import { z } from 'zod';

import { HysteriaClientSchema } from '@/schemas/protocols/inbound/hysteria';

// hysteria2 is wire-distinct from hysteria (different parent protocol literal,
// different Go validate tag) but the panel's settings payload is structurally
// identical — same client shape, same auth-based clients. We pin `version` to
// the literal 2 here so a hysteria2 inbound can never silently downgrade.
export const Hysteria2InboundSettingsSchema = z.object({
  version: z.literal(2).default(2),
  clients: z.array(HysteriaClientSchema).default([]),
});
export type Hysteria2InboundSettings = z.infer<typeof Hysteria2InboundSettingsSchema>;
