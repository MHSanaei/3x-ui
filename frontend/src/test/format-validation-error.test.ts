/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';
import { z } from 'zod';
import type { TFunction } from 'i18next';

import { formatInboundIssue, formatInboundValidation } from '@/pages/inbounds/form/formatValidationError';

const templates: Record<string, string> = {
  'pages.inbounds.toasts.invalidClientField': 'Client {client}: {field} — {reason}',
  'pages.inbounds.toasts.invalidField': '{field} — {reason}',
  'pages.inbounds.toasts.moreIssues': '{message}  (+{count} more)',
  clients: 'clients',
};

const t = ((key: string, opts?: Record<string, unknown>) => {
  let out = templates[key] ?? (opts?.defaultValue as string | undefined) ?? key;
  if (opts) {
    for (const [k, v] of Object.entries(opts)) {
      out = out.split(`{${k}}`).join(String(v));
    }
  }
  return out;
}) as unknown as TFunction;

describe('formatInboundValidation', () => {
  it('resolves a real client array index back to the client email', () => {
    const schema = z.object({
      settings: z.object({
        clients: z.array(z.object({ email: z.string(), tgId: z.number() })),
      }),
    });
    const values = {
      settings: {
        clients: [
          { email: 'first@x.com', tgId: 1 },
          { email: 'broken@x.com', tgId: 'oops' },
        ],
      },
    };
    const parsed = schema.safeParse(values);
    expect(parsed.success).toBe(false);
    if (parsed.success) return;
    expect(formatInboundIssue(parsed.error.issues[0], values, t)).toContain('Client "broken@x.com": tgId — ');
  });

  it('falls back to the index when the client has no email', () => {
    const issue = { path: ['settings', 'clients', 7, 'tgId'], message: 'Invalid input' };
    const values = { settings: { clients: [] } };
    expect(formatInboundIssue(issue, values, t)).toBe('Client #7: tgId — Invalid input');
  });

  it('formats non-client paths plainly', () => {
    const issue = { path: ['port'], message: 'Invalid input' };
    expect(formatInboundIssue(issue, {}, t)).toBe('port — Invalid input');
  });

  it('appends a count when several fields fail', () => {
    const issues = [
      { path: ['settings', 'clients', 0, 'tgId'], message: 'Invalid input' },
      { path: ['port'], message: 'Invalid input' },
    ];
    const values = { settings: { clients: [{ email: 'a@x.com' }] } };
    expect(formatInboundValidation(issues, values, t)).toBe('Client "a@x.com": tgId — Invalid input  (+1 more)');
  });
});
