export const EXIT_POOL_TAG_PREFIX = 'fanout-';

type RoutingRule = Record<string, unknown>;

interface ExitPoolTemplate {
  routing?: {
    rules?: RoutingRule[];
    [key: string]: unknown;
  };
  [key: string]: unknown;
}

export interface ExitPoolBinding {
  inboundTag: string;
  outboundTag: string;
}

function inboundTags(value: unknown): string[] {
  if (typeof value === 'string') return [value];
  if (!Array.isArray(value)) return [];
  return value.filter((tag): tag is string => typeof tag === 'string' && tag.length > 0);
}

function isExitPoolRule(rule: RoutingRule): boolean {
  return typeof rule.outboundTag === 'string' && rule.outboundTag.startsWith(EXIT_POOL_TAG_PREFIX);
}

function cloneTemplate<T extends ExitPoolTemplate>(template: T): T {
  return JSON.parse(JSON.stringify(template)) as T;
}

export function listExitPoolBindings(template: ExitPoolTemplate): ExitPoolBinding[] {
  const bindings: ExitPoolBinding[] = [];
  for (const rule of template.routing?.rules || []) {
    if (!isExitPoolRule(rule)) continue;
    const outboundTag = rule.outboundTag as string;
    for (const inboundTag of inboundTags(rule.inboundTag)) {
      bindings.push({ inboundTag, outboundTag });
    }
  }
  return bindings;
}

export function unbindExitPoolRoute<T extends ExitPoolTemplate>(
  template: T,
  inboundTag: string,
): T {
  const next = cloneTemplate(template);
  const rules = next.routing?.rules || [];
  const cleaned: RoutingRule[] = [];

  for (const rule of rules) {
    if (!isExitPoolRule(rule)) {
      cleaned.push(rule);
      continue;
    }
    const remaining = inboundTags(rule.inboundTag).filter((tag) => tag !== inboundTag);
    if (remaining.length > 0) cleaned.push({ ...rule, inboundTag: remaining });
  }

  next.routing = { ...(next.routing || {}), rules: cleaned };
  return next;
}

export function bindExitPoolRoute<T extends ExitPoolTemplate>(
  template: T,
  inboundTag: string,
  outboundTag: string,
): T {
  if (!inboundTag) throw new Error('An inbound tag is required');
  if (!outboundTag.startsWith(EXIT_POOL_TAG_PREFIX)) {
    throw new Error('Only fanout-* outbounds can be managed by the exit pool');
  }

  const next = unbindExitPoolRoute(template, inboundTag);
  const rules = next.routing?.rules || [];
  rules.unshift({
    type: 'field',
    // Keep every transport on the selected exit.  TCP-only rules allow UDP
    // DNS, QUIC and WebRTC/STUN to fall through to a later direct rule.
    network: 'tcp,udp',
    inboundTag: [inboundTag],
    outboundTag,
  });
  next.routing = { ...(next.routing || {}), rules };
  return next;
}
