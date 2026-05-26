import { z } from 'zod';

import { HttpInboundSettingsSchema } from './http';
import { HysteriaInboundSettingsSchema } from './hysteria';
import { MixedInboundSettingsSchema } from './mixed';
import { ShadowsocksInboundSettingsSchema } from './shadowsocks';
import { TrojanInboundSettingsSchema } from './trojan';
import { TunInboundSettingsSchema } from './tun';
import { TunnelInboundSettingsSchema } from './tunnel';
import { VlessInboundSettingsSchema } from './vless';
import { VmessInboundSettingsSchema } from './vmess';
import { WireguardInboundSettingsSchema } from './wireguard';

export * from './http';
export * from './hysteria';
export * from './mixed';
export * from './shadowsocks';
export * from './trojan';
export * from './tun';
export * from './tunnel';
export * from './vless';
export * from './vmess';
export * from './wireguard';

// Tagged-wrapper discriminated union. The discriminator (`protocol`) lives on
// the wrapper, not inside `settings`, mirroring the wire format Xray emits:
//   { protocol: 'vless', settings: { clients: [...], ... }, ... }
// Consumers narrow on `.protocol` and TypeScript narrows `.settings` to the
// matching leaf type.
export const InboundSettingsSchema = z.discriminatedUnion('protocol', [
  z.object({ protocol: z.literal('vmess'),       settings: VmessInboundSettingsSchema }),
  z.object({ protocol: z.literal('vless'),       settings: VlessInboundSettingsSchema }),
  z.object({ protocol: z.literal('trojan'),      settings: TrojanInboundSettingsSchema }),
  z.object({ protocol: z.literal('shadowsocks'), settings: ShadowsocksInboundSettingsSchema }),
  z.object({ protocol: z.literal('wireguard'),   settings: WireguardInboundSettingsSchema }),
  z.object({ protocol: z.literal('hysteria'),    settings: HysteriaInboundSettingsSchema }),
  z.object({ protocol: z.literal('http'),        settings: HttpInboundSettingsSchema }),
  z.object({ protocol: z.literal('mixed'),       settings: MixedInboundSettingsSchema }),
  z.object({ protocol: z.literal('tunnel'),      settings: TunnelInboundSettingsSchema }),
  z.object({ protocol: z.literal('tun'),         settings: TunInboundSettingsSchema }),
]);
export type InboundSettings = z.infer<typeof InboundSettingsSchema>;
