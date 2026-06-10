import { describe, expect, it } from 'vitest';

import { syncObservatories } from '@/pages/xray/balancers/balancer-helpers';
import type { XraySettingsValue } from '@/hooks/useXraySetting';

function tpl(routing: Record<string, unknown>, extra: Record<string, unknown> = {}): XraySettingsValue {
  return { routing, ...extra } as unknown as XraySettingsValue;
}

// Observatory sections have no reload API in xray-core, so creating one turns
// a balancer save from a live (hot-applied) routing change into a full
// restart. These tests pin the rule: only strategies that genuinely need an
// observer may create one.
describe('syncObservatories', () => {
  it('does not create burstObservatory for a fresh random balancer (stays hot-appliable)', () => {
    const t = tpl({ balancers: [{ tag: 'b1', selector: ['direct'] }] });
    syncObservatories(t);
    expect(t.burstObservatory).toBeUndefined();
    expect(t.observatory).toBeUndefined();
  });

  it('does not create burstObservatory for roundRobin', () => {
    const t = tpl({ balancers: [{ tag: 'b1', selector: ['a'], strategy: { type: 'roundRobin' } }] });
    syncObservatories(t);
    expect(t.burstObservatory).toBeUndefined();
  });

  it('creates burstObservatory for leastLoad (required by the strategy)', () => {
    const t = tpl({ balancers: [{ tag: 'b1', selector: ['a'], strategy: { type: 'leastLoad' } }] });
    syncObservatories(t);
    expect(t.burstObservatory).toBeDefined();
    expect((t.burstObservatory as { subjectSelector: string[] }).subjectSelector).toEqual(['a']);
  });

  it('creates observatory for leastPing (required by the strategy)', () => {
    const t = tpl({ balancers: [{ tag: 'b1', selector: ['a'], strategy: { type: 'leastPing' } }] });
    syncObservatories(t);
    expect(t.observatory).toBeDefined();
    expect((t.observatory as { subjectSelector: string[] }).subjectSelector).toEqual(['a']);
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
});
