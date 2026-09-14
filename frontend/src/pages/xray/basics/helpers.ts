import type { XraySettingsValue } from '@/hooks/useXraySetting';
import { freedomDomainStrategyFromWire } from '@/lib/xray/outbound-form-adapter';
import { blockedSettings, directSettings } from './constants';

// Freedom resolves through the socket layer, so the outbound root and its own
// settings only hold legacy aliases the core warns about (infra/conf/xray.go).
const LEGACY_FREEDOM_STRATEGY_KEYS = ['domainStrategy', 'targetStrategy'] as const;

type Outbound = Record<string, unknown>;

// The core lowercases a protocol id before it resolves the handler, so matching
// it exactly would append a second "direct" the core refuses to load.
export function isDirectFreedomOutbound(o: Outbound | undefined): boolean {
  const protocol = o?.protocol;
  return (
    typeof protocol === 'string' && protocol.toLowerCase() === 'freedom' && o?.tag === 'direct'
  );
}

function directFreedom(t: XraySettingsValue | null): Outbound | undefined {
  return t?.outbounds?.find((o) => isDirectFreedomOutbound(o)) as Outbound | undefined;
}

export function directFreedomStrategy(t: XraySettingsValue | null): string {
  const outbound = directFreedom(t);
  if (!outbound) return 'AsIs';
  return freedomDomainStrategyFromWire(outbound) || 'AsIs';
}

export function setDirectFreedomStrategy(t: XraySettingsValue, next: string): void {
  if (!Array.isArray(t.outbounds)) t.outbounds = [];
  let idx = t.outbounds.findIndex((o) => isDirectFreedomOutbound(o));
  if (idx < 0) {
    t.outbounds.push({ protocol: 'freedom', tag: 'direct', settings: {} } as never);
    idx = t.outbounds.length - 1;
  }
  const ob = t.outbounds[idx] as Outbound;
  // Drop the legacy placements, or the loader keeps warning and the core keeps
  // preferring the root key it resets over the sockopt value set here.
  const settings = (ob.settings ?? {}) as Outbound;
  for (const key of LEGACY_FREEDOM_STRATEGY_KEYS) delete settings[key];
  ob.settings = settings;
  const stream = (ob.streamSettings ?? {}) as Outbound;
  const sockopt = (stream.sockopt ?? {}) as Outbound;
  if (next === 'AsIs') delete sockopt.domainStrategy;
  else sockopt.domainStrategy = next;
  if (Object.keys(sockopt).length === 0) delete stream.sockopt;
  else stream.sockopt = sockopt;
  if (Object.keys(stream).length === 0) delete ob.streamSettings;
  else ob.streamSettings = stream;
}

export function ruleGetter(
  t: XraySettingsValue | null,
  outboundTag: string,
  property: string,
): string[] {
  if (!t?.routing?.rules) return [];
  const out: string[] = [];
  for (const rule of t.routing.rules) {
    if (
      rule &&
      Object.prototype.hasOwnProperty.call(rule, property) &&
      Object.prototype.hasOwnProperty.call(rule, 'outboundTag') &&
      rule.outboundTag === outboundTag
    ) {
      const v = (rule as Record<string, unknown>)[property];
      if (Array.isArray(v)) out.push(...(v as string[]));
    }
  }
  return out;
}

export function ruleSetter(
  t: XraySettingsValue,
  outboundTag: string,
  property: string,
  data: string[],
): void {
  if (!t.routing) return;
  if (!Array.isArray(t.routing.rules)) t.routing.rules = [];
  const current = ruleGetter(t, outboundTag, property);
  if (current.length === 0) {
    t.routing.rules.push({ type: 'field', outboundTag, [property]: data });
    return;
  }
  const next: typeof t.routing.rules = [];
  let inserted = false;
  for (const rule of t.routing.rules) {
    const matches =
      rule &&
      Object.prototype.hasOwnProperty.call(rule, property) &&
      Object.prototype.hasOwnProperty.call(rule, 'outboundTag') &&
      rule.outboundTag === outboundTag;
    if (matches) {
      if (!inserted && data.length > 0) {
        (rule as Record<string, unknown>)[property] = data;
        next.push(rule);
        inserted = true;
      }
    } else {
      next.push(rule);
    }
  }
  t.routing.rules = next;
}

export function syncOutbound(t: XraySettingsValue, tag: string, settings: Record<string, unknown>) {
  if (!t.outbounds || !t.routing) return;
  const rules = t.routing.rules || [];
  const haveRules = rules.some((r) => r?.outboundTag === tag);
  const idx = t.outbounds.findIndex((o) => o?.tag === tag);
  if (!haveRules && idx > 0) t.outbounds.splice(idx, 1);
  if (haveRules && idx < 0) t.outbounds.push(settings as never);
}

export function getDefaultOutboundTag(t: XraySettingsValue | null): string {
  const tag = t?.outbounds?.[0]?.tag;
  return typeof tag === 'string' && tag.length > 0 ? tag : 'direct';
}

export function setDefaultOutboundTag(t: XraySettingsValue, tag: string): void {
  if (!tag) return;
  if (!Array.isArray(t.outbounds)) t.outbounds = [];
  const idx = t.outbounds.findIndex((o) => o?.tag === tag);
  if (idx < 0) {
    if (tag === 'direct') t.outbounds.push(directSettings as never);
    else if (tag === 'blocked') t.outbounds.push(blockedSettings as never);
    else return;
    const newIdx = t.outbounds.length - 1;
    const [moved] = t.outbounds.splice(newIdx, 1);
    t.outbounds.unshift(moved);
  } else if (idx > 0) {
    const [moved] = t.outbounds.splice(idx, 1);
    t.outbounds.unshift(moved);
  }
}

export function propagateOutboundTagRename(
  t: XraySettingsValue,
  oldTag: string,
  newTag: string,
): void {
  if (!oldTag || !newTag || oldTag === newTag) return;

  const rules = t.routing?.rules;
  if (Array.isArray(rules)) {
    for (const rule of rules) {
      if (rule?.outboundTag === oldTag) rule.outboundTag = newTag;
    }
  }

  const balancers = t.routing?.balancers;
  if (Array.isArray(balancers)) {
    for (const balancer of balancers) {
      if (balancer?.fallbackTag === oldTag) balancer.fallbackTag = newTag;
      if (Array.isArray(balancer?.selector)) {
        balancer.selector = balancer.selector.map((sel) => (sel === oldTag ? newTag : sel));
      }
    }
  }

  if (Array.isArray(t.outbounds)) {
    for (const outbound of t.outbounds) {
      const sockopt = (outbound as { streamSettings?: { sockopt?: { dialerProxy?: string } } })
        ?.streamSettings?.sockopt;
      if (sockopt?.dialerProxy === oldTag) sockopt.dialerProxy = newTag;
    }
  }
}
