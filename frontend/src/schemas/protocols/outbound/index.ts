import { z } from 'zod';

import { BlackholeOutboundSettingsSchema } from './blackhole';
import { DNSOutboundSettingsSchema } from './dns';
import { FreedomOutboundSettingsSchema } from './freedom';
import { HttpOutboundSettingsSchema } from './http';
import { HysteriaOutboundSettingsSchema } from './hysteria';
import { LoopbackOutboundSettingsSchema } from './loopback';
import { ShadowsocksOutboundSettingsSchema } from './shadowsocks';
import { SocksOutboundSettingsSchema } from './socks';
import { TrojanOutboundSettingsSchema } from './trojan';
import { VlessOutboundSettingsSchema } from './vless';
import { VmessOutboundSettingsSchema } from './vmess';
import { WireguardOutboundSettingsSchema } from './wireguard';

export * from './blackhole';
export * from './dns';
export * from './freedom';
export * from './http';
export * from './hysteria';
export * from './loopback';
export * from './shadowsocks';
export * from './socks';
export * from './trojan';
export * from './vless';
export * from './vmess';
export * from './wireguard';

// Outbound discriminated union spans 13 protocols (mixed/tunnel are
// inbound-only; freedom/blackhole/dns/loopback are outbound-only). The wire
// shape is `{ protocol, settings }` — same wrapper pattern as the inbound
// union, even though some leaf schemas (freedom, blackhole) are sparse.
export const OutboundSettingsSchema = z.discriminatedUnion('protocol', [
  z.object({ protocol: z.literal('vmess'),       settings: VmessOutboundSettingsSchema }),
  z.object({ protocol: z.literal('vless'),       settings: VlessOutboundSettingsSchema }),
  z.object({ protocol: z.literal('trojan'),      settings: TrojanOutboundSettingsSchema }),
  z.object({ protocol: z.literal('shadowsocks'), settings: ShadowsocksOutboundSettingsSchema }),
  z.object({ protocol: z.literal('wireguard'),   settings: WireguardOutboundSettingsSchema }),
  z.object({ protocol: z.literal('hysteria'),    settings: HysteriaOutboundSettingsSchema }),
  z.object({ protocol: z.literal('http'),        settings: HttpOutboundSettingsSchema }),
  z.object({ protocol: z.literal('socks'),       settings: SocksOutboundSettingsSchema }),
  z.object({ protocol: z.literal('freedom'),     settings: FreedomOutboundSettingsSchema }),
  z.object({ protocol: z.literal('blackhole'),   settings: BlackholeOutboundSettingsSchema }),
  z.object({ protocol: z.literal('dns'),         settings: DNSOutboundSettingsSchema }),
  z.object({ protocol: z.literal('loopback'),    settings: LoopbackOutboundSettingsSchema }),
]);
export type OutboundSettings = z.infer<typeof OutboundSettingsSchema>;
