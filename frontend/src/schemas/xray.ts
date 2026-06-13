import { z } from 'zod';
import { DnsObjectSchema } from './dns';
import {
  BalancerObjectSchema,
  BalancerStrategySettingsSchema,
  BalancerStrategyTypeSchema,
  RuleObjectSchema,
} from './routing';

export const XraySettingsValueSchema = z.object({
  inbounds: z.array(z.unknown()).optional(),
  outbounds: z
    .array(
      z.object({
        tag: z.string().optional(),
        protocol: z.string().optional(),
        settings: z.unknown().optional(),
        streamSettings: z.unknown().optional(),
      }).loose(),
    )
    .optional(),
  routing: z.object({
    rules: z.array(RuleObjectSchema).optional(),
    balancers: z.array(BalancerObjectSchema).optional(),
    domainStrategy: z.string().optional(),
  }).loose().optional(),
  dns: DnsObjectSchema.optional(),
  log: z.record(z.string(), z.unknown()).optional(),
  policy: z.object({
    system: z.record(z.string(), z.boolean()).optional(),
    levels: z.record(z.string(), z.record(z.string(), z.unknown())).optional(),
  }).loose().optional(),
  observatory: z.unknown().optional(),
  burstObservatory: z.unknown().optional(),
  fakedns: z.unknown().optional(),
}).loose();

export const XrayConfigPayloadSchema = z.object({
  xraySetting: XraySettingsValueSchema,
  inboundTags: z.array(z.string()).optional(),
  clientReverseTags: z.array(z.string()).optional(),
  outboundTestUrl: z.string().optional(),
  // Subscription outbounds are injected at runtime (not persisted in xraySetting).
  // They are provided here so the UI can display them and use their tags in
  // balancers / routing rules.
  subscriptionOutbounds: z.array(z.unknown()).optional(),
  subscriptionOutboundTags: z.array(z.string()).optional(),
}).loose();

export const OutboundTrafficRowSchema = z.object({
  tag: z.string(),
  up: z.number(),
  down: z.number(),
});

export const OutboundTrafficListSchema = z.array(OutboundTrafficRowSchema);

export const OutboundTestResultSchema = z.object({
  tag: z.string().optional(),
  success: z.boolean(),
  delay: z.number().optional(),
  error: z.string().optional(),
  mode: z.string().optional(),
  // HTTP-mode extras: status answered by the test URL plus the httptrace
  // timing breakdown (dial to local inbound / target TLS via the outbound /
  // time to first byte).
  httpStatus: z.number().optional(),
  connectMs: z.number().optional(),
  tlsMs: z.number().optional(),
  ttfbMs: z.number().optional(),
  endpoints: z
    .array(
      z.object({
        address: z.string(),
        delay: z.number().optional(),
        success: z.boolean(),
        error: z.string().optional(),
      }).loose(),
    )
    .optional(),
}).loose();

// Batch results from /xray/testOutbounds, aligned with the request order.
export const OutboundTestResultListSchema = z.array(OutboundTestResultSchema);

export const RuleFormSchema = z.object({
  enabled: z.boolean(),
  domain: z.string(),
  ip: z.string(),
  port: z.string(),
  sourcePort: z.string(),
  vlessRoute: z.string(),
  network: z.string(),
  sourceIP: z.string(),
  user: z.string(),
  inboundTag: z.array(z.string()),
  protocol: z.array(z.string()),
  attrs: z.array(z.tuple([z.string(), z.string()])),
  outboundTag: z.string(),
  balancerTag: z.string(),
});

export const BalancerFormSchema = z.object({
  tag: z.string().trim().min(1, 'pages.xray.balancerTagRequired'),
  strategy: BalancerStrategyTypeSchema.default('random'),
  selector: z.array(z.string()).min(1, 'pages.xray.balancerSelectorRequired'),
  fallbackTag: z.string().default(''),
  settings: BalancerStrategySettingsSchema.optional(),
});

export const OutboundTagSchema = z
  .string()
  .trim()
  .min(1, 'pages.xray.outboundTagRequired');

export type BalancerFormValues = z.infer<typeof BalancerFormSchema>;
export type RuleFormValues = z.infer<typeof RuleFormSchema>;
export type XraySettingsValue = z.infer<typeof XraySettingsValueSchema>;
export type XrayConfigPayload = z.infer<typeof XrayConfigPayloadSchema>;
export type OutboundTrafficRow = z.infer<typeof OutboundTrafficRowSchema>;
export type OutboundTestResult = z.infer<typeof OutboundTestResultSchema>;
