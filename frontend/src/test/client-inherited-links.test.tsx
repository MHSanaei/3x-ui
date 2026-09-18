import { act, fireEvent, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import ClientFormModal from '@/pages/clients/ClientFormModal';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';
import { HttpUtil, Msg } from '@/utils';

import { renderWithProviders } from './test-utils';

const CLIENT = { id: 5, email: 'a@x', enable: true } as unknown as ClientRecord;

const OWN_VALUE = 'vless://own@example.com:443';
const SHARED_VALUE = 'vless://shared@example.com:443';
const OFF_VALUE = 'vless://off@example.com:443';

function view(overrides: Record<string, unknown>): Record<string, unknown> {
  return {
    assignmentId: 1,
    cacheTtl: 0,
    enable: true,
    expiryTime: 0,
    kind: 'link',
    lastFetchAt: 0,
    lastFetchError: '',
    linkId: 1,
    namePrefix: '',
    own: false,
    remark: '',
    scope: 'client',
    scopeTarget: 0,
    userAgent: '',
    value: SHARED_VALUE,
    ...overrides,
  };
}

function mockClientLinks(rows: unknown[]) {
  const gets: string[] = [];
  vi.spyOn(HttpUtil, 'get').mockImplementation(async (url: string) => {
    gets.push(url);
    if (url.includes('/panel/api/links/client/')) return new Msg(true, '', rows);
    return new Msg(true, '', url.includes('/panel/api/inbounds/options') ? [] : {});
  });
  vi.spyOn(HttpUtil, 'post').mockImplementation(async () => new Msg(true, '', {}));
  return gets;
}

afterEach(() => {
  vi.restoreAllMocks();
});

function renderModal(client: ClientRecord | null = CLIENT) {
  return renderWithProviders(
    <MemoryRouter initialEntries={['/clients']}>
      <ClientFormModal
        open
        mode={client ? 'edit' : 'add'}
        client={client}
        inbounds={[] as InboundOption[]}
        save={vi.fn().mockResolvedValue(null)}
        onOpenChange={() => {}}
      />
    </MemoryRouter>,
  );
}

function openLinksTab() {
  const tab = Array.from(document.querySelectorAll('.ant-tabs-tab')).find(
    (el) => (el.textContent ?? '').trim() === 'Links',
  );
  if (!tab) throw new Error('Links tab not found');
  fireEvent.click(tab);
}

function inheritedBlock(): HTMLElement {
  const heading = Array.from(document.querySelectorAll('h1, h2, h3, h4, h5')).find(
    (el) => (el.textContent ?? '').trim() === 'Inherited links',
  );
  const block = heading?.parentElement;
  if (!block) throw new Error('Inherited links block not rendered');
  return block;
}

function inheritedCards(block: HTMLElement): string[] {
  return Array.from(block.querySelectorAll<HTMLElement>('.external-link-card')).map(
    (el) => el.textContent ?? '',
  );
}

describe('ClientFormModal inherited links', () => {
  it('lists only what the client inherits, naming the scope that granted it', async () => {
    mockClientLinks([
      view({ linkId: 1, value: OWN_VALUE, own: true, scope: 'client' }),
      view({ linkId: 2, value: SHARED_VALUE, scope: 'group', remark: 'paid pool' }),
      view({
        linkId: 3,
        value: OFF_VALUE,
        scope: 'global',
        enable: false,
        lastFetchError: 'HTTP 503',
      }),
    ]);
    renderModal();
    openLinksTab();

    const block = await waitFor(() => inheritedBlock());
    expect(block.textContent).toContain(
      'Granted by the panel library; edit the entry there to change it for every client.',
    );

    const cards = inheritedCards(block);
    expect(cards).toHaveLength(2);

    expect(cards[0]).toContain('Group');
    expect(cards[0]).toContain(SHARED_VALUE);
    expect(cards[0]).toContain('paid pool');

    expect(cards[1]).toContain('Every client');
    expect(cards[1]).toContain(OFF_VALUE);
    expect(cards[1]).toContain('Disabled');

    // A row the client owns is edited here, not inherited from the library.
    expect(block.textContent).not.toContain(OWN_VALUE);
  });

  it('leaves the section out when every link the client holds is its own', async () => {
    const gets = mockClientLinks([view({ linkId: 1, value: OWN_VALUE, own: true })]);
    renderModal();
    openLinksTab();

    await waitFor(() => expect(gets).toContain('/panel/api/links/client/5'));
    // The response is applied by the time the view is asserted, so an absent
    // section means the filter dropped the row, not that the query was pending.
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(screen.queryByText('Inherited links')).toBeNull();
    expect(document.body.textContent).not.toContain(OWN_VALUE);
  });

  it('does not look up the links of a client that has no id yet', async () => {
    const gets = mockClientLinks([]);
    renderModal(null);
    openLinksTab();

    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(gets.filter((url) => url.includes('/panel/api/links/client/'))).toEqual([]);
    expect(screen.queryByText('Inherited links')).toBeNull();
  });
});
