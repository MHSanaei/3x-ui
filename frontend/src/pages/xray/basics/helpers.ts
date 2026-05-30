import type { XraySettingsValue } from '@/hooks/useXraySetting';

export function ruleGetter(t: XraySettingsValue | null, outboundTag: string, property: string): string[] {
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

export function ruleSetter(t: XraySettingsValue, outboundTag: string, property: string, data: string[]): void {
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
