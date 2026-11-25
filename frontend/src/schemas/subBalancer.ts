import { z } from 'zod';

export const SubBalancerStrategySchema = z.enum(['leastLoad', 'leastPing', 'random', 'roundRobin']);
export type SubBalancerStrategy = z.infer<typeof SubBalancerStrategySchema>;

export const SubBalancerProblemCodeSchema = z.enum([
  'unknown_inbound',
  'disabled_inbound',
  'json_incapable_protocol',
  'inbound_without_clients',
  'hosts_excluded_from_json',
  'no_members',
]);
export type SubBalancerProblemCode = z.infer<typeof SubBalancerProblemCodeSchema>;

export const SubBalancerProblemSchema = z.object({
  code: SubBalancerProblemCodeSchema,
  inboundIds: z.array(z.number()).optional(),
  detail: z.string().optional(),
});
export type SubBalancerProblem = z.infer<typeof SubBalancerProblemSchema>;

export const SubBalancerSchema = z.object({
  id: z.number(),
  remark: z.string(),
  strategy: SubBalancerStrategySchema,
  inboundIds: z.array(z.number()),
  memberWeights: z.record(z.string(), z.number()).nullish(),
  sortOrder: z.number(),
  enabled: z.boolean(),
  // Advisories from the list API; create/update responses omit them.
  problems: z.array(SubBalancerProblemSchema).optional(),
  createdAt: z.number().optional(),
  updatedAt: z.number().optional(),
});
export type SubBalancer = z.infer<typeof SubBalancerSchema>;

export const SubBalancerListSchema = z.array(SubBalancerSchema);

export const SubBalancerFormSchema = z.object({
  remark: z
    .string()
    .trim()
    .min(1, 'pages.settings.subBalancers.errRemarkRequired')
    .max(256, 'pages.settings.subBalancers.errRemarkRequired'),
  strategy: SubBalancerStrategySchema,
  inboundIds: z
    .array(z.number().int().positive())
    .min(1, 'pages.settings.subBalancers.errInboundsRequired'),
  // inboundId (stringified) -> leastLoad weight; absent members weigh 1.0.
  memberWeights: z
    .record(
      z.string(),
      z
        .number({ error: 'pages.settings.subBalancers.errWeightPositive' })
        .positive('pages.settings.subBalancers.errWeightPositive'),
    )
    .optional(),
  sortOrder: z
    .number({ error: 'pages.settings.subBalancers.errSortOrder' })
    .int('pages.settings.subBalancers.errSortOrder')
    .min(1, 'pages.settings.subBalancers.errSortOrder'),
  enabled: z.boolean(),
});
export type SubBalancerFormValues = z.infer<typeof SubBalancerFormSchema>;
