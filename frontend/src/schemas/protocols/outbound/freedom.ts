import { z } from 'zod';

export const OutboundDomainStrategySchema = z.enum([
  'AsIs',
  'UseIP',
  'UseIPv4',
  'UseIPv6',
  'UseIPv6v4',
  'UseIPv4v6',
  'ForceIP',
  'ForceIPv6v4',
  'ForceIPv6',
  'ForceIPv4v6',
  'ForceIPv4',
]);
export type OutboundDomainStrategy = z.infer<typeof OutboundDomainStrategySchema>;

// Fragment knobs are TCP-level splitting controls; all four fields are
// dash-range strings (e.g. '1-3', '10-20').
export const FreedomFragmentSchema = z.object({
  packets: z.string().default('1-3'),
  length: z.string().default(''),
  interval: z.string().default(''),
  maxSplit: z.string().default(''),
});
export type FreedomFragment = z.infer<typeof FreedomFragmentSchema>;

export const FreedomNoiseTypeSchema = z.enum(['rand', 'str', 'base64', 'hex']);
export const FreedomNoiseApplyToSchema = z.enum(['ip', 'ipv4', 'ipv6']);

export const FreedomNoiseSchema = z.object({
  type: FreedomNoiseTypeSchema.default('rand'),
  packet: z.string().default('10-20'),
  delay: z.string().default('10-16'),
  applyTo: FreedomNoiseApplyToSchema.default('ip'),
});
export type FreedomNoise = z.infer<typeof FreedomNoiseSchema>;

export const FreedomFinalRuleActionSchema = z.enum(['allow', 'block']);

// Final rules express the legacy ipsBlocked behavior plus generalized
// allow/block per network+port+ip combinations.
export const FreedomFinalRuleSchema = z.object({
  action: FreedomFinalRuleActionSchema.default('block'),
  network: z.string().optional(),
  port: z.string().optional(),
  ip: z.array(z.string()).default([]),
  blockDelay: z.string().optional(),
});
export type FreedomFinalRule = z.infer<typeof FreedomFinalRuleSchema>;

export const FreedomOutboundSettingsSchema = z.object({
  domainStrategy: OutboundDomainStrategySchema.optional(),
  redirect: z.string().optional(),
  userLevel: z.number().int().min(0).optional(),
  proxyProtocol: z.number().optional(),
  fragment: FreedomFragmentSchema.optional(),
  noises: z.array(FreedomNoiseSchema).optional(),
  finalRules: z.array(FreedomFinalRuleSchema).optional(),
});
export type FreedomOutboundSettings = z.infer<typeof FreedomOutboundSettingsSchema>;
