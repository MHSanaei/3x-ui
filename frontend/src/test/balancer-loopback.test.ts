import { describe, it, expect } from 'vitest';

import type { XraySettingsValue } from '@/hooks/useXraySetting';
import {
  isBalancerLoopbackTag,
  loopbackTagFor,
  balancerTagFromLoopback,
  resolveLoopbackFallback,
  ensureBalancerLoopback,
  ensureMissingBalancerLoopbacks,
  removeBalancerLoopback,
  removeBalancerLoopbackIfOrphaned,
  propagateBalancerTagRename,
  detectBalancerCycles,
  cleanupOrphanedBalancerLoopbacks,
} from '@/pages/xray/balancers/balancer-loopback';

interface OutboundEntry {
  tag?: string;
  protocol?: string;
  settings?: { inboundTag?: string };
}
interface RuleEntry {
  type?: string;
  inboundTag?: string[];
  balancerTag?: string;
  domain?: string[];
}
interface BalancerEntry {
  tag?: string;
  selector?: string[];
  fallbackTag?: string;
}

function makeSettings(input: {
  outbounds?: OutboundEntry[];
  rules?: RuleEntry[];
  balancers?: BalancerEntry[];
}): XraySettingsValue {
  return {
    outbounds: input.outbounds,
    routing: {
      rules: input.rules,
      balancers: input.balancers,
    },
  } as XraySettingsValue;
}

function outboundTags(settings: XraySettingsValue): string[] {
  return ((settings.outbounds ?? []) as OutboundEntry[]).map((o) => o.tag ?? '');
}

function loopbackOutbounds(settings: XraySettingsValue): OutboundEntry[] {
  return ((settings.outbounds ?? []) as OutboundEntry[]).filter((o) => o.protocol === 'loopback');
}

function ruleEntries(settings: XraySettingsValue): RuleEntry[] {
  return (settings.routing?.rules ?? []) as RuleEntry[];
}

function balancerEntries(settings: XraySettingsValue): BalancerEntry[] {
  return (settings.routing?.balancers ?? []) as BalancerEntry[];
}

describe('loopback tag helpers', () => {
  const cases: Array<{ tag: string; isLoopback: boolean; roundtrip: string | null }> = [
    { tag: '_bl_main', isLoopback: true, roundtrip: 'main' },
    { tag: 'main', isLoopback: false, roundtrip: null },
    { tag: '_bl_', isLoopback: true, roundtrip: '' },
    { tag: 'proxy_bl_', isLoopback: false, roundtrip: null },
  ];

  it.each(cases)('classifies $tag', ({ tag, isLoopback, roundtrip }) => {
    expect(isBalancerLoopbackTag(tag)).toBe(isLoopback);
    expect(balancerTagFromLoopback(tag)).toBe(roundtrip);
  });

  it('builds a loopback tag that round-trips back to the balancer tag', () => {
    expect(loopbackTagFor('cluster-a')).toBe('_bl_cluster-a');
    expect(balancerTagFromLoopback(loopbackTagFor('cluster-a'))).toBe('cluster-a');
  });
});

describe('resolveLoopbackFallback', () => {
  const settings = makeSettings({
    rules: [{ type: 'field', inboundTag: ['_bl_bal1'], balancerTag: 'bal1' }],
  });

  const cases: Array<{ name: string; input: string; expected: string }> = [
    { name: 'resolves a loopback tag through its routing rule', input: '_bl_bal1', expected: 'bal1' },
    { name: 'returns a plain outbound tag unchanged', input: 'direct', expected: 'direct' },
    { name: 'returns an empty tag unchanged', input: '', expected: '' },
    { name: 'derives the balancer tag when no rule maps it', input: '_bl_bal2', expected: 'bal2' },
  ];

  it.each(cases)('$name', ({ input, expected }) => {
    expect(resolveLoopbackFallback(settings, input)).toBe(expected);
  });
});

