import { z } from 'zod';

export const RuleProtocolSchema = z.enum(['http', 'tls', 'quic', 'bittorrent']);
export type RuleProtocol = z.infer<typeof RuleProtocolSchema>;

const PortValueSchema = z.union([
  z.number().int().min(0).max(65535),
  z.string(),
]);

export const RuleWebhookSchema = z.object({
  url: z.string(),
  deduplication: z.number().int().min(0).optional(),
  headers: z.record(z.string(), z.string()).optional(),
});
export type RuleWebhook = z.infer<typeof RuleWebhookSchema>;

export const RuleObjectSchema = z.object({
  type: z.literal('field').default('field'),
  domain: z.array(z.string()).optional(),
  ip: z.array(z.string()).optional(),
  port: PortValueSchema.optional(),
  sourcePort: PortValueSchema.optional(),
  localPort: PortValueSchema.optional(),
  network: z.string().optional(),
  sourceIP: z.array(z.string()).optional(),
  localIP: z.array(z.string()).optional(),
  user: z.array(z.string()).optional(),
  vlessRoute: PortValueSchema.optional(),
  inboundTag: z.array(z.string()).optional(),
  protocol: z.array(z.string()).optional(),
  attrs: z.record(z.string(), z.string()).optional(),
  process: z.array(z.string()).optional(),
  outboundTag: z.string().optional(),
  balancerTag: z.string().optional(),
  ruleTag: z.string().optional(),
  webhook: RuleWebhookSchema.optional(),
});
export type RuleObject = z.infer<typeof RuleObjectSchema>;

export const BalancerStrategyTypeSchema = z.enum([
  'random',
  'roundRobin',
  'leastPing',
  'leastLoad',
]);
export type BalancerStrategyType = z.infer<typeof BalancerStrategyTypeSchema>;

export const BalancerCostObjectSchema = z.object({
  regexp: z.boolean().default(false),
  match: z.string(),
  value: z.number(),
});
export type BalancerCostObject = z.infer<typeof BalancerCostObjectSchema>;

export const BalancerStrategySettingsSchema = z.object({
  expected: z.number().int().min(0).optional(),
  maxRTT: z.string().optional(),
  tolerance: z.number().min(0).max(1).optional(),
  baselines: z.array(z.string()).optional(),
  costs: z.array(BalancerCostObjectSchema).optional(),
});
export type BalancerStrategySettings = z.infer<typeof BalancerStrategySettingsSchema>;

export const BalancerStrategySchema = z.object({
  type: BalancerStrategyTypeSchema.default('random'),
  settings: BalancerStrategySettingsSchema.optional(),
});
export type BalancerStrategy = z.infer<typeof BalancerStrategySchema>;

export const BalancerObjectSchema = z.object({
  tag: z.string().trim().min(1),
  selector: z.array(z.string()).min(1),
  fallbackTag: z.string().optional(),
  strategy: BalancerStrategySchema.optional(),
});
export type BalancerObject = z.infer<typeof BalancerObjectSchema>;
