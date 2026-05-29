import { z } from 'zod';

// mKCP transport (Xray's reliable UDP). The panel renames upCap/downCap on
// the JS side back to uplinkCapacity/downlinkCapacity on the wire. Defaults
// match xray-core's recommended values.
export const KcpStreamSettingsSchema = z.object({
  mtu: z.number().int().min(576).max(1460).default(1350),
  tti: z.number().int().min(10).max(100).default(20),
  uplinkCapacity: z.number().int().min(0).default(5),
  downlinkCapacity: z.number().int().min(0).default(20),
  cwndMultiplier: z.number().int().min(1).default(1),
  maxSendingWindow: z.number().int().min(0).default(2097152),
});
export type KcpStreamSettings = z.infer<typeof KcpStreamSettingsSchema>;
