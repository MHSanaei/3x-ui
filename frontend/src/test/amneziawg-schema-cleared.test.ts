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

// The form must reject what amneziawg-go's UAPI parsers reject (device/uapi.go:
// jc/jmin/jmax uint32, s1-s4 uint16), or the save silently outlives the apply.
describe('AmneziawgServerSchema obfuscation bounds', () => {
  const overWidth: Array<[string, number]> = [
    ['s1', 65536],
    ['s2', 70000],
    ['s3', 65],
    ['s4', 33],
    ['jc', 4294967296],
    ['jmin', 4294967296],
    ['jmax', 5000000000],
  ];

  it.each(overWidth)('rejects %s above the width amneziawg-go parses', (field, value) => {
    expect(AmneziawgServerSchema.safeParse({ [field]: value }).success).toBe(false);
  });

  const atLimit: Array<[string, number]> = [
    ['s1', 65535],
    ['s2', 65535],
    ['s3', 64],
    ['s4', 32],
    ['jc', 4294967295],
  ];

  it.each(atLimit)('accepts %s exactly at its limit', (field, value) => {
    const parsed = AmneziawgServerSchema.safeParse({ [field]: value });
    expect(parsed.success).toBe(true);
  });

  it('still rejects negatives on every junk and padding field', () => {
    for (const field of ['jc', 'jmin', 'jmax', 's1', 's2', 's3', 's4']) {
      expect(AmneziawgServerSchema.safeParse({ [field]: -1 }).success).toBe(false);
    }
  });
});
