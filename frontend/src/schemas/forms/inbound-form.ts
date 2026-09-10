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
      message: 'pages.inbounds.form.tlsServerCertificateRequired',
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
  lastTrafficResetTime: z.number().int().default(0),
  nodeId: z.number().int().nullable().optional(),
  shareAddrStrategy: ShareAddrStrategySchema.default('node'),
  shareAddr: z.string().default(''),
  subSortIndex: z.number().int().min(1).default(1),
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
export const InboundFormSchema =
  InboundFormBaseSchema.and(InboundDbFieldsSchema).and(InboundSettingsSchema);
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
