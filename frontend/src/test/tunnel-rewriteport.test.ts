import { describe, expect, it } from 'vitest';

import { TunnelInboundSettingsSchema } from '@/schemas/protocols/inbound/tunnel';

// Regression for issue #5516: AntD InputNumber writes null when the Rewrite
// port field is cleared, which used to crash validation with "Invalid input".
describe('TunnelInboundSettingsSchema rewritePort', () => {
  it('accepts null (cleared field) and omits the port', () => {
    const parsed = TunnelInboundSettingsSchema.parse({ rewritePort: null });
    expect(parsed.rewritePort).toBeUndefined();
  });

  it('accepts a missing field', () => {
    const parsed = TunnelInboundSettingsSchema.parse({});
    expect(parsed.rewritePort).toBeUndefined();
  });

  it('preserves a valid port', () => {
    const parsed = TunnelInboundSettingsSchema.parse({ rewritePort: 8443 });
    expect(parsed.rewritePort).toBe(8443);
  });

  it('still rejects out-of-range ports', () => {
    expect(() => TunnelInboundSettingsSchema.parse({ rewritePort: 70000 })).toThrow();
  });
});
