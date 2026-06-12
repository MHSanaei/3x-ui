/**
 * Display label for an inbound: the remark when one is set, otherwise the
 * inbound tag. Falls back to an empty string when neither is present.
 */
export function formatInboundLabel(tag?: string, remark?: string): string {
  const remarkText = (remark || '').trim();
  if (remarkText) return remarkText;
  return (tag || '').trim();
}
