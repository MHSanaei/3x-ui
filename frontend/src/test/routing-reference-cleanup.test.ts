import { describe, expect, it } from 'vitest';

import {
  applyBalancerDeletion,
  applyOutboundDeletion,
  planBalancerDeletion,
  planOutboundDeletion,
} from '@/pages/xray/reference-cleanup';
import type { XraySettingsValue } from '@/hooks/useXraySetting';

function tpl(parts: Record<string, unknown>): XraySettingsValue {
  return parts as unknown as XraySettingsValue;
}

function dialerProxyOf(tt: XraySettingsValue, tag: string): string | undefined {
  const o = tt.outbounds?.find((x) => x?.tag === tag);
  return (o as { streamSettings?: { sockopt?: { dialerProxy?: string } } } | undefined)
    ?.streamSettings?.sockopt?.dialerProxy;
}

describe('outbound deletion', () => {
  it('drops a rule whose only destination was the deleted outbound', () => {
    const tt = tpl({
      outbounds: [{ tag: 'proxy-us' }],
      routing: { rules: [{ type: 'field', inboundTag: ['in-443'], outboundTag: 'proxy-us' }], balancers: [] },
    });
    applyOutboundDeletion(tt, 0);
    expect(tt.routing!.rules).toEqual([]);
    expect(tt.outbounds).toEqual([]);
  });

  it('keeps a rule that still has a balancer, dropping only the dead outboundTag', () => {
    const tt = tpl({
      outbounds: [{ tag: 'proxy-us' }],
      routing: {
        rules: [{ type: 'field', outboundTag: 'proxy-us', balancerTag: 'eu-pool' }],
        balancers: [{ tag: 'eu-pool', selector: ['direct'] }],
      },
    });
    applyOutboundDeletion(tt, 0);
    expect(tt.routing!.rules).toHaveLength(1);
    expect(tt.routing!.rules![0].outboundTag).toBeUndefined();
    expect(tt.routing!.rules![0].balancerTag).toBe('eu-pool');
  });

  it('reduces a multi-target selector and leaves the balancer and its rules intact', () => {
    const tt = tpl({
      outbounds: [{ tag: 'proxy-us' }, { tag: 'proxy-uk' }],
      routing: {
        rules: [{ type: 'field', inboundTag: ['in'], balancerTag: 'pool' }],
        balancers: [{ tag: 'pool', selector: ['proxy-us', 'proxy-uk'] }],
      },
    });
    applyOutboundDeletion(tt, 0);
    expect(tt.routing!.balancers![0].selector).toEqual(['proxy-uk']);
    expect(tt.routing!.rules).toHaveLength(1);
    expect(tt.routing!.rules![0].balancerTag).toBe('pool');
    expect((tt.outbounds || []).map((o) => o?.tag)).toEqual(['proxy-uk']);
  });

  it('cascade-removes a balancer whose selector is emptied, repairing its rules', () => {
    const tt = tpl({
      outbounds: [{ tag: 'proxy-us' }],
      routing: {
        rules: [
          { type: 'field', inboundTag: ['in'], balancerTag: 'pool' },
          { type: 'field', outboundTag: 'direct', balancerTag: 'pool' },
        ],
        balancers: [{ tag: 'pool', selector: ['proxy-us'] }],
      },
    });
    const impact = planOutboundDeletion(tt, 0);
    expect(impact.balancers).toEqual([{ tag: 'pool', reason: 'selectorEmptied' }]);
    expect(impact.rules).toEqual([
      { index: 0, label: '#1', fate: 'removed' },
      { index: 1, label: '#2', fate: 'modified', keeps: 'direct' },
    ]);

    applyOutboundDeletion(tt, 0);
    expect(tt.routing!.balancers).toEqual([]);
    expect(tt.routing!.rules).toHaveLength(1);
    expect(tt.routing!.rules![0].outboundTag).toBe('direct');
    expect(tt.routing!.rules![0].balancerTag).toBeUndefined();
  });

  it('cascade-removes the burst observer when deleting an outbound removes the last leastLoad balancer', () => {
    const tt = tpl({
      outbounds: [{ tag: 'll-out' }],
      routing: {
        rules: [],
        balancers: [{ tag: 'll', selector: ['ll-out'], strategy: { type: 'leastLoad' } }],
      },
      burstObservatory: { subjectSelector: ['ll-out'] },
    });
    const impact = planOutboundDeletion(tt, 0);
    expect(impact.balancers).toEqual([{ tag: 'll', reason: 'selectorEmptied' }]);
    expect(impact.burst).toBe(true);
    applyOutboundDeletion(tt, 0);
    expect(tt.burstObservatory).toBeUndefined();
    expect(tt.routing!.balancers).toEqual([]);
  });

  it('cascade-switches from burst to regular observer when only leastPing remains', () => {
    const tt = tpl({
      outbounds: [{ tag: 'lp-out' }, { tag: 'll-out' }],
      routing: {
        rules: [],
        balancers: [
          { tag: 'lp', selector: ['lp-out'], strategy: { type: 'leastPing' } },
          { tag: 'll', selector: ['ll-out'], strategy: { type: 'leastLoad' } },
        ],
      },
      burstObservatory: { subjectSelector: ['lp-out', 'll-out'] },
    });
    const impact = planOutboundDeletion(tt, 1);
    expect(impact.balancers).toEqual([{ tag: 'll', reason: 'selectorEmptied' }]);
    expect(impact.burst).toBe(true);
    applyOutboundDeletion(tt, 1);
    expect(tt.burstObservatory).toBeUndefined();
    expect((tt.observatory as { subjectSelector: string[] }).subjectSelector).toEqual(['lp-out']);
    expect(tt.routing!.balancers).toEqual([{ tag: 'lp', selector: ['lp-out'], strategy: { type: 'leastPing' } }]);
  });

  it('cascade-keeps burst observer when leastPing is removed but leastLoad remains', () => {
    const tt = tpl({
      outbounds: [{ tag: 'lp-out' }, { tag: 'll-out' }],
      routing: {
        rules: [],
        balancers: [
          { tag: 'lp', selector: ['lp-out'], strategy: { type: 'leastPing' } },
          { tag: 'll', selector: ['ll-out'], strategy: { type: 'leastLoad' } },
        ],
      },
      burstObservatory: { subjectSelector: ['lp-out', 'll-out'] },
    });
    const impact = planOutboundDeletion(tt, 0);
    expect(impact.balancers).toEqual([{ tag: 'lp', reason: 'selectorEmptied' }]);
    expect(impact.burst).toBe(false);
    applyOutboundDeletion(tt, 0);
    expect(tt.observatory).toBeUndefined();
    expect((tt.burstObservatory as { subjectSelector: string[] }).subjectSelector).toEqual(['ll-out']);
    expect(tt.routing!.balancers).toEqual([{ tag: 'll', selector: ['ll-out'], strategy: { type: 'leastLoad' } }]);
  });

  it('clears a fallbackTag and a dialerProxy pointing at the deleted outbound', () => {
    const tt = tpl({
      outbounds: [
        { tag: 'proxy-us' },
        { tag: 'chain', streamSettings: { sockopt: { dialerProxy: 'proxy-us' } } },
      ],
      routing: {
        rules: [],
        balancers: [{ tag: 'pool', selector: ['proxy-us', 'proxy-uk'], fallbackTag: 'proxy-us' }],
      },
    });
    applyOutboundDeletion(tt, 0);
    expect(tt.routing!.balancers![0].selector).toEqual(['proxy-uk']);
    expect(tt.routing!.balancers![0].fallbackTag).toBe('');
    expect(dialerProxyOf(tt, 'chain')).toBeUndefined();
  });

  it('never cascade-removes a tagless balancer (an empty tag must not match others)', () => {
    const tt = tpl({
      outbounds: [{ tag: 'proxy-us' }],
      routing: {
        rules: [],
        balancers: [
          { tag: '', selector: ['proxy-us'] },
          { tag: '', selector: ['keep-me'] },
        ],
      },
    });
    applyOutboundDeletion(tt, 0);
    expect(tt.routing!.balancers).toHaveLength(2);
  });

  it('does not throw on null entries in rules/balancers/outbounds (unvalidated config)', () => {
    const tt = tpl({
      outbounds: [{ tag: 'proxy-us' }, null],
      routing: {
        rules: [null, { type: 'field', inboundTag: ['in'], outboundTag: 'proxy-us' }],
        balancers: [null, { tag: 'pool', selector: ['keep'] }],
      },
    });
    expect(() => planOutboundDeletion(tt, 0)).not.toThrow();
    expect(() => applyOutboundDeletion(tt, 0)).not.toThrow();
    expect(tt.routing!.balancers).toEqual([{ tag: 'pool', selector: ['keep'] }]);
  });

  it('drops a rule that loses BOTH destinations in one cascade', () => {
    const tt = tpl({
      outbounds: [{ tag: 'proxy-us' }],
      routing: {
        rules: [{ type: 'field', outboundTag: 'proxy-us', balancerTag: 'pool' }],
        balancers: [{ tag: 'pool', selector: ['proxy-us'] }],
      },
    });
    applyOutboundDeletion(tt, 0);
    expect(tt.routing!.rules).toEqual([]);
    expect(tt.routing!.balancers).toEqual([]);
  });

  it('cleans a disabled rule too', () => {
    const tt = tpl({
      outbounds: [{ tag: 'proxy-us' }],
      routing: {
        rules: [{ type: 'field', enabled: false, inboundTag: ['in'], outboundTag: 'proxy-us' }],
        balancers: [],
      },
    });
    applyOutboundDeletion(tt, 0);
    expect(tt.routing!.rules).toEqual([]);
  });

  it('leaves unrelated rules and outbounds untouched', () => {
    const tt = tpl({
      outbounds: [{ tag: 'proxy-us' }, { tag: 'direct' }],
      routing: {
        rules: [
          { type: 'field', inboundTag: ['in'], outboundTag: 'proxy-us' },
          { type: 'field', inboundTag: ['in2'], outboundTag: 'direct' },
        ],
        balancers: [],
      },
    });
    applyOutboundDeletion(tt, 0);
    expect(tt.routing!.rules).toHaveLength(1);
    expect(tt.routing!.rules![0].outboundTag).toBe('direct');
    expect((tt.outbounds || []).map((o) => o?.tag)).toEqual(['direct']);
  });

  it('removes a referenced outbound with no rules and reports an empty impact', () => {
    const tt = tpl({ outbounds: [{ tag: 'lonely' }], routing: { rules: [], balancers: [] } });
    expect(planOutboundDeletion(tt, 0)).toEqual({ rules: [], balancers: [], observatory: false, burst: false });
    applyOutboundDeletion(tt, 0);
    expect(tt.outbounds).toEqual([]);
  });

  it('uses ruleTag as the impact label when present', () => {
    const tt = tpl({
      outbounds: [{ tag: 'x' }],
      routing: { rules: [{ type: 'field', ruleTag: 'block-ads', outboundTag: 'x' }], balancers: [] },
    });
    expect(planOutboundDeletion(tt, 0).rules[0].label).toBe('block-ads');
  });

  it('does not mutate the template when only planning', () => {
    const tt = tpl({
      outbounds: [{ tag: 'proxy-us' }],
      routing: {
        rules: [{ type: 'field', outboundTag: 'proxy-us', balancerTag: 'pool' }],
        balancers: [{ tag: 'pool', selector: ['proxy-us'] }],
      },
      burstObservatory: { subjectSelector: ['proxy-us'] },
    });
    const before = JSON.stringify(tt);
    planOutboundDeletion(tt, 0);
    expect(JSON.stringify(tt)).toBe(before);
  });

  it('predicts the surviving rule count exactly (plan/apply parity)', () => {
    const make = () =>
      tpl({
        outbounds: [{ tag: 'proxy-us' }],
        routing: {
          rules: [
            { type: 'field', inboundTag: ['a'], outboundTag: 'proxy-us' },
            { type: 'field', outboundTag: 'proxy-us', balancerTag: 'pool' },
            { type: 'field', inboundTag: ['b'], outboundTag: 'direct' },
            { type: 'field', inboundTag: ['c'], balancerTag: 'pool' },
          ],
          balancers: [{ tag: 'pool', selector: ['proxy-us'] }],
        },
      });
    const planned = make();
    const applied = make();
    const total = planned.routing!.rules!.length;
    const removed = planOutboundDeletion(planned, 0).rules.filter((r) => r.fate === 'removed').length;
    applyOutboundDeletion(applied, 0);
    expect(applied.routing!.rules!.length).toBe(total - removed);
  });
});