describe('ensureBalancerLoopback dedup', () => {
  it('creates exactly one loopback outbound and one rule when called repeatedly', () => {
    const settings = makeSettings({ outbounds: [], rules: [], balancers: [] });

    ensureBalancerLoopback(settings, 'bal1');
    ensureBalancerLoopback(settings, 'bal1');

    const loopbacks = loopbackOutbounds(settings);
    expect(loopbacks).toHaveLength(1);
    expect(loopbacks[0]).toEqual({
      tag: '_bl_bal1',
      protocol: 'loopback',
      settings: { inboundTag: '_bl_bal1' },
    });

    const matchingRules = ruleEntries(settings).filter(
      (r) => Array.isArray(r.inboundTag) && r.inboundTag.includes('_bl_bal1'),
    );
    expect(matchingRules).toHaveLength(1);
    expect(matchingRules[0].balancerTag).toBe('bal1');
  });

  it('does not duplicate a loopback shared by multiple balancers', () => {
    const settings = makeSettings({
      outbounds: [],
      rules: [],
      balancers: [
        { tag: 'A', selector: [], fallbackTag: '_bl_shared' },
        { tag: 'B', selector: [], fallbackTag: '_bl_shared' },
      ],
    });

    ensureMissingBalancerLoopbacks(settings);

    expect(loopbackOutbounds(settings)).toHaveLength(1);
    expect(
      ruleEntries(settings).filter(
        (r) => Array.isArray(r.inboundTag) && r.inboundTag.includes('_bl_shared'),
      ),
    ).toHaveLength(1);
  });
});

describe('ensureBalancerLoopback rule ordering', () => {
  function loopbackRuleIndex(settings: XraySettingsValue, lbTag: string): number {
    return ruleEntries(settings).findIndex(
      (r) => Array.isArray(r.inboundTag) && r.inboundTag.includes(lbTag),
    );
  }
  function generalRuleIndex(settings: XraySettingsValue): number {
    return ruleEntries(settings).findIndex(
      (r) => !Array.isArray(r.inboundTag) || r.inboundTag.length === 0,
    );
  }

  it('inserts a new loopback rule ahead of a general (no inboundTag) rule', () => {
    const settings = makeSettings({
      rules: [{ type: 'field', domain: ['example.com'], balancerTag: 'parent' }],
      balancers: [{ tag: 'parent', selector: [] }],
    });

    ensureBalancerLoopback(settings, 'target');

    expect(loopbackRuleIndex(settings, '_bl_target')).toBeLessThan(
      generalRuleIndex(settings),
    );
  });

  it('repositions an existing loopback rule that landed after a general rule', () => {
    const settings = makeSettings({
      rules: [
        { type: 'field', domain: ['example.com'], balancerTag: 'parent' },
        { type: 'field', inboundTag: ['_bl_target'], balancerTag: 'stale' },
      ],
      balancers: [{ tag: 'parent', selector: [] }],
    });

    ensureBalancerLoopback(settings, 'target');

    const lbIdx = loopbackRuleIndex(settings, '_bl_target');
    expect(lbIdx).toBeLessThan(generalRuleIndex(settings));
    expect(ruleEntries(settings)[lbIdx].balancerTag).toBe('target');
  });

  it('leaves inboundTag-restricted rules in place and slots loopback ahead of general rules only', () => {
    const settings = makeSettings({
      rules: [
        { type: 'field', inboundTag: ['api'], balancerTag: 'stats' },
        { type: 'field', domain: ['example.com'], balancerTag: 'parent' },
      ],
      balancers: [{ tag: 'parent', selector: [] }],
    });

    ensureBalancerLoopback(settings, 'target');

    const entries = ruleEntries(settings);
    expect(entries[0].inboundTag).toEqual(['api']);
    const lbIdx = loopbackRuleIndex(settings, '_bl_target');
    const generalIdx = generalRuleIndex(settings);
    expect(lbIdx).toBeLessThan(generalIdx);
    expect(lbIdx).toBeGreaterThan(0);
  });

  it('ensureMissingBalancerLoopbacks repositions every mis-ordered loopback rule', () => {
    const settings = makeSettings({
      rules: [
        { type: 'field', domain: ['example.com'], balancerTag: 'B1' },
        { type: 'field', inboundTag: ['_bl_B2'], balancerTag: 'B2' },
      ],
      balancers: [
        { tag: 'B1', selector: [], fallbackTag: '_bl_B2' },
        { tag: 'B2', selector: [] },
      ],
    });

    ensureMissingBalancerLoopbacks(settings);

    expect(loopbackRuleIndex(settings, '_bl_B2')).toBeLessThan(
      generalRuleIndex(settings),
    );
  });

  it('keeps the loopback rule ahead of the general rule after a second ensureBalancerLoopback call', () => {
    const settings = makeSettings({
      rules: [{ type: 'field', domain: ['example.com'], balancerTag: 'parent' }],
      balancers: [{ tag: 'parent', selector: [] }],
    });

    ensureBalancerLoopback(settings, 'target');
    ensureBalancerLoopback(settings, 'target');

    expect(loopbackRuleIndex(settings, '_bl_target')).toBeLessThan(
      generalRuleIndex(settings),
    );
    expect(
      ruleEntries(settings).filter(
        (r) => Array.isArray(r.inboundTag) && r.inboundTag.includes('_bl_target'),
      ),
    ).toHaveLength(1);
  });
});

