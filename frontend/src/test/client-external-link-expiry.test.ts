import { describe, expect, it } from 'vitest';

import { resolveExternalLinkExpiry } from '@/lib/clients/external-link';

describe('resolveExternalLinkExpiry', () => {
  it('uses the client expiry when the external link has no specific expiry', () => {
    expect(resolveExternalLinkExpiry(0, 1_800_000_000_000)).toBe(1_800_000_000_000);
  });

  it('keeps an explicit external-link expiry', () => {
    expect(resolveExternalLinkExpiry(1_700_000_000_000, 1_800_000_000_000)).toBe(1_700_000_000_000);
  });

  it('stays empty when neither expiry is set', () => {
    expect(resolveExternalLinkExpiry(0, 0)).toBe(0);
  });

  it('treats a negative client expiry (delayed-start encoding) as unset', () => {
    expect(resolveExternalLinkExpiry(0, -2_592_000_000)).toBe(0);
  });
});
