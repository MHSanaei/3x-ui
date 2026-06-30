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
// consults an observatory. leastLoad always needs the burst observer, leastPing
// the regular one.
//
// So each observer lives exactly as long as something requires it, and is
// dropped the moment nothing does — clearing the last fallbackTag (or deleting
// the last leastLoad) removes the burst observer again. A no-fallback balancer's
// selector is still probed while the observer exists for another reason, but
// never keeps it alive on its own.
export function syncObservatories(t: XraySettingsValue) {
  const balancers = (t.routing?.balancers || []) as BalancerObject[];

  const leastPings = balancers.filter((b) => b.strategy?.type === 'leastPing');
  if (leastPings.length > 0) {
    if (!t.observatory) t.observatory = JSON.parse(JSON.stringify(DEFAULT_OBSERVATORY));
    (t.observatory as { subjectSelector: string[] }).subjectSelector = collectSelectors(leastPings);
  } else {
    delete t.observatory;
  }

  const hasFallback = (b: BalancerObject) => (b.fallbackTag ?? '').length > 0;
  const required = balancers.filter((b) => {
    const type = b.strategy?.type || 'random';
    if (type === 'leastLoad') return true;
    return (type === 'random' || type === 'roundRobin') && hasFallback(b);
  });
  const optional = balancers.filter((b) => {
    const type = b.strategy?.type || 'random';
    return (type === 'random' || type === 'roundRobin') && !hasFallback(b);
  });
  if (required.length > 0) {
    if (!t.burstObservatory) t.burstObservatory = JSON.parse(JSON.stringify(DEFAULT_BURST_OBSERVATORY));
    (t.burstObservatory as { subjectSelector: string[] }).subjectSelector = collectSelectors([...required, ...optional]);
  } else {
    delete t.burstObservatory;
  }
}
