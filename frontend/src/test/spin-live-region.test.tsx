import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Spin } from 'antd';

// Every page wraps its live stats in Spin, which sets aria-live="polite" on that wrapper.
// The fix relies on our aria-live="off" winning; an antd upgrade that flips the prop order breaks it.
describe('Spin live region', () => {
  it('lets aria-live="off" override the polite region around page content', () => {
    render(
      <Spin aria-live="off" spinning={false}>
        <span>47.2%</span>
      </Spin>,
    );
    const region = screen.getByText('47.2%').closest('[aria-live]');
    expect(region?.getAttribute('aria-live')).toBe('off');
  });
});
