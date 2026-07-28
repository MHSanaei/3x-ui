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

  it('parses string values from stringMode inputs', () => {
    const apply = vi.fn();
    onNumber(apply)('2096');
    expect(apply).toHaveBeenCalledWith(2096);
  });

  it('ignores cleared and non-numeric events instead of writing a synthetic value', () => {
    const apply = vi.fn();
    const handler = onNumber(apply);
    handler(null);
    handler(undefined);
    handler('');
    handler('abc');
    handler(NaN);
    expect(apply).not.toHaveBeenCalled();
  });
});
