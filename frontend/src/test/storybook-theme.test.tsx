import { render } from '@testing-library/react';
import { expect, test } from 'vitest';

import preview from '../../.storybook/preview';
import { ThemeProvider } from '@/hooks/useTheme';

const decorator = Array.isArray(preview.decorators) ? preview.decorators[0] : preview.decorators;

if (!decorator) throw new Error('Storybook theme decorator is required');
const storybookDecorator = decorator;

function Story() {
  return <div>Story</div>;
}

test('preserves unrelated body classes when applying the Storybook theme', () => {
  document.body.className = 'storybook-fixture';

  function DecoratedStory() {
    return storybookDecorator(Story, { globals: { theme: 'light' } } as unknown as Parameters<typeof storybookDecorator>[1]);
  }

  render(<DecoratedStory />);

  expect(document.body.classList.contains('storybook-fixture')).toBe(true);
  expect(document.body.classList.contains('light')).toBe(true);
  document.body.className = '';
});

test('preserves unrelated body classes when applying the panel theme', () => {
  document.body.className = 'panel-fixture';

  render(<ThemeProvider><div>Panel</div></ThemeProvider>);

  expect(document.body.classList.contains('panel-fixture')).toBe(true);
  expect(document.body.classList.contains('dark') || document.body.classList.contains('light')).toBe(true);
  document.body.className = '';
});
