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

function openDropdown(combobox: HTMLElement) {
  fireEvent.mouseDown(combobox);
}

function pickDohEndpoint(label: string) {
  const helper = screen.getByTestId('ech-dynamic-helper');
  const [, dohSelect] = within(helper).getAllByRole('combobox');
  openDropdown(dohSelect);
  const option = Array.from(
    document.querySelectorAll(
      '.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option',
    ),
  ).find((o) => (o.getAttribute('title') ?? o.textContent ?? '').trim().startsWith(label));
  if (!option) {
    const seen = Array.from(document.querySelectorAll('.ant-select-item-option'))
      .map((o) => o.getAttribute('title') ?? o.textContent ?? '')
      .join(' | ');
    throw new Error(`DoH option '${label}' not rendered; saw: ${seen}`);
  }
  fireEvent.click(option);
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

  it('drops a confirmation opened for an earlier value when the pickers change', () => {
    // A stale prompt would let the operator confirm a pair the form no longer
    // shows, applying an endpoint they did not pick.
    renderWithProviders(<DynamicEchHelper hasLocalKeys />);
    const helper = screen.getByTestId('ech-dynamic-helper');
    fireEvent.click(within(helper).getByTestId('ech-dynamic-apply'));
    expect(within(helper).getByTestId('ech-dynamic-overwrite-warning')).toBeTruthy();
    expect(within(helper).getByTestId('ech-dynamic-overwrite-value').textContent).toBe(
      'cloudflare-ech.com+https://dns.alidns.com/dns-query',
    );

    pickDohEndpoint('https://dns.google/dns-query');
    expect(within(helper).queryByTestId('ech-dynamic-overwrite-warning')).toBeNull();

    fireEvent.click(within(helper).getByTestId('ech-dynamic-apply'));
    expect(within(helper).getByTestId('ech-dynamic-overwrite-value').textContent).toBe(
      'cloudflare-ech.com+https://dns.google/dns-query',
    );
  });

  it('confirms with the pair the form shows now, not the one confirmed earlier', () => {
    // The confirm handler used to replay the captured preview, so a picker change
    // made between Apply and Replace applied the stale endpoint. Keeping the
    // prompt open across the change is what makes the live value observable.
    const onApply = vi.fn();
    renderWithProviders(
      <DynamicEchHelper hasLocalKeys onApply={onApply} onClearLocalKeys={vi.fn()} />,
    );
    const helper = screen.getByTestId('ech-dynamic-helper');
    fireEvent.click(within(helper).getByTestId('ech-dynamic-apply'));

    pickDohEndpoint('https://dns.google/dns-query');
    fireEvent.click(within(helper).getByTestId('ech-dynamic-apply'));

    const warning = within(helper).getByTestId('ech-dynamic-overwrite-warning');
    fireEvent.click(within(warning).getByRole('button', { name: /replace/i }));
    expect(onApply).toHaveBeenLastCalledWith('cloudflare-ech.com+https://dns.google/dns-query');
  });

  it('treats a pasted value with an unsupported scheme as unparseable', () => {
    // quic:// does not round-trip, so the helper falls back to its defaults
    // instead of handing back an endpoint Xray can never reach.
    renderWithProviders(
      <DynamicEchHelper onApply={vi.fn()} value="cloudflare-ech.com+quic://1.1.1.1/dns-query" />,
    );
    expect(screen.getByTestId('ech-dynamic-preview').textContent).toBe(
      'cloudflare-ech.com+https://dns.alidns.com/dns-query',
    );
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
