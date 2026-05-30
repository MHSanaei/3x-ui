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
