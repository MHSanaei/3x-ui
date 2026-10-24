import { act, fireEvent, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import HostsPage from '@/pages/hosts/HostsPage';
import { HttpUtil, Msg } from '@/utils';
import { renderWithProviders } from './test-utils';
import { installPointerEvents, pointAt } from './pointer-events';

vi.mock('@/layouts/AppSidebar', () => ({ default: () => null }));
vi.mock('@/pages/hosts/HostFormModal', () => ({ default: () => null }));

const initial = [
  { groupId: 'a', remark: 'Host A', sortOrder: 0, inboundIds: [5], hosts: ['a.example.com'] },
  { groupId: 'b', remark: 'Host B', sortOrder: 1, inboundIds: [20], hosts: ['b.example.com'] },
  { groupId: 'c', remark: 'Host C', sortOrder: 2, inboundIds: [5, 20], hosts: ['c.example.com'] },
];

function rowOrder() {
  return [...document.querySelectorAll('tr[data-row-key]')].map((row) =>
    row.getAttribute('data-row-key'),
  );
}

async function openHosts() {
  const view = renderWithProviders(
    <MemoryRouter>
      <HostsPage />
    </MemoryRouter>,
  );
  await screen.findByText('Host A');
  return view;
}

function drag(from: string, to: string, after = false, cancel = false) {
  const target = document.querySelector(`tr[data-row-key="${to}"]`);
  if (!target) throw new Error('missing drop row');
  pointAt(target);
  fireEvent.pointerDown(
    screen.getByRole('button', { name: `Drag to reorder Host ${from.toUpperCase()}` }),
    { button: 0, pointerId: 1, clientY: 0 },
  );
  fireEvent.pointerMove(document, { pointerId: 1, clientX: 10, clientY: after ? 48 : 32 });
  fireEvent[cancel ? 'pointerCancel' : 'pointerUp'](document, {
    pointerId: 1,
    clientX: 10,
    clientY: after ? 48 : 32,
  });
}

describe('Hosts page reordering', () => {
  afterEach(() => vi.unstubAllGlobals());
  beforeEach(() => {
    installPointerEvents();
    let stored = initial.map((h) => ({ ...h }));
    vi.spyOn(HttpUtil, 'get').mockImplementation(
      async (url) => new Msg(true, '', url.includes('/hosts/list') ? stored : []),
    );
    vi.spyOn(HttpUtil, 'post').mockImplementation(async (url, body) => {
      if (url === '/panel/api/hosts/reorder') {
        const ids = (body as { ids: string[] }).ids;
        stored = ids.map((id, sortOrder) => ({
          ...stored.find((h) => h.groupId === id)!,
          sortOrder,
        }));
      }
      return new Msg(true, '', {});
    });
  });

  it('persists one cross-inbound reorder on drop and reloads the saved order', async () => {
    await openHosts();
    drag('b', 'a');
    await waitFor(() =>
      expect(HttpUtil.post).toHaveBeenCalledExactlyOnceWith(
        '/panel/api/hosts/reorder',
        { ids: ['b', 'a', 'c'] },
        { headers: { 'Content-Type': 'application/json' } },
      ),
    );
    await waitFor(() => expect(rowOrder()).toEqual(['b', 'a', 'c']));
  });

  it('does not persist a no-op drop or a cancelled pointer', async () => {
    await openHosts();
    drag('a', 'a');
    drag('a', 'b');
    drag('b', 'a', true);
    drag('b', 'a', false, true);
    expect(HttpUtil.post).not.toHaveBeenCalled();
    expect(rowOrder()).toEqual(['a', 'b', 'c']);
  });

  it.each([
    [false, ['b', 'a', 'c']],
    [true, ['b', 'c', 'a']],
  ])('inserts before/after the target (after=%s)', async (after, expected) => {
    await openHosts();
    drag('a', 'c', after);
    await waitFor(() => expect(rowOrder()).toEqual(expected));
    await waitFor(() =>
      expect(HttpUtil.post).toHaveBeenCalledExactlyOnceWith(
        '/panel/api/hosts/reorder',
        { ids: expected },
        { headers: { 'Content-Type': 'application/json' } },
      ),
    );
  });

  it('persists a touch drag in the mobile RTL table', async () => {
    const nativeMatchMedia = window.matchMedia;
    vi.spyOn(window, 'matchMedia').mockImplementation((query) => ({
      ...nativeMatchMedia(query),
      matches: query === '(max-width: 768px)',
    }));
    const { container } = await openHosts();
    container.dir = 'rtl';
    pointAt(document.querySelector('tr[data-row-key="a"]'));
    const event = { pointerId: 4, button: 0, pointerType: 'touch', clientX: 10, clientY: 32 };
    fireEvent.pointerDown(screen.getByRole('button', { name: 'Drag to reorder Host B' }), {
      ...event,
      clientY: 0,
    });
    fireEvent.pointerMove(document, event);
    fireEvent.pointerUp(document, event);
    await waitFor(() => expect(rowOrder()).toEqual(['b', 'a', 'c']));
    await waitFor(() => expect(HttpUtil.post).toHaveBeenCalledTimes(1));
  });

  it('cancels an active drag on unmount', async () => {
    const view = await openHosts();
    fireEvent.pointerDown(screen.getByRole('button', { name: 'Drag to reorder Host A' }), {
      pointerId: 1,
      button: 0,
      clientY: 0,
    });
    view.unmount();
    fireEvent.pointerMove(document, { pointerId: 1, clientY: 40, clientX: 10 });
    fireEvent.pointerUp(document, { pointerId: 1 });
    expect(HttpUtil.post).not.toHaveBeenCalled();
  });

  it('restores the visible order after failed persistence', async () => {
    vi.spyOn(HttpUtil, 'post').mockResolvedValue(new Msg(false, 'Cannot save order'));
    await openHosts();
    drag('a', 'c', true);
    await screen.findByText('Cannot save order');
    expect(rowOrder()).toEqual(['a', 'b', 'c']);
  });

  it('prevents overlapping reorders while a save is pending', async () => {
    let finish: ((result: Msg<unknown>) => void) | undefined;
    vi.spyOn(HttpUtil, 'post').mockImplementation(
      () =>
        new Promise<Msg<unknown>>((resolve) => {
          finish = resolve;
        }),
    );
    await openHosts();
    drag('b', 'a');
    await waitFor(() => expect(HttpUtil.post).toHaveBeenCalledTimes(1));
    expect(
      (screen.getByRole('button', { name: 'Drag to reorder Host C' }) as HTMLButtonElement)
        .disabled,
    ).toBe(true);
    drag('c', 'a');
    expect(HttpUtil.post).toHaveBeenCalledTimes(1);
    await act(async () => {
      finish?.(new Msg(false, 'Cannot save order'));
    });
    expect(rowOrder()).toEqual(['a', 'b', 'c']);
  });

  it('keeps Up/Down controls and row actions separate from dragging', async () => {
    await openHosts();
    fireEvent.click(screen.getAllByRole('button', { name: 'Move down' })[0]);
    await waitFor(() => expect(rowOrder()).toEqual(['b', 'a', 'c']));
    expect(HttpUtil.post).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getAllByRole('button', { name: 'Edit' })[0]);
    fireEvent.click(screen.getAllByRole('checkbox')[1]);
    expect(HttpUtil.post).toHaveBeenCalledTimes(1);
  });
});
