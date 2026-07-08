import type { XraySettingsValue } from '@/hooks/useXraySetting';

const LOOPBACK_PREFIX = '_bl_';

export function isBalancerLoopbackTag(tag: string): boolean {
  return tag.startsWith(LOOPBACK_PREFIX);
}

export function loopbackTagFor(targetBalancerTag: string): string {
  return LOOPBACK_PREFIX + targetBalancerTag;
}

export function balancerTagFromLoopback(loopbackTag: string): string | null {
  if (!isBalancerLoopbackTag(loopbackTag)) return null;
  return loopbackTag.slice(LOOPBACK_PREFIX.length);
}

function loopbackMatchesTarget(loopbackTag: string, targetTag: string): boolean {
  if (!isBalancerLoopbackTag(loopbackTag)) return false;
  const target = balancerTagFromLoopback(loopbackTag);
  return target === targetTag;
}

function findLoopbackTarget(settings: XraySettingsValue, loopbackTag: string): string | null {
  const rules = (settings.routing?.rules || []) as Array<{ inboundTag?: string[]; balancerTag?: string }>;
  for (const r of rules) {
    if (Array.isArray(r.inboundTag) && r.inboundTag.includes(loopbackTag) && r.balancerTag) {
      return r.balancerTag;
    }
  }
  return null;
}

export function resolveLoopbackFallback(
  settings: XraySettingsValue,
  fallbackTag: string,
): string {
  if (!fallbackTag || !isBalancerLoopbackTag(fallbackTag)) return fallbackTag;
  const target = findLoopbackTarget(settings, fallbackTag);
  if (target) return target;
  const targetTag = balancerTagFromLoopback(fallbackTag);
  return targetTag || fallbackTag;
}

function countLoopbackRefs(settings: XraySettingsValue, targetTag: string): number {
  let count = 0;
  for (const b of (settings.routing?.balancers || []) as Array<{ fallbackTag?: string }>) {
    if (b.fallbackTag && isBalancerLoopbackTag(b.fallbackTag) && loopbackMatchesTarget(b.fallbackTag, targetTag)) {
      count++;
    }
  }
  return count;
}

export function ensureBalancerLoopback(
  settings: XraySettingsValue,
  targetBalancerTag: string,
): void {
  const lbTag = loopbackTagFor(targetBalancerTag);

  if (!Array.isArray(settings.outbounds)) settings.outbounds = [];
  const existingIdx = (settings.outbounds as Array<{ tag?: string; protocol?: string }>).findIndex(
    (o) => o.tag === lbTag,
  );
  const newOutbound = { tag: lbTag, protocol: 'loopback', settings: { inboundTag: lbTag } };
  if (existingIdx >= 0) {
    (settings.outbounds as Record<string, unknown>[])[existingIdx] = newOutbound;
  } else {
    (settings.outbounds as Record<string, unknown>[]).push(newOutbound);
  }

  if (!settings.routing) settings.routing = { rules: [], balancers: [] };
  if (!Array.isArray(settings.routing.rules)) settings.routing.rules = [];

  const existingRuleIdx = (settings.routing.rules as Array<{ inboundTag?: string[] }>).findIndex(
    (r) => Array.isArray(r.inboundTag) && r.inboundTag.includes(lbTag),
  );
  if (existingRuleIdx >= 0) {
    (settings.routing.rules as Record<string, unknown>[])[existingRuleIdx].balancerTag = targetBalancerTag;
  } else {
    (settings.routing.rules as Record<string, unknown>[]).push({
      type: 'field',
      inboundTag: [lbTag],
      balancerTag: targetBalancerTag,
    });
  }
}

export function removeBalancerLoopbackIfOrphaned(
  settings: XraySettingsValue,
  targetBalancerTag: string,
): void {
  if (countLoopbackRefs(settings, targetBalancerTag) > 0) return;
  removeBalancerLoopback(settings, targetBalancerTag);
}

