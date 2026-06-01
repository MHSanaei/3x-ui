import { z } from 'zod';

import { PortSchema } from '@/schemas/primitives';
import { VmessSecuritySchema } from '@/schemas/protocols/shared/vmess';

// Vmess outbound persists in the standard Xray `vnext` shape:
// { vnext: [{ address, port, users: [{ id, security }] }] }
// — distinct from VLESS outbound which the panel stores flat.
export const VmessOutboundUserSchema = z.object({
  id: z.uuid(),
  security: VmessSecuritySchema.default('auto'),
});
export type VmessOutboundUser = z.infer<typeof VmessOutboundUserSchema>;

export const VmessOutboundServerSchema = z.object({
  address: z.string().min(1),
  port: PortSchema,
  users: z.array(VmessOutboundUserSchema).min(1),
});
export type VmessOutboundServer = z.infer<typeof VmessOutboundServerSchema>;

export const VmessOutboundSettingsSchema = z.object({
  vnext: z.array(VmessOutboundServerSchema).min(1),
});
export type VmessOutboundSettings = z.infer<typeof VmessOutboundSettingsSchema>;
