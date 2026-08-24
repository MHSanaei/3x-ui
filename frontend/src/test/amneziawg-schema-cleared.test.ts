import { describe, expect, it } from 'vitest';

import { AmneziawgServerSchema } from '@/schemas/protocols/inbound/amneziawg';

// AntD InputNumber emits null when cleared; a cleared numeric field must
// refill its schema default instead of failing validation and blocking the save.
describe('AmneziawgServerSchema cleared numeric fields', () => {
  it('accepts null for every InputNumber-backed field and refills the default', () => {
    const parsed = AmneziawgServerSchema.parse({
      subnetCidr: null,
      jc: null,
      jmin: null,
      jmax: null,
      s1: null,
      s2: null,
      s3: null,
      s4: null,
    });
    expect(parsed.subnetCidr).toBe(24);
    expect(parsed.jc).toBe(5);
    expect(parsed.jmin).toBe(10);
    expect(parsed.jmax).toBe(50);
    expect(parsed.s1).toBe(30);
    expect(parsed.s2).toBe(45);
    expect(parsed.s3).toBe(10);
    expect(parsed.s4).toBe(5);
  });

  it('keeps absent-key defaults unchanged', () => {
    const parsed = AmneziawgServerSchema.parse({});
    expect(parsed.subnetCidr).toBe(24);
    expect(parsed.jc).toBe(5);
  });
});
