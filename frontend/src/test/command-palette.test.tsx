import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, fireEvent, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';

import CommandPalette from '@/components/command-palette/CommandPalette';
import { commandPaletteStore } from '@/components/command-palette/useCommandPalette';
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
});
