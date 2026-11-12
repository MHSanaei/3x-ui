import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { Select } from 'antd';

import { selectTagRender } from '@/components/form';

const SNIS = ['www.microsoft.com', 'www.apple.com', 'dl.google.com'];

function SniSelect({ onValue }: { onValue?: (v: string[]) => void }) {
  const [value, setValue] = useState(SNIS);
  return (
    <Select
      aria-label="SNI"
      mode="tags"
      tagRender={selectTagRender}
      value={value}
      onChange={(next) => {
        setValue(next);
        onValue?.(next);
      }}
    />
  );
}

// Ant Design's own tag "x" is an aria-hidden span with no tabIndex, so a screen-reader
// user could only Backspace the last SNI away, never one from the middle.
describe('selectTagRender', () => {
  it('gives every tag a named, focusable remove button', () => {
    render(<SniSelect />);
    const names = screen.getAllByRole('button').map((b) => b.getAttribute('aria-label'));
    expect(names).toEqual([
      'Remove www.microsoft.com',
      'Remove www.apple.com',
      'Remove dl.google.com',
    ]);
  });

  it('removes a middle tag and moves focus to the next remove button', async () => {
    let latest: string[] = [];
    render(<SniSelect onValue={(v) => (latest = v)} />);
    const middle = screen.getByRole('button', { name: 'Remove www.apple.com' });
    middle.focus();
    fireEvent.click(middle);
    expect(latest).toEqual(['www.microsoft.com', 'dl.google.com']);
    await waitFor(() =>
      expect(document.activeElement).toBe(
        screen.getByRole('button', { name: 'Remove dl.google.com' }),
      ),
    );
  });

  it('falls back to the search input after removing the only tag', async () => {
    function Single() {
      const [value, setValue] = useState(['example.com']);
      return (
        <Select
          aria-label="SNI"
          mode="tags"
          tagRender={selectTagRender}
          value={value}
          onChange={setValue}
        />
      );
    }
    render(<Single />);
    const remove = screen.getByRole('button', { name: 'Remove example.com' });
    remove.focus();
    fireEvent.click(remove);
    await waitFor(() =>
      expect(document.activeElement).toBe(screen.getByRole('combobox', { name: 'SNI' })),
    );
  });

  it('keeps Enter and mouse presses on the remove button from opening the dropdown', () => {
    render(<SniSelect />);
    const combobox = screen.getByRole('combobox', { name: 'SNI' });
    const remove = screen.getByRole('button', { name: 'Remove www.apple.com' });
    fireEvent.keyDown(remove, { key: 'Enter', code: 'Enter', keyCode: 13 });
    fireEvent.mouseDown(remove);
    expect(combobox.getAttribute('aria-expanded')).toBe('false');
  });
});
