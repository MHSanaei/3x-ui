import { describe, expect, it } from 'vitest';
import { z } from 'zod';

import { NaiveOutboundSettingsSchema } from '@/schemas/protocols/outbound/naive';

describe('NaiveOutboundSettingsSchema', () => {
  it('accepts a minimal valid payload', () => {
    const result = NaiveOutboundSettingsSchema.safeParse({
      proxy: 'https://user:pass@example.com:443',
    });
    expect(result.success).toBe(true);
  });

  it('accepts all optional fields', () => {
    const result = NaiveOutboundSettingsSchema.safeParse({
      proxy: 'quic://user:pass@example.com:443',
      insecureConcurrency: 4,
      tunnelTimeout: 1800,
      idleTimeout: 600,
      extraHeaders: 'X-Custom: value',
      hostResolverRules: 'MAP * ~NOTFOUND',
      resolverRange: '100.64.0.0/10',
      noPostQuantum: true,
    });
    expect(result.success).toBe(true);
  });

  it('rejects insecureConcurrency out of range', () => {
    const result = NaiveOutboundSettingsSchema.safeParse({
      proxy: 'https://user:pass@example.com:443',
      insecureConcurrency: 0,
    });
    expect(result.success).toBe(false);
  });

  it('rejects insecureConcurrency above max', () => {
    const result = NaiveOutboundSettingsSchema.safeParse({
      proxy: 'https://user:pass@example.com:443',
      insecureConcurrency: 9,
    });
    expect(result.success).toBe(false);
  });

  it('defaults proxy to empty string when omitted', () => {
    const result = NaiveOutboundSettingsSchema.safeParse({});
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.proxy).toBe('');
    }
  });

  it('accepts negative tunnelTimeout as zero boundary', () => {
    const result = NaiveOutboundSettingsSchema.safeParse({
      proxy: 'https://user:pass@host.com',
      tunnelTimeout: -1,
    });
    expect(result.success).toBe(false);
  });

  it('is compatible with z.infer type', () => {
    type T = z.infer<typeof NaiveOutboundSettingsSchema>;
    const val: T = { proxy: 'https://u:p@h.com', noPostQuantum: false };
    expect(val.proxy).toBeTruthy();
  });
});
