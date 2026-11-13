import { z } from 'zod';

export const ChainProxyTypeSchema = z.enum(['socks', 'http']);
export type ChainProxyType = z.infer<typeof ChainProxyTypeSchema>;

export const ChainSelectorStrategySchema = z.enum([
  'leastLoad',
  'leastPing',
  'random',
  'roundRobin',
]);
export type ChainSelectorStrategy = z.infer<typeof ChainSelectorStrategySchema>;

export const ChainProxySchema = z.object({
  id: z.number(),
  remark: z.string(),
  type: ChainProxyTypeSchema,
  server: z.string(),
  port: z.number(),
  username: z.string(),
  password: z.string(),
  tls: z.boolean(),
  enabled: z.boolean(),
  createdAt: z.number().optional(),
  updatedAt: z.number().optional(),
});
export type ChainProxy = z.infer<typeof ChainProxySchema>;

export const ChainProxyListSchema = z.array(ChainProxySchema);

export const ChainProxyFormSchema = z.object({
  remark: z
    .string()
    .trim()
    .min(1, 'pages.settings.chainProxies.errRemarkRequired')
    .max(256, 'pages.settings.chainProxies.errRemarkRequired'),
  type: ChainProxyTypeSchema,
  server: z
    .string()
    .trim()
    .min(1, 'pages.settings.chainProxies.errServerRequired')
    .refine((value) => !/\s/.test(value), 'pages.settings.chainProxies.errServerInvalid'),
  port: z
    .number({ error: 'pages.settings.chainProxies.errPortRequired' })
    .int('pages.settings.chainProxies.errPortRequired')
    .min(1, 'pages.settings.chainProxies.errPortRequired')
    .max(65535, 'pages.settings.chainProxies.errPortRequired'),
  username: z.string().max(256),
  password: z.string().max(256),
  // xray's socks outbound has no TLS, so an HTTPS proxy is http-only.
  tls: z.boolean(),
  enabled: z.boolean(),
});
export type ChainProxyFormValues = z.infer<typeof ChainProxyFormSchema>;

export const ChainSelectorSchema = z.object({
  id: z.number(),
  remark: z.string(),
  strategy: ChainSelectorStrategySchema,
  proxyIds: z.array(z.number()),
  enabled: z.boolean(),
  createdAt: z.number().optional(),
  updatedAt: z.number().optional(),
});
export type ChainSelector = z.infer<typeof ChainSelectorSchema>;

export const ChainSelectorListSchema = z.array(ChainSelectorSchema);

export const ChainSelectorFormSchema = z.object({
  remark: z
    .string()
    .trim()
    .min(1, 'pages.settings.chainSelectors.errRemarkRequired')
    .max(256, 'pages.settings.chainSelectors.errRemarkRequired'),
  strategy: ChainSelectorStrategySchema,
  proxyIds: z
    .array(z.number().int().positive())
    .min(1, 'pages.settings.chainSelectors.errProxiesRequired'),
  enabled: z.boolean(),
});
export type ChainSelectorFormValues = z.infer<typeof ChainSelectorFormSchema>;