describe('detectBalancerCycles', () => {
  const cases: Array<{ name: string; balancers: BalancerEntry[]; expected: string[][] }> = [
    {
      name: 'two-balancer loop A -> B -> A',
      balancers: [
        { tag: 'A', fallbackTag: '_bl_B' },
        { tag: 'B', fallbackTag: '_bl_A' },
      ],
      expected: [
        ['A', 'B'],
        ['B', 'A'],
      ],
    },
    {
      name: 'self loop A -> A',
      balancers: [{ tag: 'A', fallbackTag: '_bl_A' }],
      expected: [['A', 'A']],
    },
    {
      name: 'three-balancer loop A -> B -> C -> A',
      balancers: [
        { tag: 'A', fallbackTag: '_bl_B' },
        { tag: 'B', fallbackTag: '_bl_C' },
        { tag: 'C', fallbackTag: '_bl_A' },
      ],
      expected: [
        ['A', 'B'],
        ['B', 'C'],
        ['C', 'A'],
      ],
    },
    {
      name: 'linear chain is not a cycle',
      balancers: [{ tag: 'A', fallbackTag: '_bl_B' }, { tag: 'B' }],
      expected: [],
    },
    {
      name: 'non-loopback fallback is ignored',
      balancers: [{ tag: 'A', fallbackTag: 'direct' }],
      expected: [],
    },
  ];

  it.each(cases)('$name', ({ balancers, expected }) => {
    expect(detectBalancerCycles(makeSettings({ balancers }))).toEqual(expected);
  });
});

describe('propagateBalancerTagRename', () => {
  it('rewrites the loopback outbound, its rule and referring fallback tags', () => {
    const settings = makeSettings({
      outbounds: [{ tag: '_bl_old', protocol: 'loopback', settings: { inboundTag: '_bl_old' } }],
      rules: [{ type: 'field', inboundTag: ['_bl_old'], balancerTag: 'old' }],
      balancers: [{ tag: 'user', selector: [], fallbackTag: '_bl_old' }],
    });

    propagateBalancerTagRename(settings, 'old', 'new');

    const [outbound] = loopbackOutbounds(settings);
    expect(outbound.tag).toBe('_bl_new');
    expect(outbound.settings?.inboundTag).toBe('_bl_new');
    expect(ruleEntries(settings)[0].inboundTag).toEqual(['_bl_new']);
    expect(balancerEntries(settings)[0].fallbackTag).toBe('_bl_new');
  });

  it('leaves unrelated loopback tags untouched', () => {
    const settings = makeSettings({
      outbounds: [{ tag: '_bl_other', protocol: 'loopback', settings: { inboundTag: '_bl_other' } }],
      rules: [{ type: 'field', inboundTag: ['_bl_other'], balancerTag: 'other' }],
      balancers: [{ tag: 'user', selector: [], fallbackTag: '_bl_other' }],
    });

    propagateBalancerTagRename(settings, 'old', 'new');

    expect(loopbackOutbounds(settings)[0].tag).toBe('_bl_other');
    expect(ruleEntries(settings)[0].inboundTag).toEqual(['_bl_other']);
    expect(balancerEntries(settings)[0].fallbackTag).toBe('_bl_other');
  });
});

