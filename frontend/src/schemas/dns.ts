import { z } from 'zod';

import { PortSchema } from '@/schemas/primitives';

export const DnsQueryStrategySchema = z.enum([
  'UseIP',
  'UseIPv4',
  'UseIPv6',
  'UseSystem',
]);
export type DnsQueryStrategy = z.infer<typeof DnsQueryStrategySchema>;

const DnsHostValueSchema = z.union([z.string(), z.array(z.string())]);
export const DnsHostsSchema = z.record(z.string(), DnsHostValueSchema);
export type DnsHosts = z.infer<typeof DnsHostsSchema>;

export const DnsServerObjectInnerSchema = z.object({
  address: z.string(),
  port: PortSchema.default(53),
  domains: z.array(z.string()).optional(),
  expectedIPs: z.array(z.string()).optional(),
  unexpectedIPs: z.array(z.string()).optional(),
  skipFallback: z.boolean().optional(),
  finalQuery: z.boolean().optional(),
  tag: z.string().optional(),
  clientIP: z.string().optional(),
  queryStrategy: DnsQueryStrategySchema.optional(),
  disableCache: z.boolean().optional(),
  timeoutMs: z.number().int().min(0).default(4000),
  serveStale: z.boolean().optional(),
  serveExpiredTTL: z.number().int().min(0).optional(),
});

export const DnsServerObjectSchema = z.preprocess(
  (val) => {
    if (typeof val !== 'object' || val === null || Array.isArray(val)) return val;
    const v = val as Record<string, unknown>;
    if (v.expectIPs && !v.expectedIPs) {
      return { ...v, expectedIPs: v.expectIPs };
    }
    return val;
  },
  DnsServerObjectInnerSchema,
);
export type DnsServerObject = z.infer<typeof DnsServerObjectSchema>;

export const DnsServerEntrySchema = z.union([z.string(), DnsServerObjectSchema]);
export type DnsServerEntry = z.infer<typeof DnsServerEntrySchema>;

export const DnsObjectSchema = z.object({
  tag: z.string().optional(),
  hosts: DnsHostsSchema.optional(),
  servers: z.array(DnsServerEntrySchema).optional(),
  clientIp: z.string().optional(),
  queryStrategy: DnsQueryStrategySchema.default('UseIP'),
  disableCache: z.boolean().default(false),
  disableFallback: z.boolean().default(false),
  disableFallbackIfMatch: z.boolean().default(false),
  enableParallelQuery: z.boolean().default(false),
  useSystemHosts: z.boolean().default(false),
  serveStale: z.boolean().default(false),
  serveExpiredTTL: z.number().int().min(0).default(0),
});
export type DnsObject = z.infer<typeof DnsObjectSchema>;
