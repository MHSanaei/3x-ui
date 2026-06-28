import { z } from 'zod';

export const ObservatorySchema = z
  .object({
    subjectSelector: z.array(z.string()).default([]),
    probeURL: z.string().default('https://www.google.com/generate_204'),
    probeInterval: z.string().default('1m'),
    enableConcurrency: z.boolean().default(true),
  })
  .loose();
export type ObservatoryObject = z.infer<typeof ObservatorySchema>;

export const ObservatoryHttpMethodSchema = z.enum(['HEAD', 'GET']);
export type ObservatoryHttpMethod = z.infer<typeof ObservatoryHttpMethodSchema>;

export const PingConfigSchema = z
  .object({
    destination: z.string().default('https://www.google.com/generate_204'),
    connectivity: z.string().default('http://connectivitycheck.platform.hicloud.com/generate_204'),
    interval: z.string().default('1m'),
    timeout: z.string().default('5s'),
    sampling: z.number().int().min(1).default(2),
    httpMethod: ObservatoryHttpMethodSchema.default('HEAD'),
  })
  .loose();
export type PingConfigObject = z.infer<typeof PingConfigSchema>;

export const BurstObservatorySchema = z
  .object({
    subjectSelector: z.array(z.string()).default([]),
    pingConfig: PingConfigSchema.default(PingConfigSchema.parse({})),
  })
  .loose();
export type BurstObservatoryObject = z.infer<typeof BurstObservatorySchema>;
