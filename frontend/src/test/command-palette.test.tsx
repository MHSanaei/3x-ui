import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, fireEvent, screen } from '@testing-library/react';
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

  it('renders and focuses input when opened via store', () => {
    renderPalette();

    act(() => {
      commandPaletteStore.open();
    });

    expect(screen.getByRole('dialog')).toBeTruthy();
    expect(screen.getByPlaceholderText(/Type a command or search/i)).toBeTruthy();
  });

  it('toggles open and closed with Ctrl+K and Escape keyboard shortcuts', () => {
    renderPalette();

    // Trigger Ctrl+K
    act(() => {
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }));
    });
    expect(commandPaletteStore.getSnapshot()).toBe(true);

    // Trigger Escape
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

    // Initial active index 0
    expect(items[0]?.classList.contains('active')).toBe(true);

    // Press ArrowDown -> active index 1
    fireEvent.keyDown(input, { key: 'ArrowDown' });
    const updatedItems = document.querySelectorAll('.command-palette-item');
    expect(updatedItems[1]?.classList.contains('active')).toBe(true);

    // Press ArrowUp -> active index 0
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

    // Should filter items
    expect(screen.getAllByText(/Panel Settings/i).length).toBeGreaterThan(0);
  });
});
