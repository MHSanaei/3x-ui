import { describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, within } from '@testing-library/react';

import DynamicEchHelper from '@/components/form/DynamicEchHelper';
import { renderWithProviders } from './test-utils';

function openCustomDomainAnd(type: string) {
  const helper = screen.getByTestId('ech-dynamic-helper');
  const [domainSelect] = within(helper).getAllByRole('combobox');
  fireEvent.mouseDown(domainSelect);
  fireEvent.change(domainSelect, { target: { value: 'custom' } });
  const option = Array.from(
    document.querySelectorAll(
      '.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option',
    ),
  ).find((o) => (o.getAttribute('title') ?? o.textContent ?? '').trim() === 'Add custom option');
  if (!option) throw new Error('custom option not rendered');
  fireEvent.click(option);
  const input = within(helper).getAllByRole('textbox').at(-1) as HTMLInputElement;
  fireEvent.change(input, { target: { value: type } });
  return { helper, input };
}

describe('DynamicEchHelper', () => {
  it('stops offering the value once the custom domain stops being resolvable', () => {
    // A stale preview must not survive an edit that makes the pair invalid:
    // otherwise Apply hands Xray a target that can never resolve.
    renderWithProviders(<DynamicEchHelper hasLocalKeys />);
    const { helper } = openCustomDomainAnd('not a domain');
    expect((within(helper).getByDisplayValue('not a domain') as HTMLInputElement).value).toBe(
      'not a domain',
    );
    expect(within(helper).queryByTestId('ech-dynamic-preview')).toBeNull();
    expect((within(helper).getByTestId('ech-dynamic-apply') as HTMLButtonElement).disabled).toBe(
      true,
    );
  });

  it('drops a confirmation opened for an earlier value when the input changes', () => {
    renderWithProviders(<DynamicEchHelper hasLocalKeys />);
    const helper = screen.getByTestId('ech-dynamic-helper');
    fireEvent.click(within(helper).getByTestId('ech-dynamic-apply'));
    expect(within(helper).getByTestId('ech-dynamic-overwrite-warning')).toBeTruthy();

    openCustomDomainAnd('example.org');
    expect(within(helper).queryByTestId('ech-dynamic-overwrite-warning')).toBeNull();
  });

  it('preselects the Cloudflare target unless the SNI itself can carry ECH', () => {
    const onApply = vi.fn();
    const { unmount } = renderWithProviders(<DynamicEchHelper onApply={onApply} />);
    fireEvent.click(screen.getByTestId('ech-dynamic-apply'));
    expect(onApply).toHaveBeenCalledWith('cloudflare-ech.com+https://dns.alidns.com/dns-query');
    unmount();

    const second = vi.fn();
    renderWithProviders(<DynamicEchHelper onApply={second} sni="t.example.com" />);
    fireEvent.click(screen.getByTestId('ech-dynamic-apply'));
    expect(second).toHaveBeenCalledWith('t.example.com+https://dns.alidns.com/dns-query');
  });
});
