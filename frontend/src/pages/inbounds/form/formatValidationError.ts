import type { TFunction } from 'i18next';

type IssueLike = { path: PropertyKey[]; message: string };

interface ClientLike {
  email?: unknown;
}

/**
 * Turns one Zod issue from the inbound-form schema into a human-readable line.
 * The schema validates the whole form at once, so a bad client field surfaces
 * as `settings.clients.<index>.<field>` — useless on its own when an inbound
 * holds hundreds of clients. We resolve that index back to the client's email
 * so the operator can find the offending entry. The reason is translated when
 * it is a custom message key; Zod defaults like "Invalid input" pass through.
 */
export function formatInboundIssue(issue: IssueLike, values: unknown, t: TFunction): string {
  const path = Array.isArray(issue?.path) ? issue.path : [];
  const reason = t(issue?.message, { defaultValue: issue?.message });

  if (path[0] === 'settings' && path[1] === 'clients' && typeof path[2] === 'number') {
    const index = path[2];
    const clients = (values as { settings?: { clients?: ClientLike[] } })?.settings?.clients;
    const client = Array.isArray(clients) ? clients[index] : undefined;
    const email = typeof client?.email === 'string' && client.email !== '' ? client.email : '';
    const who = email ? `"${email}"` : `#${index}`;
    const field = path.slice(3).map(String).join('.') || t('clients');
    return t('pages.inbounds.toasts.invalidClientField', { client: who, field, reason });
  }

  const field = path.map(String).join('.') || 'value';
  return t('pages.inbounds.toasts.invalidField', { field, reason });
}

/**
 * Builds the single-line toast for a failed inbound save: the first issue,
 * fully described, plus a "(+N more)" tail when several fields failed.
 */
export function formatInboundValidation(issues: IssueLike[], values: unknown, t: TFunction): string {
  const first = formatInboundIssue(issues[0], values, t);
  if (issues.length <= 1) return first;
  return t('pages.inbounds.toasts.moreIssues', { message: first, count: issues.length - 1 });
}