export function removeBalancerLoopback(
  settings: XraySettingsValue,
  targetBalancerTag: string,
): void {
  const lbTag = loopbackTagFor(targetBalancerTag);

  if (Array.isArray(settings.outbounds)) {
    settings.outbounds = (settings.outbounds as Array<{ tag?: string }>).filter(
      (o) => o.tag !== lbTag,
    ) as XraySettingsValue['outbounds'];
  }

  if (settings.routing && Array.isArray(settings.routing.rules)) {
    settings.routing.rules = settings.routing.rules.filter(
      (r) => !(Array.isArray(r.inboundTag) && r.inboundTag.includes(lbTag)),
    );
  }
}

export function propagateBalancerTagRename(
  settings: XraySettingsValue,
  oldTag: string,
  newTag: string,
): void {
  const oldLbTag = loopbackTagFor(oldTag);
  const newLbTag = loopbackTagFor(newTag);

  if (Array.isArray(settings.outbounds)) {
    for (const o of settings.outbounds as Array<{ tag?: string; settings?: { inboundTag?: string } }>) {
      if (o.tag === oldLbTag) o.tag = newLbTag;
      if (o.settings?.inboundTag === oldLbTag) o.settings.inboundTag = newLbTag;
    }
  }

  if (settings.routing && Array.isArray(settings.routing.rules)) {
    for (const r of settings.routing.rules as Array<{ inboundTag?: string[] }>) {
      if (Array.isArray(r.inboundTag)) {
        const idx = r.inboundTag.indexOf(oldLbTag);
        if (idx !== -1) r.inboundTag[idx] = newLbTag;
      }
    }
  }

  if (settings.routing && Array.isArray(settings.routing.balancers)) {
    for (const b of settings.routing.balancers as Array<{ tag?: string; fallbackTag?: string }>) {
      if (b.fallbackTag === oldLbTag) b.fallbackTag = newLbTag;
    }
  }
}

export function detectBalancerCycles(settings: XraySettingsValue): string[][] {
  const balancers = (settings.routing?.balancers || []) as Array<{ tag?: string; fallbackTag?: string }>;
  const cycles: string[][] = [];

  for (const b of balancers) {
    if (!b.tag || !b.fallbackTag || !isBalancerLoopbackTag(b.fallbackTag)) continue;
    const targetTag = balancerTagFromLoopback(b.fallbackTag);
    if (!targetTag) continue;

    const visited = new Set<string>();
    let cursor = targetTag;
    while (cursor && !visited.has(cursor)) {
      if (cursor === b.tag) {
        cycles.push([b.tag, targetTag]);
        break;
      }
      visited.add(cursor);
      const next = balancers.find((x) => x.tag === cursor);
      const fb = next?.fallbackTag;
      if (!fb || !isBalancerLoopbackTag(fb)) break;
      cursor = balancerTagFromLoopback(fb) || '';
    }
  }
  return cycles;
}

export function ensureMissingBalancerLoopbacks(settings: XraySettingsValue): void {
  const balancers = (settings.routing?.balancers || []) as Array<{ tag?: string; fallbackTag?: string }>;
  for (const b of balancers) {
    if (!b.fallbackTag || !isBalancerLoopbackTag(b.fallbackTag)) continue;
    const targetTag = balancerTagFromLoopback(b.fallbackTag);
    if (!targetTag) continue;
    ensureBalancerLoopback(settings, targetTag);
  }
}

export function cleanupOrphanedBalancerLoopbacks(settings: XraySettingsValue): void {
  if (!Array.isArray(settings.outbounds)) return;

  const orphanedTags: string[] = [];
  for (const o of settings.outbounds as Array<{ tag?: string; protocol?: string }>) {
    if (o.protocol !== 'loopback' || !o.tag || !isBalancerLoopbackTag(o.tag)) continue;
    const targetTag = balancerTagFromLoopback(o.tag);
    if (targetTag && countLoopbackRefs(settings, targetTag) === 0) {
      orphanedTags.push(targetTag);
    }
  }

  for (const tag of orphanedTags) {
    removeBalancerLoopback(settings, tag);
  }
}
