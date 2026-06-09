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

export function chipPreview(value?: string): string {
  const parts = csv(value);
  if (parts.length === 0) return '';
  if (parts.length === 1) return parts[0];
  return `${parts[0]} +${parts.length - 1}`;
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

export function formatInboundTagsWithRemarks(
  tags?: string,
  remarkByTag: Record<string, string> = {},
): string | undefined {
  if (!tags) return undefined;
  const formatted = csv(tags).map((tag) => {
    const label = remarkByTag[tag]?.trim();
    if (!label || label === tag) return tag;
    return `${tag} (${label})`;
  });
  return formatted.length > 0 ? formatted.join(',') : undefined;
}

export function inboundTagsDisplayTitle(
  tags?: string,
  remarkByTag: Record<string, string> = {},
): string | undefined {
  return formatInboundTagsWithRemarks(tags, remarkByTag)?.split(',').join(', ');
}

export function inboundTagChipPreview(
  tags?: string,
  remarkByTag: Record<string, string> = {},
): string {
  return chipPreview(formatInboundTagsWithRemarks(tags, remarkByTag));
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
