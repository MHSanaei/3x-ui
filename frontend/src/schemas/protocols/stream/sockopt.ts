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

// Sockopt knobs are an orthogonal layer on streamSettings — they tune
// the underlying socket (TCP keepalive, TFO, mark, tproxy, dialer proxy,
// IPv6-only, MPTCP). The wire field is `interface` (single word) but the
// panel class names it `interfaceName` internally to avoid the JS
// reserved keyword. We use `interfaceName` here too and document the
// renames; serializers writing back to wire must rename.
//
// trustedXForwardedFor is omitted from the wire payload when empty
// (legacy toJson() filters it); our default([]) lets parsing succeed but
// the shadow canonicalize step treats [] and absence as equivalent.
export const SockoptStreamSettingsSchema = z.object({
  acceptProxyProtocol: z.boolean().default(false),
  tcpFastOpen: z.boolean().default(false),
  mark: z.number().int().min(0).default(0),
  tproxy: TproxyModeSchema.default('off'),
  tcpMptcp: z.boolean().default(false),
  penetrate: z.boolean().default(false),
  domainStrategy: SockoptDomainStrategySchema.default('UseIP'),
  tcpMaxSeg: z.number().int().min(0).default(1440),
  dialerProxy: z.string().default(''),
  tcpKeepAliveInterval: z.number().int().min(0).default(0),
  tcpKeepAliveIdle: z.number().int().min(0).default(300),
  tcpUserTimeout: z.number().int().min(0).default(10000),
  tcpcongestion: TcpCongestionSchema.default('bbr'),
  V6Only: z.boolean().default(false),
  tcpWindowClamp: z.number().int().min(0).default(600),
  interfaceName: z.string().default(''),
  trustedXForwardedFor: z.array(z.string()).default([]),
});
export type SockoptStreamSettings = z.infer<typeof SockoptStreamSettingsSchema>;
