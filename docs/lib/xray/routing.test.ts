import { describe, it, expect } from 'vitest';
import {
  buildBalancer,
  buildRule,
  buildObservatory,
  buildRouting,
  buildRoutingJson,
  type RoutingInput,
} from './routing';

describe('buildBalancer', () => {
  it('emits tag, selector, and strategy.type; omits fallbackTag when empty', () => {
    const b = buildBalancer({ tag: 'lb', selector: ['proxy', 'hk-'], strategy: 'roundRobin' });
    expect(b.tag).toBe('lb');
    expect(b.selector).toEqual(['proxy', 'hk-']);
    expect((b.strategy as Record<string, unknown>).type).toBe('roundRobin');
    expect('fallbackTag' in b).toBe(false);
  });

  it('includes fallbackTag when set', () => {
    const b = buildBalancer({ tag: 'lb', selector: ['a'], strategy: 'random', fallbackTag: 'direct' });
    expect(b.fallbackTag).toBe('direct');
  });
});

describe('buildRule', () => {
  it('emits only the matchers that are set, with outboundTag', () => {
    const r = buildRule({ domain: ['geosite:google'], target: { kind: 'outbound', tag: 'warp' } });
    expect(r.type).toBe('field');
    expect(r.domain).toEqual(['geosite:google']);
    expect('ip' in r).toBe(false);
    expect(r.outboundTag).toBe('warp');
    expect('balancerTag' in r).toBe(false);
  });

  it('uses balancerTag (and not outboundTag) when the target is a balancer', () => {
    const r = buildRule({ ip: ['geoip:cn'], target: { kind: 'balancer', tag: 'lb' } });
    expect(r.balancerTag).toBe('lb');
    expect('outboundTag' in r).toBe(false);
    expect(r.ip).toEqual(['geoip:cn']);
  });

  it('passes port + network through and carries ruleTag', () => {
    const r = buildRule({
      port: '443,8443',
      network: 'tcp,udp',
      ruleTag: 'r1',
      target: { kind: 'outbound', tag: 'direct' },
    });
    expect(r.port).toBe('443,8443');
    expect(r.network).toBe('tcp,udp');
    expect(r.ruleTag).toBe('r1');
  });
});

describe('buildObservatory', () => {
  it('observatory mode emits probeURL default + enableConcurrency, no burst', () => {
    const o = buildObservatory({ mode: 'observatory', subjectSelector: ['proxy'] });
    const obs = (o as Record<string, Record<string, unknown>>).observatory;
    expect(obs.probeURL).toBe('https://www.google.com/generate_204');
    expect(obs.subjectSelector).toEqual(['proxy']);
    expect(obs.enableConcurrency).toBe(true);
    expect('burstObservatory' in o).toBe(false);
  });

  it('burst mode emits pingConfig.destination default and no observatory', () => {
    const o = buildObservatory({ mode: 'burst', subjectSelector: ['proxy'] });
    const burst = (o as Record<string, Record<string, Record<string, unknown>>>).burstObservatory;
    expect(burst.pingConfig.destination).toBe('https://www.google.com/generate_204');
    expect('observatory' in o).toBe(false);
  });
});

describe('buildRouting', () => {
  const base: RoutingInput = {
    balancers: [{ tag: 'lb', selector: ['proxy'], strategy: 'random' }],
    rules: [{ domain: ['geosite:category-ads-all'], target: { kind: 'outbound', tag: 'block' } }],
  };

  it('nests rules and balancers under routing', () => {
    const out = buildRouting(base) as { routing: { rules: unknown[]; balancers: unknown[] } };
    expect(out.routing.rules).toHaveLength(1);
    expect(out.routing.balancers).toHaveLength(1);
  });

  it('puts observatory at the TOP level (not under routing) for a leastPing balancer', () => {
    const out = buildRouting({
      ...base,
      balancers: [{ tag: 'lb', selector: ['proxy'], strategy: 'leastPing' }],
    }) as Record<string, Record<string, unknown>>;
    expect(out.observatory).toBeDefined();
    expect('observatory' in out.routing).toBe(false);
  });

  it('uses a burstObservatory for a leastLoad balancer', () => {
    const out = buildRouting({
      ...base,
      balancers: [{ tag: 'lb', selector: ['proxy'], strategy: 'leastLoad' }],
    }) as Record<string, unknown>;
    expect(out.burstObservatory).toBeDefined();
  });

  it('carries domainStrategy when set', () => {
    const out = buildRouting({ ...base, domainStrategy: 'IPIfNonMatch' }) as {
      routing: Record<string, unknown>;
    };
    expect(out.routing.domainStrategy).toBe('IPIfNonMatch');
  });

  it('does not auto-add an observatory for random/roundRobin balancers', () => {
    const out = buildRouting(base) as Record<string, unknown>;
    expect('observatory' in out).toBe(false);
    expect('burstObservatory' in out).toBe(false);
  });
});

describe('buildRoutingJson', () => {
  it('round-trips and is 2-space indented', () => {
    const input: RoutingInput = {
      balancers: [],
      rules: [{ ip: ['geoip:private'], target: { kind: 'outbound', tag: 'direct' } }],
    };
    const json = buildRoutingJson(input);
    expect(json).toContain('\n  "');
    expect(JSON.parse(json)).toEqual(buildRouting(input));
  });
});
