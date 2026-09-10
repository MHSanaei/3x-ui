import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, fireEvent, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';

import CommandPalette from '@/components/command-palette/CommandPalette';
import { commandPaletteStore } from '@/components/command-palette/useCommandPalette';
import { HttpUtil, Msg } from '@/utils';
import { renderWithProviders } from './test-utils';

function renderPalette() {
  return renderWithProviders(
    <MemoryRouter>
      <CommandPalette />
    </MemoryRouter>,
  );
}

describe('CommandPalette component', () => {
  beforeEach(() => {
    window.HTMLElement.prototype.scrollIntoView = vi.fn();
    act(() => {
      commandPaletteStore.close();
    });
    vi.clearAllMocks();
  });

  it('does not render when closed', () => {
    renderPalette();
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('renders and focuses input when opened via store', async () => {
    renderPalette();

    act(() => {
      commandPaletteStore.open();
    });

    expect(screen.getByRole('dialog')).toBeTruthy();
    const input = screen.getByPlaceholderText(/Type a command or search/i);
    expect(input).toBeTruthy();
    await waitFor(() => {
      expect(document.activeElement).toBe(input);
    });
  });

  it('toggles open and closed with Ctrl+K and Escape keyboard shortcuts', () => {
    renderPalette();

    act(() => {
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', code: 'KeyK', ctrlKey: true }));
    });
    expect(commandPaletteStore.getSnapshot()).toBe(true);

    act(() => {
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    });
    expect(commandPaletteStore.getSnapshot()).toBe(false);

    act(() => {
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'ن', code: 'KeyK', ctrlKey: true }));
    });
    expect(commandPaletteStore.getSnapshot()).toBe(true);

    act(() => {
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    });
    expect(commandPaletteStore.getSnapshot()).toBe(false);
  });

  it('closes when clicking backdrop', () => {
    renderPalette();
    act(() => {
      commandPaletteStore.open();
    });

    const backdrop = screen.getByRole('presentation');
    fireEvent.click(backdrop);

    expect(commandPaletteStore.getSnapshot()).toBe(false);
  });

  it('navigates items with ArrowDown and ArrowUp', () => {
    renderPalette();
    act(() => {
      commandPaletteStore.open();
    });

    const input = screen.getByPlaceholderText(/Type a command or search/i);
    const items = document.querySelectorAll('.command-palette-item');
    expect(items.length).toBeGreaterThan(0);

    expect(items[0]?.classList.contains('active')).toBe(true);

    fireEvent.keyDown(input, { key: 'ArrowDown' });
    const updatedItems = document.querySelectorAll('.command-palette-item');
    expect(updatedItems[1]?.classList.contains('active')).toBe(true);

    fireEvent.keyDown(input, { key: 'ArrowUp' });
    const reupdatedItems = document.querySelectorAll('.command-palette-item');
    expect(reupdatedItems[0]?.classList.contains('active')).toBe(true);
  });

  it('filters items when typing a search query', async () => {
    renderPalette();
    act(() => {
      commandPaletteStore.open();
    });

    const input = screen.getByPlaceholderText(/Type a command or search/i);
    fireEvent.change(input, { target: { value: 'settings' } });

    expect(screen.getAllByText(/Panel Settings/i).length).toBeGreaterThan(0);
  });

  it('resets query on close and does not persist query on reopen', async () => {
    renderPalette();
    act(() => {
      commandPaletteStore.open();
    });

    const input = screen.getByPlaceholderText(/Type a command or search/i) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'settings' } });
    expect(input.value).toBe('settings');

    act(() => {
      commandPaletteStore.close();
    });

    act(() => {
      commandPaletteStore.open();
    });

    const reopenedInput = screen.getByPlaceholderText(
      /Type a command or search/i,
    ) as HTMLInputElement;
    expect(reopenedInput.value).toBe('');
  });

  it('does not show spinning loader on whitespace-only input', () => {
    renderPalette();
    act(() => {
      commandPaletteStore.open();
    });

    const input = screen.getByPlaceholderText(/Type a command or search/i);
    fireEvent.change(input, { target: { value: '   ' } });

    expect(document.querySelector('.command-palette-search-icon.spinning')).toBeNull();
  });

  it('does not display stale client rows when a new search query is being fetched', async () => {
    let resolveBob: ((val: Msg<{ items: unknown[] }>) => void) | undefined;
    const bobPromise = new Promise<Msg<{ items: unknown[] }>>((resolve) => {
      resolveBob = resolve;
    });

    vi.spyOn(HttpUtil, 'get').mockImplementation(async (url: string) => {
      if (url.includes('/panel/api/inbounds/options')) {
        return new Msg(true, '', []);
      }
      if (url.includes('search=ali')) {
        return new Msg(true, '', {
          items: [
            {
              id: 1,
              email: 'alice@example.com',
              totalGB: 1000,
              enable: true,
              traffic: { up: 100, down: 200, total: 1000 },
            },
          ],
        });
      }
      if (url.includes('search=bob')) {
        return bobPromise as Promise<Msg<unknown>>;
      }
      return new Msg(true, '', {});
    });

    renderPalette();
    act(() => {
      commandPaletteStore.open();
    });

    const input = screen.getByPlaceholderText(/Type a command or search/i);

    // Type 'ali' and wait for Alice to appear after debounce
    fireEvent.change(input, { target: { value: 'ali' } });
    await waitFor(
      () => {
        expect(screen.getByText('alice@example.com')).toBeTruthy();
      },
      { timeout: 2000 },
    );

    // Now type 'bob'
    fireEvent.change(input, { target: { value: 'bob' } });

    // Alice must vanish immediately upon new input
    await waitFor(() => {
      expect(screen.queryByText('alice@example.com')).toBeNull();
    });

    // Wait past the 300ms debounce interval while bob fetch is still pending
    await new Promise((resolve) => setTimeout(resolve, 350));

    // Stale Alice row must STILL not be rendered
    expect(screen.queryByText('alice@example.com')).toBeNull();

    // Now resolve bob
    act(() => {
      resolveBob?.(
        new Msg(true, '', {
          items: [
            {
              id: 2,
              email: 'bob@example.com',
              totalGB: 500,
              enable: true,
              traffic: { up: 50, down: 100, total: 500 },
            },
          ],
        }),
      );
    });

    await waitFor(
      () => {
        expect(screen.getByText('bob@example.com')).toBeTruthy();
      },
      { timeout: 2000 },
    );
    expect(screen.queryByText('alice@example.com')).toBeNull();
  });

  it('does not re-trigger loading when adding trailing whitespace to settled query', async () => {
    const getSpy = vi.spyOn(HttpUtil, 'get').mockImplementation(async (url: string) => {
      if (url.includes('/panel/api/inbounds/options')) return new Msg(true, '', []);
      return new Msg(true, '', { items: [] });
    });

    renderPalette();
    act(() => {
      commandPaletteStore.open();
    });

    const input = screen.getByPlaceholderText(/Type a command or search/i);
    fireEvent.change(input, { target: { value: 'abc' } });

    await waitFor(
      () => {
        const calls = getSpy.mock.calls.filter((c) => String(c[0]).includes('search=abc')).length;
        expect(calls).toBe(1);
      },
      { timeout: 2000 },
    );

    // Add trailing whitespace
    fireEvent.change(input, { target: { value: 'abc ' } });
    await new Promise((resolve) => setTimeout(resolve, 350));

    // No extra search call because trimmed query has not changed
    const callsAfterAbcSpace = getSpy.mock.calls.filter((c) =>
      String(c[0]).includes('search=abc'),
    ).length;
    expect(callsAfterAbcSpace).toBe(1);
    expect(document.querySelector('.command-palette-search-icon.spinning')).toBeNull();
  });

  it('keeps the row secondary action independent of the row control', async () => {
    vi.spyOn(HttpUtil, 'post').mockImplementation(
      async (url: string) =>
        new Msg(true, '', url.includes('/setting/all') ? { subURI: 'https://sub.example/' } : {}),
    );
    vi.spyOn(HttpUtil, 'get').mockImplementation(async (url: string) => {
      if (url.includes('/panel/api/inbounds/options')) return new Msg(true, '', []);
      if (url.includes('search=ali')) {
        return new Msg(true, '', {
          items: [
            {
              id: 1,
              email: 'alice@example.com',
              subId: 'sub123',
              enable: true,
              totalGB: 0,
              traffic: { up: 100, down: 200, total: 0 },
            },
          ],
        });
      }
      return new Msg(true, '', {});
    });

    renderPalette();
    act(() => {
      commandPaletteStore.open();
    });

    const input = screen.getByPlaceholderText(/Type a command or search/i);
    fireEvent.change(input, { target: { value: 'ali' } });
    await waitFor(
      () => {
        expect(screen.getByText('alice@example.com')).toBeTruthy();
      },
      { timeout: 2000 },
    );

    const copyBtn = document.querySelector('.command-palette-action-btn');
    expect(copyBtn).toBeTruthy();
    expect(copyBtn?.parentElement?.closest('button')).toBeNull();

    // Enter on the copy button must not also fire the row's own action.
    fireEvent.keyDown(copyBtn as Element, { key: 'Enter' });
    expect(commandPaletteStore.getSnapshot()).toBe(true);
  });

  it('renders a single theme action item without duplicates', () => {
    renderPalette();
    act(() => {
      commandPaletteStore.open();
    });

    const input = screen.getByPlaceholderText(/Type a command or search/i);
    fireEvent.change(input, { target: { value: 'theme' } });

    const themeItems = screen.getAllByText(/Theme/i);
    expect(themeItems.length).toBe(1);
  });
});
