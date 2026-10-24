import { describe, it, expect } from 'vitest';
import {
  resolveClientActiveProfiles,
  ActiveProfileTreeSchema,
} from '@/lib/clients/active-profiles';

describe('exact active profile attribution', () => {
  const inbounds = {
    1: { id: 1, tag: 'same-tag', originNodeGuid: 'a', protocol: 'vless' },
    2: { id: 2, tag: 'same-tag', originNodeGuid: 'b', protocol: 'vless' },
    3: { id: 3, tag: 'hysteria', originNodeGuid: 'a', protocol: 'hysteria' },
  };
  const tree = {
    a: {
      supported: true,
      observedAt: 1,
      clients: { alice: ['same-tag', 'hysteria'], bob: ['hysteria'] },
    },
    b: { supported: true, observedAt: 1, clients: { bob: ['same-tag'] } },
  };
  it('matches the authenticated client, hosting node, tag and attachments', () => {
    expect(resolveClientActiveProfiles('alice', [1, 2, 3], inbounds, tree)).toEqual({
      activeIds: [1, 3],
      unknown: false,
    });
    expect(resolveClientActiveProfiles('bob', [1, 2, 3], inbounds, tree)).toEqual({
      activeIds: [2, 3],
      unknown: false,
    });
    expect(resolveClientActiveProfiles('alice', [2], inbounds, tree).activeIds).toEqual([]);
  });
  it('clears disconnected profiles and distinguishes unknown cores', () => {
    expect(
      resolveClientActiveProfiles('alice', [1], inbounds, {
        a: { supported: true, observedAt: 2, clients: {} },
      }),
    ).toEqual({ activeIds: [], unknown: false });
    expect(
      resolveClientActiveProfiles('alice', [1], inbounds, {
        a: { supported: false, observedAt: 2, clients: { alice: ['same-tag'] } },
      }),
    ).toEqual({ activeIds: [], unknown: true });
    expect(resolveClientActiveProfiles('alice', [1], inbounds, undefined).unknown).toBe(true);
  });
  it('validates snapshots before putting them into the live cache', () => {
    expect(ActiveProfileTreeSchema.safeParse(tree).success).toBe(true);
    expect(
      ActiveProfileTreeSchema.safeParse({ a: { supported: true, clients: { alice: 'all' } } })
        .success,
    ).toBe(false);
  });
  it('handles client names that are JavaScript object property names', () => {
    expect(resolveClientActiveProfiles('constructor', [1], inbounds, tree)).toEqual({
      activeIds: [],
      unknown: false,
    });
    expect(resolveClientActiveProfiles('__proto__', [1], inbounds, tree)).toEqual({
      activeIds: [],
      unknown: false,
    });
  });
  it('does not claim kernel-native or unauthenticated tunnels are measured', () => {
    const result = resolveClientActiveProfiles(
      'alice',
      [1],
      { 1: { id: 1, tag: 'wg', originNodeGuid: 'a', protocol: 'wireguard' } },
      tree,
    );
    expect(result).toEqual({ activeIds: [], unknown: true });
  });
});
