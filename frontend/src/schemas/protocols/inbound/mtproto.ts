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

// Optional mtg knobs mirror mtg-multi's limits (the backend re-checks them). A cleared
// input (null or '') is "unset": the key is left out and mtg uses its default.
const unset = (v: unknown) => (v === null || v === '' ? undefined : v);
const optInt = (min: number, max: number) =>
  z.preprocess(unset, z.number().int().min(min).max(max).optional());
const optText = (re: RegExp, message: string) =>
  z.preprocess(unset, z.string().trim().regex(re, message).optional());
const optList = (re: RegExp, message: string) =>
  z.preprocess(
    (v) => (Array.isArray(v) && v.length === 0 ? undefined : unset(v)),
    z.array(z.string().trim().regex(re, message)).max(32).optional(),
  );

// A Go duration (mtg's TypeDuration), e.g. 5s, 1m30s, 24h.
const DURATION = /^(\d+(\.\d+)?(ns|us|µs|ms|s|m|h))+$/i;
const DURATION_MSG = 'pages.inbounds.form.mtgInvalidDuration';
// A byte size (mtg's TypeBytes), e.g. 128kib, 1mib.
const BYTES = /^\d+\s*(b|kb|kib|mb|mib|gb|gib|tb|tib)$/i;
const BYTES_MSG = 'pages.inbounds.form.mtgInvalidSize';
const optDuration = () => optText(DURATION, DURATION_MSG);
// mtg only listens on literal IPs; metrics and WEB must stay on loopback.
const LOOPBACK_HOSTPORT = /^(127(\.\d{1,3}){3}|\[::1\]):\d{1,5}$/;
const LOOPBACK_MSG = 'pages.inbounds.form.mtgLoopbackOnly';
const MAX_UINT16 = 65535;

// [dc-pool]: warm connections per Telegram DC. `dcs` holds signed DC ids (negative =
// media, 203 = CDN) and needs an mtg-multi build with a configurable DC set.
export const MtprotoDcPoolSchema = z.object({
  enabled: z.boolean().optional(),
  size: z.number().int().min(1).max(64).optional(),
  dcs: z.preprocess(
    (v) => (Array.isArray(v) && v.length === 0 ? undefined : unset(v)),
    z
      .array(
        z.coerce
          .number()
          .int()
          .min(-32768)
          .max(32767)
          .refine((n) => n !== 0),
      )
      .max(32)
      .refine((a) => new Set(a).size === a.length, 'pages.inbounds.form.mtgDcPoolDcsDuplicate')
      .optional(),
  ),
});
export type MtprotoDcPool = z.infer<typeof MtprotoDcPoolSchema>;

const IpListSchema = z.object({
  enabled: z.boolean().optional(),
  urls: optList(/^https?:\/\/\S+$/i, 'pages.inbounds.form.mtgInvalidUrl'),
  updateEach: optDuration(),
  downloadConcurrency: optInt(1, MAX_UINT16),
});

// [network], [network.timeout] and [network.keep-alive]. `proxies` cannot be
// combined with routing through Xray, which owns mtg's upstream list.
export const MtprotoNetworkSchema = z.object({
  dns: z.preprocess(unset, z.string().trim().optional()),
  proxies: optList(/^socks5h?:\/\/\S+$/i, 'pages.inbounds.form.mtgInvalidProxy'),
  tcpNotSentLowat: optText(BYTES, BYTES_MSG),
  timeout: z
    .object({
      tcp: optDuration(),
      http: optDuration(),
      idle: optDuration(),
      handshake: optDuration(),
    })
    .optional(),
  keepAlive: z
    .object({
      disabled: z.boolean().optional(),
      idle: optDuration(),
      interval: optDuration(),
      count: optInt(1, MAX_UINT16),
    })
    .optional(),
});

// [defense.*]: anti-replay, IP block/allow lists, doppelganger and the per-IP
// pending-handshake cap (the last needs mtg-multi with pending-handshakes support).
export const MtprotoDefenseSchema = z.object({
  antiReplay: z
    .object({
      enabled: z.boolean().optional(),
      maxSize: optText(BYTES, BYTES_MSG),
      errorRate: z.preprocess(unset, z.number().gt(0).lt(100).optional()),
    })
    .optional(),
  blocklist: IpListSchema.optional(),
  allowlist: IpListSchema.optional(),
  doppelganger: z
    .object({
      urls: optList(/^https:\/\/\S+$/i, 'pages.inbounds.form.mtgInvalidHttpsUrl'),
      repeatsPerRaid: optInt(1, MAX_UINT16),
      raidEach: optDuration(),
      drs: z.boolean().optional(),
    })
    .optional(),
  pendingHandshakes: z
    .object({
      maxPerIp: optInt(0, MAX_UINT16),
      dryRun: z.boolean().optional(),
    })
    .optional(),
});