describe('orphan cleanup', () => {
  it('cleanupOrphanedBalancerLoopbacks removes only unreferenced loopbacks', () => {
    const settings = makeSettings({
      outbounds: [
        { tag: '_bl_gone', protocol: 'loopback', settings: { inboundTag: '_bl_gone' } },
        { tag: '_bl_kept', protocol: 'loopback', settings: { inboundTag: '_bl_kept' } },
        { tag: 'proxy', protocol: 'vless' },
      ],
      rules: [
        { type: 'field', inboundTag: ['_bl_gone'], balancerTag: 'gone' },
        { type: 'field', inboundTag: ['_bl_kept'], balancerTag: 'kept' },
      ],
      balancers: [{ tag: 'user', selector: [], fallbackTag: '_bl_kept' }],
    });

    cleanupOrphanedBalancerLoopbacks(settings);

    expect(outboundTags(settings)).toEqual(['_bl_kept', 'proxy']);
    expect(ruleEntries(settings).map((r) => r.inboundTag)).toEqual([['_bl_kept']]);
  });

  it('removeBalancerLoopbackIfOrphaned keeps a loopback that is still referenced', () => {
    const settings = makeSettings({
      outbounds: [{ tag: '_bl_kept', protocol: 'loopback', settings: { inboundTag: '_bl_kept' } }],
      rules: [{ type: 'field', inboundTag: ['_bl_kept'], balancerTag: 'kept' }],
      balancers: [{ tag: 'user', selector: [], fallbackTag: '_bl_kept' }],
    });

    removeBalancerLoopbackIfOrphaned(settings, 'kept');

    expect(loopbackOutbounds(settings)).toHaveLength(1);
    expect(ruleEntries(settings)).toHaveLength(1);
  });

  it('removeBalancerLoopbackIfOrphaned drops a loopback with no referrers', () => {
    const settings = makeSettings({
      outbounds: [{ tag: '_bl_kept', protocol: 'loopback', settings: { inboundTag: '_bl_kept' } }],
      rules: [{ type: 'field', inboundTag: ['_bl_kept'], balancerTag: 'kept' }],
      balancers: [],
    });

    removeBalancerLoopbackIfOrphaned(settings, 'kept');

    expect(loopbackOutbounds(settings)).toHaveLength(0);
    expect(ruleEntries(settings)).toHaveLength(0);
  });

  it('removeBalancerLoopback deletes the outbound and rule directly', () => {
    const settings = makeSettings({
      outbounds: [
        { tag: '_bl_kept', protocol: 'loopback', settings: { inboundTag: '_bl_kept' } },
        { tag: 'proxy', protocol: 'vless' },
      ],
      rules: [{ type: 'field', inboundTag: ['_bl_kept'], balancerTag: 'kept' }],
      balancers: [{ tag: 'user', selector: [], fallbackTag: '_bl_kept' }],
    });

    removeBalancerLoopback(settings, 'kept');

    expect(outboundTags(settings)).toEqual(['proxy']);
    expect(ruleEntries(settings)).toHaveLength(0);
  });
});
