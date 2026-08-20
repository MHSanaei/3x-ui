import type { GeoKind } from '@/generated/types';

const DEFAULT_SITE_FILE = 'geosite.dat';
const DEFAULT_IP_FILE = 'geoip.dat';

const LONG_FORMS: Array<[RegExp, string]> = [
  [/^ext(?:-domain|-site)?:geosite\.dat:/, 'geosite:'],
  [/^ext(?:-ip)?:geoip\.dat:/, 'geoip:'],
];

export function parseTokens(value: string): string[] {
  return value
    .split(',')
    .map((token) => token.trim())
    .filter((token) => token !== '');
}

export function formatTokens(tokens: string[]): string {
  return tokens.join(', ');
}

export function tokenFor(file: string, code: string, kind: GeoKind): string {
  if (kind === 'ip' && file === DEFAULT_IP_FILE) return `geoip:${code}`;
  if (kind === 'site' && file === DEFAULT_SITE_FILE) return `geosite:${code}`;
  return `ext:${file}:${code}`;
}

/**
 * Xray treats category codes case-insensitively and accepts both the
 * `geosite:cn` shorthand and its `ext:geosite.dat:cn` long form, so tokens are
 * compared through this normal form. Only comparison uses it — whatever the
 * user typed is what stays in the rule.
 */
export function canonicalToken(token: string): string {
  const lowered = token.trim().toLowerCase();
  for (const [pattern, shorthand] of LONG_FORMS) {
    if (pattern.test(lowered)) return lowered.replace(pattern, shorthand);
  }
  return lowered;
}

export function selectionFromValue(value: string, known: ReadonlySet<string>): string[] {
  const canonicalKnown = new Set([...known].map(canonicalToken));
  const selection: string[] = [];
  const seen = new Set<string>();
  for (const token of parseTokens(value)) {
    const canonical = canonicalToken(token);
    if (!canonicalKnown.has(canonical) || seen.has(canonical)) continue;
    seen.add(canonical);
    selection.push(token);
  }
  return selection;
}

export function mergeSelection(
  value: string,
  selected: string[],
  known: ReadonlySet<string>,
): string {
  const canonicalKnown = new Set([...known].map(canonicalToken));
  const kept = new Set(
    selected.map((token) => canonicalToken(token)).filter((token) => token !== ''),
  );
  const merged: string[] = [];
  const seen = new Set<string>();
  const append = (token: string) => {
    const canonical = canonicalToken(token);
    if (canonical === '' || seen.has(canonical)) return;
    seen.add(canonical);
    merged.push(token);
  };
  for (const token of parseTokens(value)) {
    const canonical = canonicalToken(token);
    if (canonicalKnown.has(canonical) && !kept.has(canonical)) continue;
    append(token);
  }
  for (const token of selected) {
    append(token.trim());
  }
  return formatTokens(merged);
}