// [stats.prometheus] (loopback only: the endpoint has no authentication) and
// [stats.statsd].
export const MtprotoStatsSchema = z.object({
  prometheus: z
    .object({
      enabled: z.boolean().optional(),
      bindTo: optText(LOOPBACK_HOSTPORT, LOOPBACK_MSG),
      httpPath: optText(/^\/[^\s?#]*$/, 'pages.inbounds.form.mtgInvalidPath'),
      metricPrefix: optText(/^[a-z0-9]+$/, 'pages.inbounds.form.mtgInvalidMetricPrefix'),
    })
    .optional(),
  statsd: z
    .object({
      enabled: z.boolean().optional(),
      address: z.preprocess(unset, z.string().trim().optional()),
      metricPrefix: optText(/^[a-z0-9]+$/, 'pages.inbounds.form.mtgInvalidMetricPrefix'),
      tagFormat: z.preprocess(unset, z.enum(['datadog', 'influxdb', 'graphite']).optional()),
    })
    .optional(),
});

// [web]: MTProto inside real HTTPS (tg://webproxy). mtg serves plain HTTP on loopback
// behind a TLS reverse proxy; needs an mtg-multi build with WEB mode.
export const MtprotoWebSchema = z.object({
  bindTo: optText(LOOPBACK_HOSTPORT, LOOPBACK_MSG),
  host: optText(
    /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$/i,
    'pages.inbounds.form.mtgInvalidHost',
  ),
  secretMode: z.preprocess(unset, z.enum(['dd', 'plain']).optional()),
  decoyDir: optText(/^\/\S*$/, 'pages.inbounds.form.mtgInvalidPath'),
  trustedProxies: optList(/^[0-9a-f:.]+\/\d{1,3}$/i, 'pages.inbounds.form.mtgInvalidCidr'),
  maxSessions: optInt(1, MAX_UINT16),
  maxPending: optInt(1, MAX_UINT16),
  diag: z.boolean().optional(),
});
export type MtprotoWeb = z.infer<typeof MtprotoWebSchema>;

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
export const MtprotoInboundSettingsSchema = z.object({
  fakeTlsDomain: z.string().default('www.cloudflare.com'),
  clients: z.array(MtprotoClientSchema).default([]),
  proxyProtocolListener: z.boolean().optional(),
  preferIp: z.enum(['prefer-ipv6', 'prefer-ipv4', 'only-ipv6', 'only-ipv4']).optional(),
  debug: z.boolean().optional(),
  domainFronting: MtprotoDomainFrontingSchema.optional(),
  // Caps concurrent connections across all users with a fair-share algorithm;
  // 0 or unset disables throttling.
  throttleMaxConnections: z.number().int().min(0).optional(),
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
  // Both need an mtg-multi build that knows [secured] / [dc-pool]; older ones
  // ignore the sections. An unset pool size leaves the per-DC count to mtg.
  secured: z.boolean().optional(),
  dcPool: MtprotoDcPoolSchema.optional(),
  // The remaining mtg-multi options; each is written only when set.
  concurrency: optInt(1, MAX_UINT16),
  tolerateTimeSkewness: optDuration(),
  allowFallbackOnUnknownDc: z.boolean().optional(),
  autoUpdate: z.boolean().optional(),
  throttleCheckInterval: optDuration(),
  network: MtprotoNetworkSchema.optional(),
  defense: MtprotoDefenseSchema.optional(),
  stats: MtprotoStatsSchema.optional(),
  web: MtprotoWebSchema.optional(),
  // Free-form TOML merged under the generated config; panel keys win, and the
  // backend rejects the client sections, bind-to and the API keys.
  extraToml: z.preprocess(unset, z.string().max(65536).optional()),
});
export type MtprotoInboundSettings = z.infer<typeof MtprotoInboundSettingsSchema>;
