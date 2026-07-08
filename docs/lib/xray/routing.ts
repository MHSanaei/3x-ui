// Pure builders for an Xray routing block — balancers, routing rules, and the
// observatory/burstObservatory health monitors — matching 3x-ui's schemas
// (frontend schemas/routing.ts, observatory.ts, xray.ts):
//   - balancers live under `routing.balancers[]`; rules under `routing.rules[]`.
//   - `observatory` / `burstObservatory` are TOP-LEVEL (siblings of routing).
//   - leastPing pairs with observatory; leastLoad with burstObservatory.
// No React/DOM imports — unit-tested in Node.

export type Strategy = 'random' | 'roundRobin' | 'leastPing' | 'leastLoad';
export type RuleNetwork = 'tcp' | 'udp' | 'tcp,udp';
export type DomainStrategy = 'AsIs' | 'IPIfNonMatch' | 'IPOnDemand';

export interface BalancerInput {
  tag: string;
  selector: string[];
  strategy: Strategy;
  fallbackTag?: string;
}

export interface RuleTarget {
  kind: 'outbound' | 'balancer';
  tag: string;
}

export interface RuleInput {
  domain?: string[];
  ip?: string[];
  port?: string; // "443", "1000-2000", or "443,8443"
  network?: RuleNetwork;
  protocol?: string[]; // http | tls | quic | bittorrent
  inboundTag?: string[];
  source?: string[]; // -> sourceIP
  target: RuleTarget;
  ruleTag?: string;
}

export interface ObservatoryInput {
  mode: 'observatory' | 'burst';
  subjectSelector: string[];
  probeURL?: string; // observatory
  probeInterval?: string; // observatory
  destination?: string; // burst pingConfig
  interval?: string; // burst pingConfig
}

export interface RoutingInput {
  domainStrategy?: DomainStrategy;
  balancers: BalancerInput[];
  rules: RuleInput[];
  observatory?: ObservatoryInput;
}

const DEFAULT_PROBE_URL = 'https://www.google.com/generate_204';
const DEFAULT_PROBE_INTERVAL = '1m';

export function buildBalancer(b: BalancerInput): Record<string, unknown> {
  const out: Record<string, unknown> = {
    tag: b.tag,
    selector: b.selector,
    strategy: { type: b.strategy },
  };
  if (b.fallbackTag) out.fallbackTag = b.fallbackTag;
  return out;
}

export function buildRule(r: RuleInput): Record<string, unknown> {
  const out: Record<string, unknown> = { type: 'field' };
  if (r.domain?.length) out.domain = r.domain;
  if (r.ip?.length) out.ip = r.ip;
  if (r.port) out.port = r.port;
  if (r.network) out.network = r.network;
  if (r.protocol?.length) out.protocol = r.protocol;
  if (r.inboundTag?.length) out.inboundTag = r.inboundTag;
  if (r.source?.length) out.sourceIP = r.source;
  if (r.ruleTag) out.ruleTag = r.ruleTag;
  if (r.target.kind === 'balancer') out.balancerTag = r.target.tag;
  else out.outboundTag = r.target.tag;
  return out;
}

export function buildObservatory(o: ObservatoryInput): Record<string, unknown> {
  if (o.mode === 'observatory') {
    return {
      observatory: {
        subjectSelector: o.subjectSelector,
        probeURL: o.probeURL || DEFAULT_PROBE_URL,
        probeInterval: o.probeInterval || DEFAULT_PROBE_INTERVAL,
        enableConcurrency: true,
      },
    };
  }
  return {
    burstObservatory: {
      subjectSelector: o.subjectSelector,
      pingConfig: {
        destination: o.destination || DEFAULT_PROBE_URL,
        interval: o.interval || DEFAULT_PROBE_INTERVAL,
        timeout: '5s',
        sampling: 2,
        httpMethod: 'HEAD',
      },
    },
  };
}

function uniqueSelectors(balancers: BalancerInput[]): string[] {
  return [...new Set(balancers.flatMap((b) => b.selector))];
}

export function buildRouting(input: RoutingInput): Record<string, unknown> {
  const routing: Record<string, unknown> = {};
  if (input.domainStrategy) routing.domainStrategy = input.domainStrategy;
  routing.rules = input.rules.map(buildRule);
  routing.balancers = input.balancers.map(buildBalancer);

  const out: Record<string, unknown> = { routing };

  // Latency-aware strategies need a health monitor. Honor an explicit one;
  // otherwise scaffold the matching monitor (observatory for leastPing,
  // burstObservatory for leastLoad) selecting the balancers' own selectors.
  if (input.observatory) {
    Object.assign(out, buildObservatory(input.observatory));
  } else if (input.balancers.some((b) => b.strategy === 'leastLoad')) {
    Object.assign(out, buildObservatory({ mode: 'burst', subjectSelector: uniqueSelectors(input.balancers) }));
  } else if (input.balancers.some((b) => b.strategy === 'leastPing')) {
    Object.assign(
      out,
      buildObservatory({ mode: 'observatory', subjectSelector: uniqueSelectors(input.balancers) }),
    );
  }

  return out;
}

export function buildRoutingJson(input: RoutingInput): string {
  return JSON.stringify(buildRouting(input), null, 2);
}
