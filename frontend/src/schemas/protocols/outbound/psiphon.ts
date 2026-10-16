import { z } from 'zod';

import { PortSchema } from '@/schemas/primitives';

export const PsiphonInstanceSchema = z.object({
  id: z.string(),
  countryCode: z.string(),
  socksPort: PortSchema,
  enabled: z.boolean(),
  status: z.string(),
  latencyMs: z.number().int().default(0),
  exitIp: z.string().default(''),
  createdAt: z.number().int().optional(),
});
export type PsiphonInstance = z.infer<typeof PsiphonInstanceSchema>;

export const PsiphonCountrySchema = z.object({
  code: z.string(),
  name: z.string(),
  flag: z.string(),
  added: z.boolean(),
  available: z.boolean(),
});
export type PsiphonCountry = z.infer<typeof PsiphonCountrySchema>;

export const PsiphonConfigSchema = z.object({
  configJson: z.string(),
  basePort: z.number().int().default(40001),
  autoRestart: z.boolean().default(true),
});
export type PsiphonConfig = z.infer<typeof PsiphonConfigSchema>;

export const PsiphonProbeResultSchema = z.object({
  ok: z.boolean(),
  latencyMs: z.number().int().default(0),
  exitIp: z.string().optional(),
  exitCountry: z.string().optional(),
  error: z.string().optional(),
});
export type PsiphonProbeResult = z.infer<typeof PsiphonProbeResultSchema>;
