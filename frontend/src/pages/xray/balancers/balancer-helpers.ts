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
  },
});

export function collectSelectors(list: BalancerObject[]): string[] {
  const out = new Set<string>();
  list.forEach((b) => (b.selector || []).forEach((s) => s && out.add(s)));
  return [...out];
}

// syncObservatories keeps the (burst)observatory sections aligned with the
// balancer strategies that actually require them. Observatories have no
// runtime reload API in xray-core, so any change here forces a full process
// restart — that's why random/roundRobin balancers, which work fine without
// an observer, never CREATE one: a plain balancer add/edit then stays a
// routing-only change and applies live through the core API. An already
// existing burstObservatory is still kept in sync for them (alive-only
// filtering keeps working for setups that had it), it's just never the
// reason a new one appears.
export function syncObservatories(t: XraySettingsValue) {
  const balancers = (t.routing?.balancers || []) as BalancerObject[];

  const leastPings = balancers.filter((b) => b.strategy?.type === 'leastPing');
  if (leastPings.length > 0) {
    if (!t.observatory) t.observatory = JSON.parse(JSON.stringify(DEFAULT_OBSERVATORY));
    (t.observatory as { subjectSelector: string[] }).subjectSelector = collectSelectors(leastPings);
  } else {
    delete t.observatory;
  }

  const required = balancers.filter((b) => b.strategy?.type === 'leastLoad');
  const optional = balancers.filter((b) => {
    const type = b.strategy?.type || 'random';
    return type === 'random' || type === 'roundRobin';
  });
  if (required.length > 0 || (optional.length > 0 && t.burstObservatory)) {
    if (!t.burstObservatory) t.burstObservatory = JSON.parse(JSON.stringify(DEFAULT_BURST_OBSERVATORY));
    (t.burstObservatory as { subjectSelector: string[] }).subjectSelector = collectSelectors([...required, ...optional]);
  } else if (required.length === 0 && optional.length === 0) {
    delete t.burstObservatory;
  }
}
