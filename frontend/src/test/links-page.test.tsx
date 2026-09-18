import { fireEvent, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import LinksPage from '@/pages/links/LinksPage';
import { HttpUtil, Msg } from '@/utils';

import { renderWithProviders } from './test-utils';

// The page hosts the sidebar, which would otherwise query the panel settings.
vi.mock('@/api/queries/useAllSettings', () => ({
  useAllSettings: () => ({ allSetting: {} }),
}));

const LINK_VALUE = 'vless://uuid@example.com:443';
const OTHER_VALUE = 'trojan://pass@other.example:8443';
const SUB_VALUE = 'https://provider.example/sub?token=abc';
const FAILED_VALUE = 'https://broken.example/sub';

function link(overrides: Record<string, unknown>): Record<string, unknown> {
  return {
    assignedClients: 0,
    cacheTtl: 0,
    createdAt: 1,
    enable: true,
    expiryTime: 0,
    headers: {},
    kind: 'link',
    lastFetchAt: 0,
    lastFetchError: '',
    namePrefix: '',
    origin: 'panel',
    remark: '',
    sortIndex: 0,
    updatedAt: 1,
    userAgent: '',
    ...overrides,
  };
}

interface PanelState {
  links?: unknown[];
  targets?: unknown[];
  clients?: unknown[];
  groups?: unknown[];
  inbounds?: unknown[];
  listError?: string;
}

function mockPanel(panel: PanelState = {}) {
  const posts: { url: string; payload?: unknown }[] = [];
  const gets: string[] = [];

  vi.spyOn(HttpUtil, 'get').mockImplementation(async (url: string) => {
    gets.push(url);
    if (url.includes('/panel/api/links/list')) {
      return panel.listError
        ? new Msg(false, panel.listError, null)
        : new Msg(true, '', panel.links ?? []);
    }
    if (url.includes('/panel/api/links/targets/')) return new Msg(true, '', panel.targets ?? []);
    if (url.includes('/panel/api/clients/groups')) return new Msg(true, '', panel.groups ?? []);
    if (url.includes('/panel/api/clients/list')) return new Msg(true, '', panel.clients ?? []);
    if (url.includes('/panel/api/inbounds/options')) return new Msg(true, '', panel.inbounds ?? []);
    return new Msg(true, '', null);
  });

  vi.spyOn(HttpUtil, 'post').mockImplementation(async (url: string, payload?: unknown) => {
    posts.push({ url, payload });
    if (url.includes('/links/assign/') || url.includes('/links/unassign/')) {
      return new Msg(true, '', { affected: 2 });
    }
    return new Msg(true, '', {});
  });

  return { posts, gets };
}

afterEach(() => {
  vi.restoreAllMocks();
});

function renderPage() {
  return renderWithProviders(
    <MemoryRouter initialEntries={['/links']}>
      <LinksPage />
    </MemoryRouter>,
  );
}

function rowFor(value: string): HTMLElement {
  const row = Array.from(document.querySelectorAll<HTMLElement>('.ant-table-tbody > tr')).find(
    (tr) => (tr.textContent ?? '').includes(value),
  );
  if (!row) throw new Error(`No table row rendering ${value}`);
  return row;
}

function buttonByText(text: string, root: ParentNode): HTMLButtonElement {
  const button = Array.from(root.querySelectorAll('button.ant-btn')).find(
    (el) => (el.textContent ?? '').trim() === text,
  );
  if (!button) throw new Error(`No button reading ${text}`);
  return button as HTMLButtonElement;
}

function rowButton(value: string, label: string): HTMLButtonElement {
  const button = rowFor(value).querySelector(`button[aria-label="${label}"]`);
  if (!button) throw new Error(`No ${label} button in the row rendering ${value}`);
  return button as HTMLButtonElement;
}

function modalRoot(kind: 'form' | 'confirm' = 'form'): HTMLElement {
  const selector = kind === 'confirm' ? '.ant-modal-confirm' : '.ant-modal:not(.ant-modal-confirm)';
  const root = document.querySelector(selector);
  if (!root) throw new Error(`No ${kind} modal in the document`);
  return root as HTMLElement;
}

function formItemFor(label: string): HTMLElement {
  const labelEl = Array.from(document.querySelectorAll('.ant-form-item-label label')).find(
    (el) => (el.textContent ?? '').trim() === label,
  );
  const item = labelEl?.closest('.ant-form-item') as HTMLElement | null;
  if (!item) throw new Error(`No form item labelled ${label}`);
  return item;
}

function selectInFormItemFor(label: string): HTMLElement {
  const select = formItemFor(label).querySelector('.ant-select') as HTMLElement | null;
  if (!select) throw new Error(`No select in the ${label} form item`);
  return select;
}

function openSelect(select: HTMLElement) {
  fireEvent.mouseDown((select.querySelector('.ant-select-selector') ?? select) as HTMLElement);
}

function clickOption(title: string) {
  const option = Array.from(document.querySelectorAll('.ant-select-item-option')).find(
    (el) => (el.getAttribute('title') ?? el.textContent ?? '').trim() === title,
  );
  if (!option) throw new Error(`No select option ${title}`);
  fireEvent.click(option);
}

function optionTitles(): string[] {
  return Array.from(document.querySelectorAll('.ant-select-item-option')).map((el) =>
    (el.getAttribute('title') ?? el.textContent ?? '').trim(),
  );
}

function chosenIn(select: HTMLElement): string[] {
  const items = Array.from(select.querySelectorAll('.ant-select-selection-item')).map((el) =>
    (el.getAttribute('title') ?? el.textContent ?? '').trim(),
  );
  if (items.length > 0) return items;
  // Only a single select wraps its label in content-has-value; without it the
  // content holds the placeholder, which is not a chosen target.
  if (!select.querySelector('.ant-select-content-has-value')) return [];
  const label = (select.querySelector('.ant-select-content')?.textContent ?? '').trim();
  return label ? [label] : [];
}

function tagFor(text: string): HTMLElement {
  const tag = Array.from(document.querySelectorAll<HTMLElement>('.ant-tag')).find(
    (el) => (el.textContent ?? '').trim() === text,
  );
  if (!tag) throw new Error(`No tag reading ${text}`);
  return tag;
}

describe('LinksPage list', () => {
  it('renders every row with its kind, remark, client count and fetch state', async () => {
    mockPanel({
      links: [
        link({ id: 7, value: LINK_VALUE, remark: 'fast', assignedClients: 3 }),
        link({ id: 9, kind: 'subscription', value: SUB_VALUE }),
        link({
          id: 11,
          kind: 'subscription',
          value: FAILED_VALUE,
          lastFetchAt: 1_700_000_000_000,
          lastFetchError: 'HTTP 500',
        }),
      ],
    });
    renderPage();

    await screen.findByText(LINK_VALUE);

    const linkRow = rowFor(LINK_VALUE);
    expect(linkRow.textContent).toContain('Share link');
    expect(linkRow.textContent).toContain('fast');
    expect(linkRow.textContent).toContain('3');
    // A plain link is never fetched, so the column holds the placeholder.
    expect(linkRow.textContent).toContain('-');
    expect(linkRow.textContent).not.toContain('Never fetched');

    expect(rowFor(SUB_VALUE).textContent).toContain('Subscription');
    expect(rowFor(SUB_VALUE).textContent).toContain('Never fetched');
    // The row without a remark falls back to the placeholder too.
    expect(rowFor(SUB_VALUE).textContent).toContain('-');

    expect(rowFor(FAILED_VALUE).textContent).toContain('Failed: HTTP 500');
  });

  it('shows the empty library hint instead of an empty table', async () => {
    mockPanel({ links: [] });
    renderPage();

    expect(await screen.findByText('The library is empty. Add the first entry.')).toBeTruthy();
  });

  it('reports a failed load instead of pretending the library is empty', async () => {
    mockPanel({ listError: 'links/list response failed validation' });
    renderPage();

    expect(await screen.findByText('Something went wrong')).toBeTruthy();
    expect(screen.getByText('links/list response failed validation')).toBeTruthy();
    expect(screen.queryByText('The library is empty. Add the first entry.')).toBeNull();
  });

  it('asks for the targets of the row that was opened, and of no other', async () => {
    const { gets } = mockPanel({
      links: [link({ id: 7, value: LINK_VALUE }), link({ id: 9, value: OTHER_VALUE })],
    });
    renderPage();

    await screen.findByText(LINK_VALUE);
    expect(gets.filter((url) => url.includes('/links/targets/'))).toEqual([]);

    fireEvent.click(rowButton(OTHER_VALUE, 'Assigned to'));

    await waitFor(() =>
      expect(gets.filter((url) => url.includes('/links/targets/'))).toEqual([
        '/panel/api/links/targets/9',
      ]),
    );
  });
});

describe('LinksPage row actions', () => {
  it('toggles just the row whose own button was clicked', async () => {
    const { posts } = mockPanel({
      links: [
        link({ id: 7, value: LINK_VALUE }),
        link({ id: 9, value: OTHER_VALUE, enable: false }),
      ],
    });
    renderPage();

    await screen.findByText(LINK_VALUE);
    expect(buttonByText('Enabled', rowFor(LINK_VALUE))).toBeTruthy();
    fireEvent.click(buttonByText('Disabled', rowFor(OTHER_VALUE)));

    await waitFor(() => expect(posts).toHaveLength(1));
    expect(posts[0]).toEqual({ url: '/panel/api/links/enable/9', payload: { enable: true } });
  });

  it('posts the reordered ids and refuses to move a row past the ends', async () => {
    const { posts } = mockPanel({
      links: [link({ id: 7, value: LINK_VALUE }), link({ id: 9, value: OTHER_VALUE })],
    });
    renderPage();

    await screen.findByText(LINK_VALUE);
    expect(rowButton(LINK_VALUE, 'Move up').disabled).toBe(true);
    expect(rowButton(OTHER_VALUE, 'Move down').disabled).toBe(true);

    fireEvent.click(rowButton(LINK_VALUE, 'Move down'));

    await waitFor(() => expect(posts).toHaveLength(1));
    expect(posts[0]).toEqual({ url: '/panel/api/links/reorder', payload: { ids: [9, 7] } });
  });

  it('names the row in the delete confirmation and deletes by id', async () => {
    const { posts } = mockPanel({ links: [link({ id: 7, value: LINK_VALUE, remark: 'fast' })] });
    renderPage();

    await screen.findByText(LINK_VALUE);
    fireEvent.click(rowButton(LINK_VALUE, 'Delete'));

    const confirm = await waitFor(() => modalRoot('confirm'));
    expect(confirm.textContent).toContain('Delete "fast"? Every client that inherits it loses it.');
    fireEvent.click(buttonByText('Delete', confirm));

    await waitFor(() => expect(posts).toHaveLength(1));
    expect(posts[0]).toEqual({ url: '/panel/api/links/del/7', payload: undefined });
  });
});

describe('LinksPage form', () => {
  it('prefills the edit form from the row and sends its id back on save', async () => {
    const { posts } = mockPanel({
      links: [
        link({
          id: 7,
          value: LINK_VALUE,
          remark: 'fast',
          namePrefix: 'pool-a',
          kind: 'subscription',
          userAgent: 'clash-verge/1.6',
          cacheTtl: 600,
        }),
      ],
    });
    renderPage();

    await screen.findByText(LINK_VALUE);
    fireEvent.click(rowButton(LINK_VALUE, 'Edit'));

    await waitFor(() => expect(modalRoot()).toBeTruthy());
    expect(screen.getByText('Edit entry')).toBeTruthy();
    expect(screen.getByDisplayValue(LINK_VALUE)).toBeTruthy();
    expect(screen.getByDisplayValue('fast')).toBeTruthy();
    expect(screen.getByDisplayValue('pool-a')).toBeTruthy();
    expect(screen.getByDisplayValue('clash-verge/1.6')).toBeTruthy();
    expect(chosenIn(selectInFormItemFor('Type'))).toEqual(['Subscription']);

    fireEvent.change(screen.getByDisplayValue('fast'), { target: { value: 'edited' } });
    fireEvent.click(buttonByText('Save', modalRoot()));

    await waitFor(() => expect(posts).toHaveLength(1));
    expect(posts[0].url).toBe('/panel/api/links/add');
    expect(posts[0].payload).toMatchObject({
      id: 7,
      remark: 'edited',
      kind: 'subscription',
      userAgent: 'clash-verge/1.6',
      cacheTtl: 600,
    });
  });

  it('opens a blank create form and reveals the fetch fields only for a subscription', async () => {
    const { posts } = mockPanel({ links: [link({ id: 7, value: LINK_VALUE })] });
    renderPage();

    await screen.findByText(LINK_VALUE);
    fireEvent.click(buttonByText('Add entry', document));

    await waitFor(() => expect(modalRoot()).toBeTruthy());
    expect(screen.queryByDisplayValue(LINK_VALUE)).toBeNull();
    expect(chosenIn(selectInFormItemFor('Type'))).toEqual(['Share link']);
    expect(screen.queryByText('User-Agent')).toBeNull();

    fireEvent.click(buttonByText('Save', modalRoot()));
    expect(await screen.findByText('Enter a link or a URL')).toBeTruthy();
    expect(posts).toHaveLength(0);

    openSelect(selectInFormItemFor('Type'));
    clickOption('Subscription');
    expect(await screen.findByText('User-Agent')).toBeTruthy();
    expect(screen.getByText('Cache TTL (seconds)')).toBeTruthy();
  });
});

describe('LinksPage assignment targets', () => {
  it('lists what the row is assigned to', async () => {
    mockPanel({
      links: [link({ id: 7, value: LINK_VALUE })],
      targets: [
        { targetType: 'group', targetId: 0, name: 'paid' },
        { targetType: 'client', targetId: 0, name: 'a@x' },
      ],
    });
    renderPage();

    await screen.findByText(LINK_VALUE);
    fireEvent.click(rowButton(LINK_VALUE, 'Assigned to'));

    expect(await screen.findByText('Group: paid')).toBeTruthy();
    expect(screen.getByText('Client: a@x')).toBeTruthy();
    expect(screen.queryByText('Assigned to nothing yet.')).toBeNull();
  });

  it('says so when the row is assigned to nothing', async () => {
    mockPanel({ links: [link({ id: 7, value: LINK_VALUE })], targets: [] });
    renderPage();

    await screen.findByText(LINK_VALUE);
    fireEvent.click(rowButton(LINK_VALUE, 'Assigned to'));

    expect(await screen.findByText('Assigned to nothing yet.')).toBeTruthy();
  });

  it('sends the picked client and clears the form for the next assignment', async () => {
    const { posts } = mockPanel({
      links: [link({ id: 7, value: LINK_VALUE })],
      clients: [{ email: 'b@x' }, { email: 'a@x' }],
    });
    renderPage();

    await screen.findByText(LINK_VALUE);
    fireEvent.click(rowButton(LINK_VALUE, 'Assigned to'));
    await screen.findByText('Assigned to nothing yet.');

    const emailsSelect = selectInFormItemFor('Client');
    // The option list arrives with the clients query, not with the modal.
    openSelect(emailsSelect);
    await waitFor(() => expect(optionTitles()).toContain('a@x'));
    clickOption('a@x');
    expect(chosenIn(emailsSelect)).toEqual(['a@x']);

    fireEvent.click(buttonByText('Assign', modalRoot()));

    await waitFor(() => expect(posts).toHaveLength(1));
    expect(posts[0]).toEqual({
      url: '/panel/api/links/assign/7',
      payload: { emails: ['a@x'], group: '', inboundId: 0, global: false, newClients: false },
    });
    // The toast carries the count the endpoint reported, not the form's.
    expect(await screen.findByText('2 clients now inherit it')).toBeTruthy();
    expect(chosenIn(emailsSelect)).toEqual([]);
  });

  it('sends the scope flag alone for a group, an inbound and every client', async () => {
    const { posts } = mockPanel({
      links: [link({ id: 7, value: LINK_VALUE })],
      groups: [{ name: 'paid', clientCount: 0, trafficUsed: 0, up: 0, down: 0 }],
      inbounds: [{ id: 12, remark: 'in-12' }],
    });
    renderPage();

    await screen.findByText(LINK_VALUE);
    fireEvent.click(rowButton(LINK_VALUE, 'Assigned to'));
    await screen.findByText('Assigned to nothing yet.');

    openSelect(selectInFormItemFor('Scope'));
    clickOption('Group');
    openSelect(selectInFormItemFor('Group'));
    clickOption('paid');
    fireEvent.click(buttonByText('Assign', modalRoot()));

    await waitFor(() => expect(posts).toHaveLength(1));
    expect(posts[0].payload).toEqual({
      emails: [],
      group: 'paid',
      inboundId: 0,
      global: false,
      newClients: false,
    });

    openSelect(selectInFormItemFor('Scope'));
    clickOption('Inbound');
    openSelect(selectInFormItemFor('Inbound'));
    clickOption('in-12');
    fireEvent.click(buttonByText('Assign', modalRoot()));

    await waitFor(() => expect(posts).toHaveLength(2));
    expect(posts[1].payload).toEqual({
      emails: [],
      group: '',
      inboundId: 12,
      global: false,
      newClients: false,
    });

    openSelect(selectInFormItemFor('Scope'));
    clickOption('Every client');
    fireEvent.click(buttonByText('Assign', modalRoot()));

    await waitFor(() => expect(posts).toHaveLength(3));
    expect(posts[2].payload).toEqual({
      emails: [],
      group: '',
      inboundId: 0,
      global: true,
      newClients: false,
    });
  });

  it('unassigns the exact target the tag names', async () => {
    const { posts } = mockPanel({
      links: [link({ id: 7, value: LINK_VALUE })],
      targets: [
        { targetType: 'client', targetId: 0, name: 'a@x' },
        { targetType: 'group', targetId: 0, name: 'paid' },
        { targetType: 'inbound', targetId: 12, name: '' },
      ],
    });
    renderPage();

    await screen.findByText(LINK_VALUE);
    fireEvent.click(rowButton(LINK_VALUE, 'Assigned to'));
    await screen.findByText('Inbound: 12');

    const tags = ['Client: a@x', 'Group: paid', 'Inbound: 12'];
    for (const [index, tag] of tags.entries()) {
      fireEvent.click(tagFor(tag).querySelector('.ant-tag-close-icon') as HTMLElement);
      await waitFor(() => expect(posts).toHaveLength(index + 1));
    }

    expect(posts.map((post) => ({ url: post.url, payload: post.payload }))).toEqual([
      {
        url: '/panel/api/links/unassign/7',
        payload: { emails: ['a@x'], group: '', inboundId: 0, global: false, newClients: false },
      },
      {
        url: '/panel/api/links/unassign/7',
        payload: { emails: [], group: 'paid', inboundId: 0, global: false, newClients: false },
      },
      {
        url: '/panel/api/links/unassign/7',
        payload: { emails: [], group: '', inboundId: 12, global: false, newClients: false },
      },
    ]);
  });
});
