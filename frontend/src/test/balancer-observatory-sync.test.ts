import { describe, expect, it } from 'vitest';

import { syncObservatories } from '@/pages/xray/balancers/balancer-helpers';
import type { XraySettingsValue } from '@/hooks/useXraySetting';

function tpl(routing: Record<string, unknown>, extra: Record<string, unknown> = {}): XraySettingsValue {
  return { routing, ...extra } as unknown as XraySettingsValue;
}

// Observatory sections have no reload API in xray-core, so creating one turns
// a balancer save from a live (hot-applied) routing change into a full
// restart. These tests pin the rule: only strategies that genuinely need an
// observer may create one — which, for random/roundRobin, means a fallbackTag
// is set (xray-core then requires the Observatory feature; see #5605).
describe('syncObservatories', () => {
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

  it('keeps burstObservatory while another fallback balancer still needs it', () => {
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
    expect((t.burstObservatory as { subjectSelector: string[] }).subjectSelector).toEqual(['b', 'a']);
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

  it('keeps an existing burstObservatory in sync for random balancers (legacy setups)', () => {
    const t = tpl(
      { balancers: [{ tag: 'b1', selector: ['a'] }, { tag: 'b2', selector: ['b'], strategy: { type: 'leastLoad' } }] },
      { burstObservatory: { subjectSelector: ['stale'] } },
    );
    syncObservatories(t);
    expect((t.burstObservatory as { subjectSelector: string[] }).subjectSelector).toEqual(['b', 'a']);
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
