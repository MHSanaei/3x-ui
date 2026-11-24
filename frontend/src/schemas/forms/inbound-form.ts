import { z } from 'zod';

import { InboundPortSchema, SniffingSchema } from '@/schemas/primitives';
import { InboundSettingsSchema } from '@/schemas/protocols/inbound';
import {
  TlsCertInlineSchema,
  TlsStreamSettingsSchema,
  securitySettingsSchemaFor,
  tlsCertUsesFiles,
} from '@/schemas/protocols/security';
import { NetworkSettingsSchema, StreamExtrasSchema } from '@/schemas/protocols/stream';

// Inbound certificates must follow the selected editor mode. The shared wire
// union also serves outbound TLS, where a client certificate is optional.
const InboundTlsCertFieldsSchema = TlsCertInlineSchema.extend({
  useFile: z.boolean().optional(),
  certificateFile: z.string().default(''),
  keyFile: z.string().default(''),
  certificate: z.array(z.string()).default([]),
  key: z.array(z.string()).default([]),
});

const InboundTlsCertSchema = InboundTlsCertFieldsSchema.superRefine((cert, ctx) => {
  const useFile = tlsCertUsesFiles(cert);
  const hasCertificate = useFile
    ? cert.certificateFile.trim() !== ''
    : cert.certificate.some((line) => line.trim() !== '');
  const hasKey = useFile ? cert.keyFile.trim() !== '' : cert.key.some((line) => line.trim() !== '');
  if (!hasCertificate) {
    ctx.addIssue({
      code: 'custom',
      path: [useFile ? 'certificateFile' : 'certificate'],
      message: 'pages.inbounds.form.tlsCertificateRequired',
    });
  }
  if (cert.usage !== 'verify' && !hasKey) {
    ctx.addIssue({
      code: 'custom',
      path: [useFile ? 'keyFile' : 'key'],
      message: 'pages.inbounds.form.tlsPrivateKeyRequired',
    });
  }
}).transform((cert) => {
  const { useFile: _useFile, certificateFile, keyFile, certificate, key, ...settings } = cert;
  return tlsCertUsesFiles(cert)
    ? { ...settings, certificateFile, keyFile }
    : { ...settings, certificate, key };
});

const InboundTlsSettingsSchema = TlsStreamSettingsSchema.extend({
  certificates: z
    .array(InboundTlsCertSchema)
    .default([])
    .refine((certificates) => certificates.some((cert) => cert.usage !== 'verify'), {
      error: 'pages.inbounds.form.tlsServerCertificateRequired',
    }),
});

const InboundSecuritySettingsSchema = securitySettingsSchemaFor(InboundTlsSettingsSchema);

export const InboundStreamFormSchema = NetworkSettingsSchema.and(InboundSecuritySettingsSchema).and(
  StreamExtrasSchema,
);
export type InboundStreamFormValues = z.infer<typeof InboundStreamFormSchema>;

export const TrafficResetSchema = z.enum(['never', 'hourly', 'daily', 'weekly', 'monthly']);
export type TrafficReset = z.infer<typeof TrafficResetSchema>;
export const ShareAddrStrategySchema = z.enum(['node', 'listen', 'custom']);
export type ShareAddrStrategy = z.infer<typeof ShareAddrStrategySchema>;

// Db-side fields layered on top of the xray slice. These mirror the
// DBInbound model — they live in the SQL row, not in xray's config.
export const InboundDbFieldsSchema = z.object({
  up: z.number().int().min(0).default(0),
  down: z.number().int().min(0).default(0),
  total: z.number().int().min(0).default(0),
  trafficReset: TrafficResetSchema.default('never'),
  trafficResetDay: z.number().int().min(1).max(31).default(1),
  preserveClientTraffic: z.boolean().default(false),
  lastTrafficResetTime: z.number().int().default(0),
  nodeId: z.number().int().nullable().optional(),
  shareAddrStrategy: ShareAddrStrategySchema.default('node'),
  shareAddr: z.string().default(''),
  subSortIndex: z.number().int().default(1),
  excludeFromSub: z.boolean().default(false),
  disableFlow: z.boolean().default(false),
});
export type InboundDbFields = z.infer<typeof InboundDbFieldsSchema>;

export const InboundFormBaseSchema = z.object({
  remark: z.string().default(''),
  enable: z.boolean().default(true),
  port: InboundPortSchema,
  listen: z.string().default(''),
  tag: z.string().default(''),
  expiryTime: z.number().int().default(0),
  clientStats: z.string().optional(),
  sniffing: SniffingSchema.default({
    enabled: false,
    destOverride: ['http', 'tls', 'quic', 'fakedns'],
    metadataOnly: false,
    routeOnly: false,
    ipsExcluded: [],
    domainsExcluded: [],
  }),
  streamSettings: InboundStreamFormSchema.optional(),
});
export type InboundFormBase = z.infer<typeof InboundFormBaseSchema>;

// Full form values = base + db fields + protocol-discriminated settings.
// Consumers narrow on `.protocol` to access the matching settings branch.
export const InboundFormSchema = InboundFormBaseSchema.and(InboundDbFieldsSchema)
  .and(InboundSettingsSchema)
  .superRefine((form, ctx) => {
    const stream = form.streamSettings;
    if (stream?.network !== 'xhttp') return;
    const xhttp = stream.xhttpSettings;
    const down = xhttp?.downloadSettings;
    if (!down) return;
    const issue = (field: string[], message: string) =>
      ctx.addIssue({
        code: 'custom',
        path: ['streamSettings', 'xhttpSettings', 'downloadSettings', ...field],
        message,
      });
    if (xhttp.mode === 'stream-one')
      issue([], 'Separate download is incompatible with stream-one.');
    if (!['xhttp', 'splithttp'].includes(String(down.method ?? down.network).toLowerCase()))
      issue(['network'], 'Select XHTTP for the download transport.');
    if (typeof down.address !== 'string' || !down.address.trim())
      issue(['address'], 'Enter a download address.');
    if (
      typeof down.port !== 'number' ||
      !Number.isInteger(down.port) ||
      down.port < 1 ||
      down.port > 65535
    )
      issue(['port'], 'Enter a port from 1 to 65535.');
    const security = String(down.security ?? '').toLowerCase();
    if (!['', 'none', 'tls', 'reality'].includes(security))
      issue(['security'], 'Select none, TLS or REALITY.');
    if (security === 'reality') {
      const r = down.realitySettings as Record<string, unknown> | undefined;
      const key = r?.password || r?.publicKey;
      if (typeof key !== 'string' || !/^[A-Za-z0-9_-]{43}$/.test(key))
        issue(
          ['realitySettings', r?.password || !r?.publicKey ? 'password' : 'publicKey'],
          'Enter a 32-byte unpadded base64url REALITY public key.',
        );
      if (
        r?.shortId != null &&
        (typeof r.shortId !== 'string' || !/^(?:[0-9a-fA-F]{2}){0,8}$/.test(r.shortId))
      )
        issue(['realitySettings', 'shortId'], 'Enter up to 8 hexadecimal bytes.');
    }
  });
export type InboundFormValues = z.infer<typeof InboundFormSchema>;

export const FallbackRowSchema = z.object({
  rowKey: z.string(),
  childId: z.number().int().nullable(),
  name: z.string().default(''),
  alpn: z.string().default(''),
  path: z.string().default(''),
  dest: z.string().default(''),
  xver: z.number().int().min(0).max(2).default(0),
});
export type FallbackRow = z.infer<typeof FallbackRowSchema>;
