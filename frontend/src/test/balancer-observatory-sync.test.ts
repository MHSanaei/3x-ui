import { describe, expect, it } from 'vitest';

import { syncObservatories } from '@/pages/xray/balancers/balancer-helpers';
import type { XraySettingsValue } from '@/hooks/useXraySetting';

function tpl(routing: Record<string, unknown>, extra: Record<string, unknown> = {}): XraySettingsValue {
  return { routing, ...extra } as unknown as XraySettingsValue;
}

type ExpectedObserver = 'none' | 'observatory' | 'burstObservatory';

function expectObserver(t: XraySettingsValue, expected: ExpectedObserver, selectors: string[] = []) {
  if (expected === 'none') {
    expect(t.observatory).toBeUndefined();
    expect(t.burstObservatory).toBeUndefined();
    return;
  }

  if (expected === 'observatory') {
    expect(t.observatory).toBeDefined();
    expect(t.burstObservatory).toBeUndefined();
    expect(new Set((t.observatory as { subjectSelector: string[] }).subjectSelector)).toEqual(new Set(selectors));
    return;
  }

  expect(t.observatory).toBeUndefined();
  expect(t.burstObservatory).toBeDefined();
  expect(new Set((t.burstObservatory as { subjectSelector: string[] }).subjectSelector)).toEqual(new Set(selectors));
}

