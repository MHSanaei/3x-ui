// Template variables an operator can embed in a Host's Remark. At subscription
// time the backend (internal/sub/remark_vars.go) substitutes each {{TOKEN}}
// per client. This file is the single frontend source of truth for the picker
// UI and the live preview — keep the token list in sync with remark_vars.go.

export type RemarkVarGroup = 'client' | 'traffic' | 'time';

export interface RemarkVar {
  /** Bare token name, e.g. "TRAFFIC_LEFT" (rendered as {{TRAFFIC_LEFT}}). */
  token: string;
  group: RemarkVarGroup;
  /** Example value used only for the form's live preview. */
  sample: string;
}

export const REMARK_VAR_GROUPS: RemarkVarGroup[] = ['client', 'traffic', 'time'];

export const REMARK_VARIABLES: RemarkVar[] = [
  // Client identity
  { token: 'EMAIL', group: 'client', sample: 'john' },
  { token: 'INBOUND', group: 'client', sample: 'Germany' },
  { token: 'HOST', group: 'client', sample: 'CDN' },
  { token: 'ID', group: 'client', sample: '3f2a9c1b-aaaa-bbbb-cccc-1234567890ab' },
  { token: 'SHORT_ID', group: 'client', sample: '3f2a9c1b' },
  { token: 'TELEGRAM_ID', group: 'client', sample: '123456789' },
  { token: 'SUB_ID', group: 'client', sample: 'subABC' },
  { token: 'COMMENT', group: 'client', sample: 'vip' },
  // Traffic
  { token: 'TRAFFIC_USED', group: 'traffic', sample: '8.40GB' },
  { token: 'TRAFFIC_LEFT', group: 'traffic', sample: '41.60GB' },
  { token: 'TRAFFIC_TOTAL', group: 'traffic', sample: '50.00GB' },
  { token: 'TRAFFIC_USED_BYTES', group: 'traffic', sample: '9019431321' },
  { token: 'TRAFFIC_LEFT_BYTES', group: 'traffic', sample: '44667656679' },
  { token: 'TRAFFIC_TOTAL_BYTES', group: 'traffic', sample: '53687091200' },
  { token: 'UP', group: 'traffic', sample: '5.20GB' },
  { token: 'DOWN', group: 'traffic', sample: '3.20GB' },
  // Time / status
  { token: 'STATUS', group: 'time', sample: 'active' },
  { token: 'DAYS_LEFT', group: 'time', sample: '12' },
  { token: 'EXPIRE_DATE', group: 'time', sample: '2026-09-01' },
  { token: 'EXPIRE_UNIX', group: 'time', sample: '1788300000' },
  { token: 'CREATED_UNIX', group: 'time', sample: '1700000000' },
  { token: 'RESET_DAYS', group: 'time', sample: '30' },
];

const SAMPLE_BY_TOKEN: Record<string, string> = Object.fromEntries(
  REMARK_VARIABLES.map((v) => [v.token, v.sample]),
);

const TOKEN_RE = /\{\{([A-Z_]+)\}\}/g;

/** wrapToken("EMAIL") → "{{EMAIL}}". */
export function wrapToken(token: string): string {
  return `{{${token}}}`;
}

/** Whether a remark string uses any {{VAR}} token at all. */
export function hasRemarkTokens(template: string): boolean {
  return template.includes('{{');
}

/**
 * previewRemark renders a template against the sample values, mirroring the
 * backend substitution closely enough for an at-a-glance preview. Unknown
 * tokens collapse to empty, just like the server.
 */
export function previewRemark(template: string): string {
  if (!hasRemarkTokens(template)) return template;
  return template.replace(TOKEN_RE, (_m, tok: string) => SAMPLE_BY_TOKEN[tok] ?? '');
}
