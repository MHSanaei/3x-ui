import { z } from 'zod';

// mtg's [domain-fronting] section: where the sidecar forwards non-Telegram
// traffic (e.g. an NGINX fake site). All optional — omitted keys fall back to
// mtg's defaults (DNS-resolve the FakeTLS host, port 443, no proxy protocol).
export const MtprotoDomainFrontingSchema = z.object({
  ip: z.string().optional(),
  port: z.number().int().min(0).max(65535).optional(),
  proxyProtocol: z.boolean().optional(),
});
export type MtprotoDomainFronting = z.infer<typeof MtprotoDomainFrontingSchema>;

// MTProto (Telegram) inbound. Served by an mtg sidecar process, not Xray, so
// it has no clients and no stream settings. `secret` is the FakeTLS secret
// (ee-prefixed); the backend rebuilds it to match `fakeTlsDomain` on save.
// The remaining fields map to optional mtg config knobs and are written to the
// generated mtg.toml only when set.
export const MtprotoInboundSettingsSchema = z.object({
  fakeTlsDomain: z.string().default('www.cloudflare.com'),
  secret: z.string().default(''),
  proxyProtocolListener: z.boolean().optional(),
  preferIp: z.enum(['prefer-ipv6', 'prefer-ipv4', 'only-ipv6', 'only-ipv4']).optional(),
  debug: z.boolean().optional(),
  domainFronting: MtprotoDomainFrontingSchema.optional(),
  // When set, the mtg sidecar dials Telegram through a loopback SOCKS bridge in
  // the Xray config so the egress obeys routing rules. `outboundTag` optionally
  // forces that traffic out a specific outbound/balancer. `routeXrayPort` is the
  // bridge port; it is allocated and owned by the backend (never edited here).
  routeThroughXray: z.boolean().optional(),
  outboundTag: z.string().optional(),
  routeXrayPort: z.number().int().min(0).max(65535).optional(),
});
export type MtprotoInboundSettings = z.infer<typeof MtprotoInboundSettingsSchema>;
