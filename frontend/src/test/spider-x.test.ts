import { describe, expect, it } from 'vitest';

import { deriveSpiderX } from '@/lib/xray/spider-x';

// Cross-language vectors shared with TestDeriveSpiderXMatchesFrontendVectors
// in internal/sub/service_sharelink_test.go: subscription links come from Go,
// panel links from this module, and the two must agree byte-for-byte.
describe('deriveSpiderX', () => {
  it('matches the Go deriveSpiderX vectors', () => {
    expect(deriveSpiderX('/seed', 'subAlice')).toBe('/c252fbc3ecd3e3c');
    expect(deriveSpiderX('/', '')).toBe('/d08ed99bd9afc60');
    expect(deriveSpiderX('/seed?p=40-400&r=500-2000', 'subAlice')).toBe(
      '/09dd00b3f8c01f5?p=40-400&r=500-2000',
    );
    expect(deriveSpiderX('/?p=40-400&c=1-4&t=1-3&i=1500-6000&r=500-2000', '')).toBe(
      '/ac2cb268d22908e?p=40-400&c=1-4&t=1-3&i=1500-6000&r=500-2000',
    );
  });

  it('is stable per client, distinct across clients, and rotates with the seed', () => {
    expect(deriveSpiderX('/seed', 'subAlice')).toBe(deriveSpiderX('/seed', 'subAlice'));
    expect(deriveSpiderX('/seed', 'subAlice')).not.toBe(deriveSpiderX('/seed', 'subBob'));
    expect(deriveSpiderX('/seedA', 'subAlice')).not.toBe(deriveSpiderX('/seedB', 'subAlice'));
  });

  it('returns empty when there is nothing to derive from', () => {
    expect(deriveSpiderX('', '')).toBe('');
  });

  it('emits a /-prefixed 15-hex-char path', () => {
    expect(deriveSpiderX('/some-seed', 'client@example.com')).toMatch(/^\/[0-9a-f]{15}$/);
  });
});
