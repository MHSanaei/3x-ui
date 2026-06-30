import type { XraySettingsValue } from '@/hooks/useXraySetting';
import type { BalancerObject } from '@/schemas/routing';

export const DEFAULT_OBSERVATORY = Object.freeze({
  subjectSelector: [] as string[],
  probeURL: 'https://www.google.com/generate_204',
  probeInterval: '1m',
  enableConcurrency: true,
});

export const DEFAULT_BURST_OBSERVATORY = Object.freeze({
  subjectSelector: [] as string[],
  pingConfig: {
    destination: 'https://www.google.com/generate_204',
    interval: '1m',
    connectivity: 'http://connectivitycheck.platform.hicloud.com/generate_204',
    timeout: '5s',
    sampling: 2,
    httpMethod: 'HEAD',
  },
});

export function collectSelectors(list: BalancerObject[]): string[] {
  const out = new Set<string>();
  list.forEach((b) => (b.selector || []).forEach((s) => s && out.add(s)));
  return [...out];
}

export function balancerRequiresBurstObservatory(b: BalancerObject): boolean {
  const type = b.strategy?.type || 'random';
  return type === 'leastLoad' || ((type === 'random' || type === 'roundRobin') && (b.fallbackTag ?? '').length > 0);
}

export function settingsRequireBurstObservatory(t: XraySettingsValue | null): boolean {
  const balancers = (t?.routing?.balancers || []) as BalancerObject[];
  return balancers.some(balancerRequiresBurstObservatory);
}

// syncObservatories keeps the (burst)observatory sections aligned with the
// balancer strategies that actually require them. Observatories have no runtime
// reload API in xray-core, so creating OR removing one forces a full process
// restart — that's why an observer-less balancer never gets one and stays a
// live, routing-only change applied through the core API.
//
// xray-core binds the Observatory feature to a Random/RoundRobinStrategy only
// when its fallbackTag is set (issue #5605): with a fallbackTag the strategy
// calls RequireFeatures(Observatory) and the core aborts startup with "not all
// dependencies are resolved" if none exists; without a fallbackTag it never even
// consults an observatory. leastLoad needs the burst observer, while leastPing
// can use any extension.Observatory result with Alive/Delay. When a burst
// observer is required, keep all observer-backed balancers on burstObservatory
// to avoid xray-core resolving the earlier regular observatory feature instead.
//
// So each observer lives exactly as long as something requires it, and is
// dropped the moment nothing does — clearing the last fallbackTag (or deleting
// the last leastLoad) removes the burst observer again. A no-fallback
// Random/RoundRobin balancer never expands the observer either, because those
// strategies do not consume observer data.
export function syncObservatories(t: XraySettingsValue) {
  const balancers = (t.routing?.balancers || []) as BalancerObject[];

  const leastPings = balancers.filter((b) => b.strategy?.type === 'leastPing');
  const required = balancers.filter(balancerRequiresBurstObservatory);
  if (required.length > 0) {
    delete t.observatory;
    if (!t.burstObservatory) t.burstObservatory = JSON.parse(JSON.stringify(DEFAULT_BURST_OBSERVATORY));
    (t.burstObservatory as { subjectSelector: string[] }).subjectSelector = collectSelectors([
      ...required,
      ...leastPings,
    ]);
  } else {
    delete t.burstObservatory;
    if (leastPings.length > 0) {
      if (!t.observatory) t.observatory = JSON.parse(JSON.stringify(DEFAULT_OBSERVATORY));
      (t.observatory as { subjectSelector: string[] }).subjectSelector = collectSelectors(leastPings);
    } else {
      delete t.observatory;
    }
  }
}
