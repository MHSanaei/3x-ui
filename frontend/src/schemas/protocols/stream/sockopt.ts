import { z } from 'zod';

export const SockoptDomainStrategySchema = z.enum([
  'AsIs',
  'UseIP',
  'UseIPv6v4',
  'UseIPv6',
  'UseIPv4v6',
  'UseIPv4',
  'ForceIP',
  'ForceIPv6v4',
  'ForceIPv6',
  'ForceIPv4v6',
  'ForceIPv4',
]);
export type SockoptDomainStrategy = z.infer<typeof SockoptDomainStrategySchema>;

export const TcpCongestionSchema = z.enum(['bbr', 'cubic', 'reno']);
export type TcpCongestion = z.infer<typeof TcpCongestionSchema>;

export const TproxyModeSchema = z.enum(['off', 'redirect', 'tproxy']);
export type TproxyMode = z.infer<typeof TproxyModeSchema>;

export const AddressPortStrategySchema = z.enum([
  'none',
  'SrvPortOnly',
  'SrvAddressOnly',
  'SrvPortAndAddress',
  'TxtPortOnly',
  'TxtAddressOnly',
  'TxtPortAndAddress',
]);
export type AddressPortStrategy = z.infer<typeof AddressPortStrategySchema>;

export const HappyEyeballsSchema = z.object({
  tryDelayMs: z.number().int().min(0).default(0),
  prioritizeIPv6: z.boolean().default(false),
  interleave: z.number().int().min(1).default(1),
  maxConcurrentTry: z.number().int().min(0).default(4),
});
export type HappyEyeballs = z.infer<typeof HappyEyeballsSchema>;

export const CustomSockoptSchema = z.object({
  system: z.enum(['linux', 'windows', 'darwin']).optional(),
  type: z.enum(['int', 'str']),
  level: z.string().default('6'),
  opt: z.string(),
  value: z.union([z.string(), z.number()]),
});
export type CustomSockopt = z.infer<typeof CustomSockoptSchema>;

export const SockoptStreamSettingsSchema = z.object({
  acceptProxyProtocol: z.boolean().default(false),
  tcpFastOpen: z.union([z.boolean(), z.number().int()]).default(false),
  mark: z.number().int().default(0),
  tproxy: TproxyModeSchema.default('off'),
  tcpMptcp: z.boolean().default(false),
  penetrate: z.boolean().default(false),
  domainStrategy: SockoptDomainStrategySchema.default('AsIs'),
  tcpMaxSeg: z.number().int().min(0).default(1440),
  dialerProxy: z.string().default(''),
  tcpKeepAliveInterval: z.number().int().min(0).default(45),
  tcpKeepAliveIdle: z.number().int().min(0).default(45),
  tcpUserTimeout: z.number().int().min(0).default(10000),
  tcpcongestion: TcpCongestionSchema.default('bbr'),
  V6Only: z.boolean().default(false),
  tcpWindowClamp: z.number().int().min(0).default(600),
  interfaceName: z.string().default(''),
  trustedXForwardedFor: z.array(z.string()).default([]),
  addressPortStrategy: AddressPortStrategySchema.default('none'),
  happyEyeballs: HappyEyeballsSchema.optional(),
  customSockopt: z.array(CustomSockoptSchema).default([]),
});
export type SockoptStreamSettings = z.infer<typeof SockoptStreamSettingsSchema>;
