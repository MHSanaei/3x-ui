import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';

import { CipherSuitesSelect } from '@/components/form';

function renderSelect(value: string) {
  const onChange = vi.fn();
  render(<CipherSuitesSelect aria-label="cipher suites" value={value} onChange={onChange} />);
  return onChange;
}

describe('CipherSuitesSelect', () => {
  it('shows each colon-separated suite as its own tag', () => {
    renderSelect('TLS_AES_256_GCM_SHA384:MY_CUSTOM_SUITE');
    expect(screen.getByText('TLS_AES_256_GCM_SHA384')).toBeTruthy();
    expect(screen.getByText('MY_CUSTOM_SUITE')).toBeTruthy();
  });

  it('stores a typed custom suite joined with colons after the existing one', () => {
    const onChange = renderSelect('TLS_AES_256_GCM_SHA384');
    const input = screen.getByRole('combobox', { name: 'cipher suites' });
    fireEvent.change(input, { target: { value: 'MY_CUSTOM_SUITE' } });
    fireEvent.keyDown(input, { key: 'Enter', code: 'Enter', keyCode: 13 });
    expect(onChange).toHaveBeenLastCalledWith('TLS_AES_256_GCM_SHA384:MY_CUSTOM_SUITE');
  });

  it('stores an empty string once every suite is removed', () => {
    const onChange = renderSelect('TLS_AES_256_GCM_SHA384');
    const remove = document.querySelector('.ant-select-selection-item-remove');
    expect(remove).not.toBeNull();
    fireEvent.click(remove as Element);
    expect(onChange).toHaveBeenLastCalledWith('');
  });
});
