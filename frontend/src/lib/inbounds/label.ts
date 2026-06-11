/**
 * Display label for an inbound: `tag (remark)` when a distinct remark exists,
 * otherwise just the tag. Falls back to the remark when no tag is set, and to an
 * empty string when neither is present.
 */
export function formatInboundLabel(tag?: string, remark?: string): string {
  const tagText = (tag || '').trim();
  const remarkText = (remark || '').trim();
  if (!tagText) return remarkText;
  if (!remarkText || remarkText === tagText) return tagText;
  return `${tagText} (${remarkText})`;
}