// Observatory sections have no reload API in xray-core, so creating one turns
// a balancer save from a live (hot-applied) routing change into a full
// restart. These tests pin the rule: only strategies that genuinely need an
// observer may create one — which, for random/roundRobin, means a fallbackTag
// is set (xray-core then requires the Observatory feature; see #5605).
describe('syncObservatories', () => {
  it.each([
    {
      name: 'random without fallback',
      balancers: [{ tag: 'random', selector: ['random-out'] }],
      expected: 'none' as const,
      selectors: [],
    },
    {
      name: 'random with fallback',
      balancers: [{ tag: 'random', selector: ['random-out'], fallbackTag: 'direct' }],
      expected: 'burstObservatory' as const,
      selectors: ['random-out'],
    },
    {
      name: 'roundRobin without fallback',
      balancers: [{ tag: 'rr', selector: ['rr-out'], strategy: { type: 'roundRobin' } }],
      expected: 'none' as const,
      selectors: [],
    },
    {
      name: 'roundRobin with fallback',
      balancers: [{ tag: 'rr', selector: ['rr-out'], fallbackTag: 'direct', strategy: { type: 'roundRobin' } }],
      expected: 'burstObservatory' as const,
      selectors: ['rr-out'],
    },
    {
      name: 'leastPing without fallback',
      balancers: [{ tag: 'lp', selector: ['lp-out'], strategy: { type: 'leastPing' } }],
      expected: 'observatory' as const,
      selectors: ['lp-out'],
    },
    {
      name: 'leastPing with fallback',
      balancers: [{ tag: 'lp', selector: ['lp-out'], fallbackTag: 'direct', strategy: { type: 'leastPing' } }],
      expected: 'observatory' as const,
      selectors: ['lp-out'],
    },
    {
      name: 'leastLoad without fallback',
      balancers: [{ tag: 'll', selector: ['ll-out'], strategy: { type: 'leastLoad' } }],
      expected: 'burstObservatory' as const,
      selectors: ['ll-out'],
    },
    {
      name: 'leastLoad with fallback',
      balancers: [{ tag: 'll', selector: ['ll-out'], fallbackTag: 'direct', strategy: { type: 'leastLoad' } }],
      expected: 'burstObservatory' as const,
      selectors: ['ll-out'],
    },
  ])('covers standalone strategy: $name', ({ balancers, expected, selectors }) => {
    const t = tpl({ balancers });
    syncObservatories(t);
    expectObserver(t, expected, selectors);
  });

  it.each([
    {
      name: 'random + roundRobin without fallbacks',
      balancers: [
        { tag: 'random', selector: ['random-out'] },
        { tag: 'rr', selector: ['rr-out'], strategy: { type: 'roundRobin' } },
      ],
      expected: 'none' as const,
      selectors: [],
    },
    {
      name: 'leastPing + random without fallback',
      balancers: [
        { tag: 'lp', selector: ['lp-out'], strategy: { type: 'leastPing' } },
        { tag: 'random', selector: ['random-out'] },
      ],
      expected: 'observatory' as const,
      selectors: ['lp-out'],
    },
    {
      name: 'leastPing + roundRobin without fallback',
      balancers: [
        { tag: 'lp', selector: ['lp-out'], strategy: { type: 'leastPing' } },
        { tag: 'rr', selector: ['rr-out'], strategy: { type: 'roundRobin' } },
      ],
      expected: 'observatory' as const,
      selectors: ['lp-out'],
    },
    {
      name: 'random fallback + random without fallback',
      balancers: [
        { tag: 'rf', selector: ['random-fallback-out'], fallbackTag: 'direct' },
        { tag: 'random', selector: ['random-out'] },
      ],
      expected: 'burstObservatory' as const,
      selectors: ['random-fallback-out'],
    },
    {
      name: 'roundRobin fallback + roundRobin without fallback',
      balancers: [
        { tag: 'rrf', selector: ['rr-fallback-out'], fallbackTag: 'direct', strategy: { type: 'roundRobin' } },
        { tag: 'rr', selector: ['rr-out'], strategy: { type: 'roundRobin' } },
      ],
      expected: 'burstObservatory' as const,
      selectors: ['rr-fallback-out'],
    },
    {
      name: 'leastLoad + random without fallback',
      balancers: [
        { tag: 'll', selector: ['ll-out'], strategy: { type: 'leastLoad' } },
        { tag: 'random', selector: ['random-out'] },
      ],
      expected: 'burstObservatory' as const,
      selectors: ['ll-out'],
    },
    {
      name: 'leastLoad + roundRobin without fallback',
      balancers: [
        { tag: 'll', selector: ['ll-out'], strategy: { type: 'leastLoad' } },
        { tag: 'rr', selector: ['rr-out'], strategy: { type: 'roundRobin' } },
      ],
      expected: 'burstObservatory' as const,
      selectors: ['ll-out'],
    },
    {
      name: 'leastPing + leastLoad with disjoint selectors',
      balancers: [
        { tag: 'lp', selector: ['lp-out', 'direct'], strategy: { type: 'leastPing' } },
        { tag: 'll', selector: ['ll-out-1', 'll-out-2'], strategy: { type: 'leastLoad' } },
      ],
      expected: 'burstObservatory' as const,
      selectors: ['lp-out', 'direct', 'll-out-1', 'll-out-2'],
    },
    {
      name: 'leastPing + random fallback',
      balancers: [
        { tag: 'lp', selector: ['lp-out'], strategy: { type: 'leastPing' } },
        { tag: 'rf', selector: ['random-fallback-out'], fallbackTag: 'direct' },
      ],
      expected: 'burstObservatory' as const,
      selectors: ['lp-out', 'random-fallback-out'],
    },
    {
      name: 'leastPing + roundRobin fallback',
      balancers: [
        { tag: 'lp', selector: ['lp-out'], strategy: { type: 'leastPing' } },
        { tag: 'rrf', selector: ['rr-fallback-out'], fallbackTag: 'direct', strategy: { type: 'roundRobin' } },
      ],
      expected: 'burstObservatory' as const,
      selectors: ['lp-out', 'rr-fallback-out'],
    },
    {
      name: 'all strategies mixed',
      balancers: [
        { tag: 'random', selector: ['random-out'] },
        { tag: 'rr', selector: ['rr-out'], strategy: { type: 'roundRobin' } },
        { tag: 'rf', selector: ['random-fallback-out'], fallbackTag: 'direct' },
        { tag: 'rrf', selector: ['rr-fallback-out'], fallbackTag: 'direct', strategy: { type: 'roundRobin' } },
        { tag: 'lp', selector: ['lp-out'], strategy: { type: 'leastPing' } },
        { tag: 'll', selector: ['ll-out'], strategy: { type: 'leastLoad' } },
      ],
      expected: 'burstObservatory' as const,
      selectors: ['random-fallback-out', 'rr-fallback-out', 'lp-out', 'll-out'],
    },
    {
      name: 'shared selectors are de-duplicated',
      balancers: [
        { tag: 'lp', selector: ['shared', 'lp-only'], strategy: { type: 'leastPing' } },
        { tag: 'll', selector: ['shared', 'll-only'], strategy: { type: 'leastLoad' } },
        { tag: 'rf', selector: ['shared', 'rf-only'], fallbackTag: 'direct' },
      ],
      expected: 'burstObservatory' as const,
      selectors: ['shared', 'lp-only', 'll-only', 'rf-only'],
    },
  ])('covers mixed strategy matrix: $name', ({ balancers, expected, selectors }) => {
    const t = tpl({ balancers });
    syncObservatories(t);
    expectObserver(t, expected, selectors);
  });

  it('does not create burstObservatory for a fresh random balancer (stays hot-appliable)', () => {
    const t = tpl({ balancers: [{ tag: 'b1', selector: ['direct'] }] });
    syncObservatories(t);
    expect(t.burstObservatory).toBeUndefined();
    expect(t.observatory).toBeUndefined();
  });

  it('does not create burstObservatory for roundRobin without fallback', () => {
    const t = tpl({ balancers: [{ tag: 'b1', selector: ['a'], strategy: { type: 'roundRobin' } }] });
    syncObservatories(t);
    expect(t.burstObservatory).toBeUndefined();
  });

  it('creates burstObservatory for a random balancer with a fallbackTag (#5605)', () => {
    const t = tpl({ balancers: [{ tag: 'OverProxy', selector: ['opera-proxy'], fallbackTag: 'warp' }] });
    syncObservatories(t);
    expect(t.burstObservatory).toBeDefined();
    expect((t.burstObservatory as { subjectSelector: string[] }).subjectSelector).toEqual(['opera-proxy']);
  });

  it('creates burstObservatory for roundRobin with a fallbackTag', () => {
    const t = tpl({ balancers: [{ tag: 'b1', selector: ['a'], fallbackTag: 'warp', strategy: { type: 'roundRobin' } }] });
    syncObservatories(t);
    expect(t.burstObservatory).toBeDefined();
    expect((t.burstObservatory as { subjectSelector: string[] }).subjectSelector).toEqual(['a']);
  });

  it('treats an empty-string fallbackTag as no fallback (stays hot-appliable)', () => {
    const t = tpl({ balancers: [{ tag: 'b1', selector: ['a'], fallbackTag: '' }] });
    syncObservatories(t);
    expect(t.burstObservatory).toBeUndefined();
  });

  it('removes burstObservatory when a random balancer drops its fallbackTag', () => {
    const t = tpl(
      { balancers: [{ tag: 'OverProxy', selector: ['opera-proxy'], fallbackTag: '' }] },
      { burstObservatory: { subjectSelector: ['opera-proxy'] } },
    );
    syncObservatories(t);
    expect(t.burstObservatory).toBeUndefined();
  });

  it('removes burstObservatory when a roundRobin balancer drops its fallbackTag', () => {
    const t = tpl(
      { balancers: [{ tag: 'b1', selector: ['a'], strategy: { type: 'roundRobin' } }] },
      { burstObservatory: { subjectSelector: ['a'] } },
    );
    syncObservatories(t);
    expect(t.burstObservatory).toBeUndefined();
  });

  it('keeps burstObservatory while another fallback balancer still needs it without adding no-fallback selectors', () => {
    const t = tpl(
      {
        balancers: [
          { tag: 'b1', selector: ['a'] },
          { tag: 'b2', selector: ['b'], fallbackTag: 'warp', strategy: { type: 'roundRobin' } },
        ],
      },
      { burstObservatory: { subjectSelector: ['a', 'b'] } },
    );
    syncObservatories(t);
    expect(t.burstObservatory).toBeDefined();
    expect((t.burstObservatory as { subjectSelector: string[] }).subjectSelector).toEqual(['b']);
  });

  it('creates burstObservatory for leastLoad (required by the strategy)', () => {
    const t = tpl({ balancers: [{ tag: 'b1', selector: ['a'], strategy: { type: 'leastLoad' } }] });
    syncObservatories(t);
    expect(t.burstObservatory).toBeDefined();
    expect((t.burstObservatory as { subjectSelector: string[] }).subjectSelector).toEqual(['a']);
  });

  it('creates observatory for leastPing when no burst observer is required', () => {
    const t = tpl({ balancers: [{ tag: 'b1', selector: ['a'], strategy: { type: 'leastPing' } }] });
    syncObservatories(t);
    expect(t.observatory).toBeDefined();
    expect((t.observatory as { subjectSelector: string[] }).subjectSelector).toEqual(['a']);
  });

  it('uses only burstObservatory when leastPing is mixed with leastLoad', () => {
    const t = tpl(
      {
        balancers: [
          { tag: 'lp', selector: ['least-ping-out'], strategy: { type: 'leastPing' } },
          { tag: 'll', selector: ['least-load-out'], strategy: { type: 'leastLoad' } },
        ],
      },
      { observatory: { subjectSelector: ['stale-least-ping-out'] } },
    );
    syncObservatories(t);
    expect(t.observatory).toBeUndefined();
    expect(new Set((t.burstObservatory as { subjectSelector: string[] }).subjectSelector)).toEqual(
      new Set(['least-load-out', 'least-ping-out']),
    );
  });

  it('uses only burstObservatory when leastPing is mixed with fallback balancers', () => {
    const t = tpl(
      {
        balancers: [
          { tag: 'lp', selector: ['least-ping-out'], strategy: { type: 'leastPing' } },
          { tag: 'rf', selector: ['random-fallback-out'], fallbackTag: 'direct' },
          { tag: 'rr', selector: ['round-robin-fallback-out'], fallbackTag: 'direct', strategy: { type: 'roundRobin' } },
        ],
      },
      { observatory: { subjectSelector: ['stale-least-ping-out'] } },
    );
    syncObservatories(t);
    expect(t.observatory).toBeUndefined();
    expect(new Set((t.burstObservatory as { subjectSelector: string[] }).subjectSelector)).toEqual(
      new Set(['random-fallback-out', 'round-robin-fallback-out', 'least-ping-out']),
    );
  });

  it('does not keep no-fallback random selectors in an observer created by another balancer', () => {
    const t = tpl(
      { balancers: [{ tag: 'b1', selector: ['a'] }, { tag: 'b2', selector: ['b'], strategy: { type: 'leastLoad' } }] },
      { burstObservatory: { subjectSelector: ['stale'] } },
    );
    syncObservatories(t);
    expect((t.burstObservatory as { subjectSelector: string[] }).subjectSelector).toEqual(['b']);
  });

  it('removes observatories when no balancer can use them', () => {
    const t = tpl({ balancers: [] }, {
      observatory: { subjectSelector: ['a'] },
      burstObservatory: { subjectSelector: ['a'] },
    });
    syncObservatories(t);
    expect(t.observatory).toBeUndefined();
    expect(t.burstObservatory).toBeUndefined();
  });

  it('switches from stale burstObservatory back to regular observatory when only leastPing remains', () => {
    const t = tpl(
      { balancers: [{ tag: 'lp', selector: ['lp-out'], strategy: { type: 'leastPing' } }] },
      {
        observatory: { subjectSelector: ['stale-observatory-out'] },
        burstObservatory: { subjectSelector: ['stale-burst-out'] },
      },
    );
    syncObservatories(t);
    expectObserver(t, 'observatory', ['lp-out']);
  });

  it('switches from stale observatory to burstObservatory when any burst strategy remains', () => {
    const t = tpl(
      { balancers: [{ tag: 'll', selector: ['ll-out'], strategy: { type: 'leastLoad' } }] },
      {
        observatory: { subjectSelector: ['stale-observatory-out'] },
        burstObservatory: { subjectSelector: ['stale-burst-out'] },
      },
    );
    syncObservatories(t);
    expectObserver(t, 'burstObservatory', ['ll-out']);
  });

  it('creates burstObservatory with the HEAD httpMethod default for leastLoad', () => {
    const t = tpl({ balancers: [{ tag: 'b1', selector: ['a'], strategy: { type: 'leastLoad' } }] });
    syncObservatories(t);
    const burst = t.burstObservatory as { pingConfig: { httpMethod: string; sampling: number } };
    expect(burst.pingConfig.httpMethod).toBe('HEAD');
    expect(burst.pingConfig.sampling).toBe(2);
  });

  it('drops only the prefixes no remaining balancer uses (note #2)', () => {
    const t = tpl({
      balancers: [
        { tag: 'a', selector: ['prefixA', 'prefixB'], strategy: { type: 'leastLoad' } },
        { tag: 'b', selector: ['prefixC', 'prefixB'], strategy: { type: 'leastLoad' } },
      ],
    });
    syncObservatories(t);
    expect(new Set((t.burstObservatory as { subjectSelector: string[] }).subjectSelector)).toEqual(
      new Set(['prefixA', 'prefixB', 'prefixC']),
    );
    (t.routing as { balancers: unknown[] }).balancers.splice(0, 1);
    syncObservatories(t);
    expect(new Set((t.burstObservatory as { subjectSelector: string[] }).subjectSelector)).toEqual(
      new Set(['prefixC', 'prefixB']),
    );
  });
});
