import { describe, expect, it } from 'vitest';

import { formatInboundLabel } from '@/lib/inbounds/label';

describe('formatInboundLabel', () => {
  it('uses remark when present', () => {
    expect(formatInboundLabel('vless-reality', 'JP Japan')).toBe('JP Japan');
  });

  it('appends the port when present', () => {
    expect(formatInboundLabel('vless-reality', 'JP Japan', 2053)).toBe('JP Japan : 2053');
  });

  it('keeps the original label when the port is missing', () => {
    expect(formatInboundLabel('vless-reality', 'JP Japan')).toBe('JP Japan');
  });
});
