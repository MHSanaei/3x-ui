import type { RuleRow } from './types';

export function arrJoin(v: unknown): string | undefined {
  if (v == null) return undefined;
  if (Array.isArray(v)) return v.join(',');
  return String(v);
}

export function csv(value?: string): string[] {
  if (!value) return [];
  return String(value).split(',').map((s) => s.trim()).filter(Boolean);
}

export function chipPreviewParts(parts: string[]): string {
  if (parts.length === 0) return '';
  if (parts.length === 1) return parts[0];
  return `${parts[0]} +${parts.length - 1}`;
}

export function chipPreview(value?: string): string {
  return chipPreviewParts(csv(value));
}

/** Same lookup as RuleFormModal inbound select: remark first, else tag. */
export function buildRemarkByTag(
  options: Array<{ tag?: string; remark?: string }>,
): Record<string, string> {
  const map: Record<string, string> = {};
  for (const ib of options) {
    if (ib.tag) map[ib.tag] = ib.remark?.trim() || ib.tag;
  }
  return map;
}

/** Format a single inbound tag as `tag (remark)`, or just `tag` when no distinct remark. */
export function formatInboundTag(
  tag: string,
  remarkByTag: Record<string, string> = {},
): string {
  const label = remarkByTag[tag]?.trim();
  if (!label || label === tag) return tag;
  return `${tag} (${label})`;
}

/**
 * Formatted inbound entries — `tag (remark)` when a distinct remark exists, else
 * `tag`. Returns an array (not a joined string) so callers never have to re-split
 * on commas, which a remark may legitimately contain.
 */
export function formatInboundTagList(
  tags?: string,
  remarkByTag: Record<string, string> = {},
): string[] {
  return csv(tags).map((tag) => formatInboundTag(tag, remarkByTag));
}

export function inboundTagsDisplayTitle(
  tags?: string,
  remarkByTag: Record<string, string> = {},
): string | undefined {
  const list = formatInboundTagList(tags, remarkByTag);
  return list.length > 0 ? list.join(', ') : undefined;
}

export function inboundTagChipPreview(
  tags?: string,
  remarkByTag: Record<string, string> = {},
): string {
  return chipPreviewParts(formatInboundTagList(tags, remarkByTag));
}

/** The internal api rule (stats traffic) — its enabled state must stay locked on. */
export function isApiRule(rule: { outboundTag?: string; inboundTag?: string | string[] }): boolean {
  if (rule.outboundTag !== 'api') return false;
  const tags = Array.isArray(rule.inboundTag) ? rule.inboundTag : csv(rule.inboundTag);
  return tags.includes('api');
}

export function ruleCriteriaChips(rule: RuleRow) {
  const chips: { label: string; value?: string }[] = [];
  if (rule.domain) chips.push({ label: 'Domain', value: rule.domain });
  if (rule.ip) chips.push({ label: 'IP', value: rule.ip });
  if (rule.port) chips.push({ label: 'Port', value: rule.port });
  if (rule.sourceIP) chips.push({ label: 'Src IP', value: rule.sourceIP });
  if (rule.sourcePort) chips.push({ label: 'Src Port', value: rule.sourcePort });
  if (rule.network) chips.push({ label: 'L4', value: rule.network });
  if (rule.protocol) chips.push({ label: 'Protocol', value: rule.protocol });
  if (rule.user) chips.push({ label: 'User', value: rule.user });
  if (rule.vlessRoute) chips.push({ label: 'VLESS', value: rule.vlessRoute });
  return chips;
}
