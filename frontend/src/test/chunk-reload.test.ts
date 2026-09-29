import { describe, expect, it } from 'vitest';

import { claimChunkReload } from '@/lib/chunk-reload';

function memoryStorage() {
  const data = new Map<string, string>();
  return {
    getItem: (k: string) => data.get(k) ?? null,
    setItem: (k: string, v: string) => void data.set(k, v),
  };
}

describe('claimChunkReload', () => {
  it('allows the first reload and blocks an immediate second one', () => {
    const s = memoryStorage();
    expect(claimChunkReload(s, 1_000_000)).toBe(true);
    expect(claimChunkReload(s, 1_003_000)).toBe(false);
  });

  it('allows recovery again once the loop window has passed', () => {
    const s = memoryStorage();
    expect(claimChunkReload(s, 1_000_000)).toBe(true);
    expect(claimChunkReload(s, 1_000_000 + 60_000)).toBe(true);
  });

  it('does not reload when storage is unavailable', () => {
    const broken = {
      getItem: () => {
        throw new Error('denied');
      },
      setItem: () => {
        throw new Error('denied');
      },
    };
    expect(claimChunkReload(broken, 1_000_000)).toBe(false);
  });
});
