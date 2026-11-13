import { act, fireEvent, render } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { ThemeProvider, useTheme } from '@/hooks/useTheme';

/*
 * `system` mode has to survive two things nothing else in the panel exercises:
 * the OS flipping its scheme while the tab stays open, and the legacy
 * `dark-mode` boolean written by every build before this feature.
 */

type SchemeListener = (event: { matches: boolean }) => void;

let systemDark = true;
let listeners: SchemeListener[] = [];

function installMatchMedia() {
  window.matchMedia = ((query: string) => ({
    matches: query.includes('prefers-color-scheme') ? systemDark : false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: (_type: string, listener: SchemeListener) => {
      listeners.push(listener);
    },
    removeEventListener: (_type: string, listener: SchemeListener) => {
      listeners = listeners.filter((l) => l !== listener);
    },
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
}

function flipSystemScheme(dark: boolean) {
  systemDark = dark;
  act(() => {
    listeners.forEach((listener) => listener({ matches: dark }));
  });
}

function Harness() {
  const { mode, isDark, isUltra, setMode, cycleTheme } = useTheme();
  return (
    <div>
      <span data-testid="mode">{mode}</span>
      <span data-testid="dark">{String(isDark)}</span>
      <span data-testid="ultra">{String(isUltra)}</span>
      <button type="button" onClick={() => setMode('system')}>
        system
      </button>
      <button type="button" onClick={cycleTheme}>
        cycle
      </button>
    </div>
  );
}

function renderHarness() {
  const utils = render(
    <ThemeProvider>
      <Harness />
    </ThemeProvider>,
  );
  return {
    ...utils,
    mode: () => utils.getByTestId('mode').textContent,
    dark: () => utils.getByTestId('dark').textContent,
    ultra: () => utils.getByTestId('ultra').textContent,
  };
}

beforeEach(() => {
  systemDark = true;
  listeners = [];
  installMatchMedia();
  localStorage.clear();
});

afterEach(() => {
  document.body.className = '';
  document.documentElement.style.colorScheme = '';
  document.documentElement.removeAttribute('data-theme');
});

describe('theme mode', () => {
  it('honours the legacy dark-mode boolean when theme-mode was never written', () => {
    localStorage.setItem('dark-mode', 'false');

    const ui = renderHarness();

    expect(ui.mode()).toBe('light');
    expect(document.body.classList.contains('light')).toBe(true);
  });

  it('follows the OS scheme in system mode', () => {
    const ui = renderHarness();
    fireEvent.click(ui.getByText('system'));

    expect(ui.mode()).toBe('system');
    expect(ui.dark()).toBe('true');
    expect(document.body.classList.contains('dark')).toBe(true);

    flipSystemScheme(false);

    expect(ui.dark()).toBe('false');
    expect(document.body.classList.contains('light')).toBe(true);
    expect(document.documentElement.style.colorScheme).toBe('light');
  });

  it('keeps an explicit mode pinned when the OS scheme changes', () => {
    const ui = renderHarness();
    expect(ui.mode()).toBe('dark');

    flipSystemScheme(false);

    expect(ui.mode()).toBe('dark');
    expect(ui.dark()).toBe('true');
    expect(document.body.classList.contains('dark')).toBe(true);
  });

  it('cycles light → dark → ultra dark → system → light', () => {
    const ui = renderHarness();
    const seen: string[] = [];

    for (let i = 0; i < 4; i += 1) {
      fireEvent.click(ui.getByText('cycle'));
      seen.push(`${ui.mode()}:${ui.ultra()}`);
    }

    expect(seen).toEqual(['dark:true', 'system:false', 'light:false', 'dark:false']);
  });
});
