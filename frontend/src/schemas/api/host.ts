import { z } from 'zod';

import { AlpnSchema, UtlsFingerprintSchema } from '@/schemas/protocols/security/tls';

export const HostSecuritySchema = z.enum(['same', 'tls', 'none', 'reality']);
export type HostSecurity = z.infer<typeof HostSecuritySchema>;

export const MihomoIpVersionSchema = z.enum(['dual', 'ipv4', 'ipv6', 'ipv4-prefer', 'ipv6-prefer']);
export const SubTypeSchema = z.enum(['raw', 'json', 'clash']);

const HostTagSchema = z.string().regex(/^[A-Z0-9_:]+$/, 'pages.hosts.toasts.badTag').max(36);

export const HostFormSchema = z.object({
  id: z.number().optional(),
  inboundId: z.number().int().positive(),
  sortOrder: z.number().int().default(0),
  remark: z.string().trim().min(1).max(256),
  serverDescription: z.string().max(64).default(''),
  isDisabled: z.boolean().default(false),
  isHidden: z.boolean().default(false),
  tags: z.array(HostTagSchema).max(10).default([]),

  address: z.string().default(''),
  port: z.number().int().min(0).max(65535).default(0),

  security: HostSecuritySchema.default('same'),
  sni: z.string().default(''),
  hostHeader: z.string().default(''),
  path: z.string().default(''),
  alpn: z.array(AlpnSchema).default([]),
  fingerprint: z.preprocess(
    (val) => (val === '' ? undefined : val),
    UtlsFingerprintSchema.optional(),
  ),
  overrideSniFromAddress: z.boolean().default(false),
  keepSniBlank: z.boolean().default(false),
  pinnedPeerCertSha256: z.array(z.string()).default([]),
  verifyPeerCertByName: z.preprocess(
    (v) => (typeof v === 'boolean' ? '' : v),
    z.string().default(''),
  ),
  allowInsecure: z.boolean().default(false),
  echConfigList: z.string().default(''),

  muxParams: z.string().default(''),
  sockoptParams: z.string().default(''),
  finalMask: z.string().default(''),
  vlessRoute: z
    .string()
    .trim()
    .regex(/^\d{1,5}$/, 'pages.hosts.toasts.badVlessRoute')
    .refine((v) => Number(v) <= 65535, 'pages.hosts.toasts.badVlessRoute')
    .or(z.literal(''))
    .default(''),

  excludeFromSubTypes: z.array(SubTypeSchema).default([]),

  nodeGuids: z.array(z.string()).default([]),

  mihomoIpVersion: z.preprocess(
    (val) => (val === '' ? undefined : val),
    MihomoIpVersionSchema.optional(),
  ),
  mihomoX25519: z.boolean().default(false),
  shuffleHost: z.boolean().default(false),
});
export type HostFormValues = z.infer<typeof HostFormSchema>;

export const HostRecordSchema = z.object({
  groupId: z.string(),
  inboundIds: z.array(z.number()),
  hosts: z.array(z.string()),
  sortOrder: z.number().optional(),
  remark: z.string().optional(),
  serverDescription: z.string().optional(),
  isDisabled: z.boolean().optional(),
  isHidden: z.boolean().optional(),
  tags: z.array(z.string()).nullish(),
  port: z.number().optional(),
  security: z.string().optional(),
  sni: z.string().optional(),
  hostHeader: z.string().optional(),
  path: z.string().optional(),
  alpn: z.array(z.string()).nullish(),
  fingerprint: z.string().optional(),
  overrideSniFromAddress: z.boolean().optional(),
  keepSniBlank: z.boolean().optional(),
  pinnedPeerCertSha256: z.array(z.string()).nullish(),
  verifyPeerCertByName: z.preprocess(
    (v) => (typeof v === 'boolean' ? '' : v),
    z.string().optional(),
  ),
  allowInsecure: z.boolean().optional(),
  echConfigList: z.string().optional(),
  muxParams: z.unknown().optional(),
  sockoptParams: z.unknown().optional(),
  finalMask: z.string().optional(),
  vlessRoute: z.string().optional(),
  excludeFromSubTypes: z.array(z.string()).nullish(),
  nodeGuids: z.array(z.string()).nullish(),
  mihomoIpVersion: z.string().optional(),
  mihomoX25519: z.boolean().optional(),
  shuffleHost: z.boolean().optional(),
}).loose();
export type HostRecord = z.infer<typeof HostRecordSchema>;

export const HostListSchema = z.array(HostRecordSchema);

export const BulkAddHostSchema = HostFormSchema.omit({ inboundId: true, address: true }).extend({
  inboundIds: z.array(z.number().int().positive()).min(1),
  hosts: z.array(z.string()).default([]),
});
export type BulkAddHostValues = z.infer<typeof BulkAddHostSchema>;