describe('balancer deletion', () => {
  it('drops a rule whose only destination was the deleted balancer', () => {
    const tt = tpl({
      routing: { rules: [{ type: 'field', inboundTag: ['in'], balancerTag: 'pool' }], balancers: [{ tag: 'pool', selector: ['a'] }] },
    });
    applyBalancerDeletion(tt, 0);
    expect(tt.routing!.balancers).toEqual([]);
    expect(tt.routing!.rules).toEqual([]);
  });

  it('keeps a rule that still has an outbound, dropping only the dead balancerTag', () => {
    const tt = tpl({
      routing: { rules: [{ type: 'field', outboundTag: 'direct', balancerTag: 'pool' }], balancers: [{ tag: 'pool', selector: ['a'] }] },
    });
    applyBalancerDeletion(tt, 0);
    expect(tt.routing!.rules).toHaveLength(1);
    expect(tt.routing!.rules![0].balancerTag).toBeUndefined();
    expect(tt.routing!.rules![0].outboundTag).toBe('direct');
  });

  it('reports and removes the observer when deleting the last leastPing balancer', () => {
    const tt = tpl({
      routing: { rules: [], balancers: [{ tag: 'b1', selector: ['a'], strategy: { type: 'leastPing' } }] },
      observatory: { subjectSelector: ['a'] },
    });
    expect(planBalancerDeletion(tt, 0).observatory).toBe(true);
    applyBalancerDeletion(tt, 0);
    expect(tt.observatory).toBeUndefined();
    expect(tt.routing!.balancers).toEqual([]);
  });

  it('reports and removes the burst observer when deleting the last leastLoad balancer', () => {
    const tt = tpl({
      routing: { rules: [], balancers: [{ tag: 'll', selector: ['a'], strategy: { type: 'leastLoad' } }] },
      burstObservatory: { subjectSelector: ['a'] },
    });
    expect(planBalancerDeletion(tt, 0).burst).toBe(true);
    applyBalancerDeletion(tt, 0);
    expect(tt.burstObservatory).toBeUndefined();
    expect(tt.routing!.balancers).toEqual([]);
  });

  it('reports and removes the burst observer when deleting the last fallback balancer', () => {
    const tt = tpl({
      routing: { rules: [], balancers: [{ tag: 'rf', selector: ['a'], fallbackTag: 'direct' }] },
      burstObservatory: { subjectSelector: ['a'] },
    });
    expect(planBalancerDeletion(tt, 0).burst).toBe(true);
    applyBalancerDeletion(tt, 0);
    expect(tt.burstObservatory).toBeUndefined();
    expect(tt.routing!.balancers).toEqual([]);
  });

  it('switches from burst to regular observer when the deleted balancer was the last burst-required one', () => {
    const tt = tpl({
      routing: {
        rules: [],
        balancers: [
          { tag: 'lp', selector: ['lp-out'], strategy: { type: 'leastPing' } },
          { tag: 'll', selector: ['ll-out'], strategy: { type: 'leastLoad' } },
        ],
      },
      burstObservatory: { subjectSelector: ['lp-out', 'll-out'] },
    });
    const impact = planBalancerDeletion(tt, 1);
    expect(impact.burst).toBe(true);
    expect(impact.observatory).toBe(false);
    applyBalancerDeletion(tt, 1);
    expect(tt.burstObservatory).toBeUndefined();
    expect((tt.observatory as { subjectSelector: string[] }).subjectSelector).toEqual(['lp-out']);
    expect(tt.routing!.balancers).toEqual([{ tag: 'lp', selector: ['lp-out'], strategy: { type: 'leastPing' } }]);
  });

  it('keeps burst observer when deleting leastPing but a burst-required balancer remains', () => {
    const tt = tpl({
      routing: {
        rules: [],
        balancers: [
          { tag: 'lp', selector: ['lp-out'], strategy: { type: 'leastPing' } },
          { tag: 'll', selector: ['ll-out'], strategy: { type: 'leastLoad' } },
        ],
      },
      burstObservatory: { subjectSelector: ['lp-out', 'll-out'] },
    });
    const impact = planBalancerDeletion(tt, 0);
    expect(impact.burst).toBe(false);
    expect(impact.observatory).toBe(false);
    applyBalancerDeletion(tt, 0);
    expect(tt.observatory).toBeUndefined();
    expect((tt.burstObservatory as { subjectSelector: string[] }).subjectSelector).toEqual(['ll-out']);
    expect(tt.routing!.balancers).toEqual([{ tag: 'll', selector: ['ll-out'], strategy: { type: 'leastLoad' } }]);
  });

  it('does not report rules when the deleted balancer is unreferenced', () => {
    const tt = tpl({
      routing: { rules: [{ type: 'field', inboundTag: ['in'], outboundTag: 'direct' }], balancers: [{ tag: 'pool', selector: ['a'] }] },
    });
    expect(planBalancerDeletion(tt, 0).rules).toEqual([]);
    applyBalancerDeletion(tt, 0);
    expect(tt.routing!.rules).toHaveLength(1);
  });
});
