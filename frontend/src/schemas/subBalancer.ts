import { z } from 'zod';
 
export const SubBalancerStrategySchema = z.enum(['leastLoad', 'leastPing', 'random']);
export type SubBalancerStrategy = z.infer<typeof SubBalancerStrategySchema>;
 
export const SubBalancerSchema = z.object({
  id: z.number(),
  remark: z.string(),
  strategy: SubBalancerStrategySchema,
  inboundIds: z.array(z.number()),
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
  sortOrder: z
    .number({ message: 'pages.settings.subBalancers.errSortOrder' })
    .int('pages.settings.subBalancers.errSortOrder')
    .min(1, 'pages.settings.subBalancers.errSortOrder'),
  enabled: z.boolean(),
});
export type SubBalancerFormValues = z.infer<typeof SubBalancerFormSchema>;