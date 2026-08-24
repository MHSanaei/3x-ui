import { z } from 'zod';

export const SubBalancerStrategySchema = z.enum(['leastLoad', 'leastPing', 'random', 'roundRobin']);
export type SubBalancerStrategy = z.infer<typeof SubBalancerStrategySchema>;

export const SubBalancerSchema = z.object({
  id: z.number(),
  remark: z.string(),
  strategy: SubBalancerStrategySchema,
  inboundIds: z.array(z.number()),
  memberWeights: z.record(z.string(), z.number()).optional(),
  sortOrder: z.number(),
  enabled: z.boolean(),
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
        .number({ message: 'pages.settings.subBalancers.errWeightPositive' })
        .positive('pages.settings.subBalancers.errWeightPositive'),
    )
    .optional(),
  sortOrder: z
    .number({ message: 'pages.settings.subBalancers.errSortOrder' })
    .int('pages.settings.subBalancers.errSortOrder')
    .min(1, 'pages.settings.subBalancers.errSortOrder'),
  enabled: z.boolean(),
});
export type SubBalancerFormValues = z.infer<typeof SubBalancerFormSchema>;
