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

// An MTProto (Telegram) inbound client (multi-client model). Each client is one
// named FakeTLS secret the mtg-multi sidecar serves through its [secrets]
// section; `secret` is the ee-prefixed FakeTLS secret whose trailing domain the
// backend rebuilds on save. `fakeTlsDomain` is stored on the inbound as the
// default domain used when generating a new client's secret.
export const MtprotoClientSchema = z.object({
  secret: z.string().default(''),
  adTag: z
    .string()
    .regex(/^[0-9a-fA-F]{32}$/, 'pages.inbounds.form.mtgAdTagInvalid')
    .or(z.literal(''))
    .optional(),
  email: z.string().min(1),
  limitIp: z.number().int().min(0).default(0),
  totalGB: z.number().int().min(0).default(0),
  expiryTime: z.number().int().default(0),
  enable: z.boolean().default(true),
  tgId: z
    .union([z.number(), z.string()])
    .transform((v) => Number(v) || 0)
    .default(0),
  subId: z.string().default(''),
  comment: z.string().default(''),
  reset: z.number().int().min(0).default(0),
  created_at: z.number().int().optional(),
  updated_at: z.number().int().optional(),
});
export type MtprotoClient = z.infer<typeof MtprotoClientSchema>;

// MTProto (Telegram) inbound. Served by an mtg-multi sidecar process, not Xray,
// so it has no stream settings. Each client carries its own FakeTLS secret and
// is served on the shared inbound port. The remaining fields map to optional mtg
// config knobs and are written to the generated mtg config only when set.
// A cleared input (null or '') is "unset": the key is left out of the config.
const unset = (v: unknown) => (v === null || v === '' ? undefined : v);

// telemt-only knobs: a per-client TCP connection cap and TOML merged under the
// generated telemt.toml (the backend refuses the keys the panel owns).
export const MtprotoTelemtSchema = z.object({
  maxTcpConnsPerUser: z.preprocess(unset, z.number().int().min(0).max(65535).optional()),
  extraToml: z.preprocess(unset, z.string().max(65536).optional()),
});
export type MtprotoTelemt = z.infer<typeof MtprotoTelemtSchema>;

export const MtprotoBackendSchema = z.enum(['mtg', 'telemt']);
export type MtprotoBackend = z.infer<typeof MtprotoBackendSchema>;

export const MtprotoInboundSettingsSchema = z.object({
  // Which sidecar serves the inbound; unset is mtg, so older inbounds keep it.
  backend: MtprotoBackendSchema.optional(),
  telemt: MtprotoTelemtSchema.optional(),
  fakeTlsDomain: z.string().default('www.cloudflare.com'),
  clients: z.array(MtprotoClientSchema).default([]),
  proxyProtocolListener: z.boolean().optional(),
  preferIp: z.enum(['prefer-ipv6', 'prefer-ipv4', 'only-ipv6', 'only-ipv4']).optional(),
  debug: z.boolean().optional(),
  domainFronting: MtprotoDomainFrontingSchema.optional(),
  // Caps concurrent connections across all users with a fair-share algorithm;
  // 0 or unset disables throttling.
  throttleMaxConnections: z.number().int().min(0).optional(),
  // mtg's [defense.pending-handshakes] max-per-ip; 0 or unset leaves it off.
  pendingHandshakesMaxPerIp: z.number().int().min(0).optional(),
  // When set, the mtg sidecar dials Telegram through a loopback SOCKS bridge in
  // the Xray config so the egress obeys routing rules. `outboundTag` optionally
  // forces that traffic out a specific outbound/balancer. `routeXrayPort` is the
  // bridge port; it is allocated and owned by the backend (never edited here).
  routeThroughXray: z.boolean().optional(),
  outboundTag: z.string().optional(),
  routeXrayPort: z.number().int().min(0).max(65535).optional(),
  // publicIpv4/publicIpv6 pin this server's reachable address the Telegram
  // middle proxy needs when clients carry ad-tags; blank = mtg auto-detects.
  publicIpv4: z.string().optional(),
  publicIpv6: z.string().optional(),
});
export type MtprotoInboundSettings = z.infer<typeof MtprotoInboundSettingsSchema>;
