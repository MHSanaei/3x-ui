import { z } from 'zod';

export const FlowSchema = z.enum([
  '',
  'xtls-rprx-vision',
  'xtls-rprx-vision-udp443',
]);
export type Flow = z.infer<typeof FlowSchema>;

// Const map matching the legacy models/inbound.ts `TLS_FLOW_CONTROL`
// export. The empty-string default isn't keyed here — the legacy never
// carried a NONE key and call sites compare against the two real flows.
export const TLS_FLOW_CONTROL = Object.freeze({
  VISION: 'xtls-rprx-vision',
  VISION_UDP443: 'xtls-rprx-vision-udp443',
}) satisfies Record<string, Exclude<Flow, ''>>;
