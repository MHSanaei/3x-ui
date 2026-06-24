/**
 * Display label for an inbound: the remark when one is set, otherwise the
 * inbound tag. Falls back to an empty string when neither is present.
 * When a listener port is available, append it so identical remarks remain
 * distinguishable in client selectors.
 */
export function formatInboundLabel(tag?: string, remark?: string, port?: number): string {
  const remarkText = (remark || '').trim();
  const label = remarkText || (tag || '').trim();
  if (!label) return '';
  if (typeof port === 'number' && port > 0) return `${label} : ${port}`;
  return label;
}
