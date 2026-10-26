import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { fireEvent, render } from '@testing-library/react';
import { afterEach, expect, test, vi } from 'vitest';

// vitest stubs CSS imports (even ?raw) to '', so read the sources from disk.
const source = (path: string) => readFileSync(resolve(process.cwd(), 'src', path), 'utf8');
const paletteCss = source('components/command-palette/CommandPalette.css');
const paletteTsx = source('components/command-palette/CommandPalette.tsx');
const glassCss = source('styles/glass.css');

afterEach(() => {
  localStorage.clear();
  document.documentElement.removeAttribute('data-glass');
  document.documentElement.style.removeProperty('--xui-accent');
});

// The palette is plain divs under a DOM-less ConfigProvider, so antd's --ant-*
// vars (scoped to the .xui class on antd roots) resolve to nothing there.
test('command palette colours do not depend on antd-scoped CSS vars', () => {
  expect(paletteCss).not.toMatch(/var\(--ant-/);
  expect(paletteTsx).not.toMatch(/var\(--ant-/);
});

test('every glass glow follows the accent', () => {
  const gradients = glassCss.split('radial-gradient(').slice(1);
  expect(gradients).toHaveLength(3);
  for (const g of gradients) expect(g).toContain('var(--xui-accent)');
});

async function renderWithStoredAccent(stored: string | null) {
  if (stored !== null) localStorage.setItem('accentColor', stored);
  vi.resetModules();
  const { ThemeProvider, useTheme } = await import('@/hooks/useTheme');
  function PickRed() {
    const { setAccent } = useTheme();
    return <button onClick={() => setAccent('red')}>red</button>;
  }
  return render(
    <ThemeProvider>
      <PickRed />
    </ThemeProvider>,
  );
}

test('--xui-accent is on :root with the glass style off', async () => {
  const { getByRole } = await renderWithStoredAccent(null);
  expect(document.documentElement.hasAttribute('data-glass')).toBe(false);
  expect(document.documentElement.style.getPropertyValue('--xui-accent')).toBe('#1677ff');

  fireEvent.click(getByRole('button'));
  expect(document.documentElement.style.getPropertyValue('--xui-accent')).toBe('#ff5468');
});

test('an unknown stored accent falls back to the default blue', async () => {
  await renderWithStoredAccent('teal');
  expect(document.documentElement.style.getPropertyValue('--xui-accent')).toBe('#1677ff');
});
