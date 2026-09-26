import { describe, expect, it } from 'vitest';

import {
  toAssignRequest,
  toLinkSaveRequest,
  type LinkAssignValues,
  type LinkSaveValues,
} from '@/schemas/api/link';
import { externalLinkScopeLabel } from '@/lib/clients/external-link';

function values(overrides: Partial<LinkAssignValues> = {}): LinkAssignValues {
  return { scope: 'client', emails: [], group: '', inboundId: 0, ...overrides };
}

function saveValues(overrides: Partial<LinkSaveValues> = {}): LinkSaveValues {
  return {
    kind: 'link',
    value: 'vless://uuid@example.com:443',
    remark: '',
    namePrefix: '',
    enable: true,
    expiryTime: 0,
    userAgent: '',
    cacheTtl: 0,
    ...overrides,
  };
}

describe('toAssignRequest', () => {
  it('sends only the flag the named scope owns', () => {
    expect(
      toAssignRequest(values({ scope: 'group', group: 'paid', emails: ['a@x'], inboundId: 9 })),
    ).toEqual({ emails: [], group: 'paid', inboundId: 0, global: false, newClients: false });

    expect(toAssignRequest(values({ scope: 'global', emails: ['a@x'] }))).toEqual({
      emails: [],
      group: '',
      inboundId: 0,
      global: true,
      newClients: false,
    });

    expect(toAssignRequest(values({ scope: 'new_clients' }))).toEqual({
      emails: [],
      group: '',
      inboundId: 0,
      global: false,
      newClients: true,
    });
  });

  it('keeps the client and inbound payloads intact', () => {
    expect(toAssignRequest(values({ scope: 'client', emails: ['a@x', 'b@x'] }))).toEqual({
      emails: ['a@x', 'b@x'],
      group: '',
      inboundId: 0,
      global: false,
      newClients: false,
    });

    expect(toAssignRequest(values({ scope: 'inbound', inboundId: 12 }))).toEqual({
      emails: [],
      group: '',
      inboundId: 12,
      global: false,
      newClients: false,
    });
  });

  it('maps every scope the API knows to an i18n key', () => {
    for (const scope of ['client', 'group', 'inbound', 'global', 'new_clients'] as const) {
      expect(externalLinkScopeLabel(scope)).toMatch(/^pages\.links\.scope/);
    }
  });
});

describe('toLinkSaveRequest', () => {
  it('leaves a blank fetch field out rather than sending a change the endpoint drops', () => {
    const payload = toLinkSaveRequest(
      saveValues({ kind: 'subscription', userAgent: '', cacheTtl: 0 }),
    );

    expect(payload).not.toHaveProperty('userAgent');
    expect(payload).not.toHaveProperty('cacheTtl');
  });

  it('carries the fetch fields when the operator set them', () => {
    const payload = toLinkSaveRequest(
      saveValues({ kind: 'subscription', userAgent: 'clash-verge/1.6', cacheTtl: 600 }),
    );

    expect(payload.userAgent).toBe('clash-verge/1.6');
    expect(payload.cacheTtl).toBe(600);
  });

  it('keeps the row identity and an explicit disable', () => {
    const payload = toLinkSaveRequest(saveValues({ id: 7, enable: false }));

    expect(payload).toMatchObject({ id: 7, enable: false, kind: 'link' });
  });
});
