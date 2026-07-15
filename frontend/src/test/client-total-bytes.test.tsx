import { describe, it, expect } from 'vitest';

import { resolveTotalBytes, gbToBytes } from '@/pages/clients/ClientFormModal';

describe('resolveTotalBytes', () => {
  it('preserves the original byte total on a no-op save of a non-GB-aligned quota', () => {
    const original = 11_505_016_832;
    const displayedGB = Math.round((original / 1024 ** 3) * 100) / 100;
    expect(resolveTotalBytes(original, displayedGB)).toBe(original);
  });

  it('re-derives bytes from GB when the user changed the quota', () => {
    const original = 11_505_016_832;
    expect(resolveTotalBytes(original, 20)).toBe(gbToBytes(20));
  });

  it('uses the GB value directly when there is no original (add mode)', () => {
    expect(resolveTotalBytes(null, 5)).toBe(gbToBytes(5));
  });
});
