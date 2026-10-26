import { z } from 'zod';

export const AWGPingRangeSchema = z
  .string()
  .max(32)
  .refine((value) => {
    const parts = value.split('/');
    if (parts.length > 2) return false;
    const octets = parts[0].split('.');
    if (
      octets.length !== 4 ||
      !octets.every((o) => /^(0|[1-9]\d{0,2})$/.test(o) && Number(o) <= 255)
    )
      return false;
    return parts.length === 1 || (/^(0|[1-9]\d?)$/.test(parts[1]) && Number(parts[1]) <= 32);
  }, 'IPv4 address or CIDR required');

export const AWGPingPolicySchema = z.object({
  enabled: z.boolean(),
  allow: z.array(AWGPingRangeSchema).max(256).default([]),
  deny: z.array(AWGPingRangeSchema).max(256).default([]),
});

export const AWGPingListInputSchema = z
  .string()
  .transform((value) => value.split(/[\s,;]+/).filter(Boolean))
  .pipe(z.array(AWGPingRangeSchema).max(256));
export const AWGPingReadinessSchema = z.object({ available: z.boolean() });
