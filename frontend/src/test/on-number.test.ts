import { describe, expect, it, vi } from 'vitest';

import { onNumber } from '@/utils/onNumber';

describe('onNumber', () => {
  it('forwards numeric values, including zero and negatives', () => {
    const apply = vi.fn();
    const handler = onNumber(apply);
    handler(8443);
    handler(0);
    handler(-1);
    expect(apply.mock.calls).toEqual([[8443], [0], [-1]]);
  });

  it('ignores cleared events instead of writing a synthetic value', () => {
    const apply = vi.fn();
    const handler = onNumber(apply);
    handler(null);
    handler(undefined);
    handler(NaN);
    expect(apply).not.toHaveBeenCalled();
  });
});
