import { describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, within } from '@testing-library/react';

import DynamicEchHelper from '@/components/form/DynamicEchHelper';
import { renderWithProviders } from './test-utils';

function combos() {
  const helper = screen.getByTestId('ech-dynamic-helper');
  return within(helper).getAllByRole('combobox') as HTMLElement[];
}

/** antd virtualises long option lists, so filter with the search box first. */
function chooseOption(combobox: HTMLElement, search: string, match: string) {
  fireEvent.mouseDown(combobox);
  fireEvent.change(combobox, { target: { value: search } });
  const option = Array.from(
    document.querySelectorAll(
      '.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option',
    ),
  ).find((o) => (o.getAttribute('title') ?? o.textContent ?? '').includes(match));
  if (!option) throw new Error(`option not found: ${match}`);
  fireEvent.click(option);
}

describe('DynamicEchHelper', () => {
  it('defaults to the Cloudflare ECH front with AliDNS', () => {
    // Behind Cloudflare the ECH record belongs to Cloudflare, not to the proxied
    // hostname, so the preset must not depend on the inbound SNI.
    const onApply = vi.fn();
    renderWithProviders(<DynamicEchHelper onApply={onApply} />);
    expect(screen.getByTestId('ech-dynamic-preview').textContent).toBe(
      'cloudflare-ech.com+https://dns.alidns.com/dns-query',
    );
    fireEvent.click(screen.getByTestId('ech-dynamic-apply'));
    expect(onApply).toHaveBeenCalledWith('cloudflare-ech.com+https://dns.alidns.com/dns-query');
  });

  it('writes the pair chosen from the preset dropdown', () => {
    const onApply = vi.fn();
    renderWithProviders(<DynamicEchHelper onApply={onApply} />);
    chooseOption(combos()[0], 'dns.google', 'https://dns.google/dns-query');
    fireEvent.click(screen.getByTestId('ech-dynamic-apply'));
    expect(onApply).toHaveBeenCalledWith('cloudflare-ech.com+https://dns.google/dns-query');
  });

  it('offers the udp:// resolvers from the issue list', () => {
    const onApply = vi.fn();
    renderWithProviders(<DynamicEchHelper onApply={onApply} />);
    chooseOption(combos()[0], 'OpenDNS', 'udp://208.67.220.220:443');
    fireEvent.click(screen.getByTestId('ech-dynamic-apply'));
    expect(onApply).toHaveBeenCalledWith('cloudflare-ech.com+udp://208.67.220.220:443');
  });

  it('normalises an upper-case scheme typed into the custom builder', () => {
    // Xray prefix-matches case-sensitively, so "HTTPS://" must ship lower-cased.
    const onApply = vi.fn();
    renderWithProviders(<DynamicEchHelper onApply={onApply} />);
    chooseOption(combos()[0], 'Custom combination', 'Custom combination');

    const dohSelect = combos()[2];
    chooseOption(dohSelect, 'Custom URL', 'Custom URL');
    const customInput = screen.getAllByRole('textbox').at(-1) as HTMLInputElement;
    fireEvent.change(customInput, { target: { value: 'HTTPS://dns.google/dns-query' } });

    fireEvent.click(screen.getByTestId('ech-dynamic-apply'));
    expect(onApply).toHaveBeenCalledWith('cloudflare-ech.com+https://dns.google/dns-query');
  });

  it('refuses a custom resolver URL Xray cannot dial', () => {
    renderWithProviders(<DynamicEchHelper onApply={vi.fn()} />);
    chooseOption(combos()[0], 'Custom combination', 'Custom combination');
    const dohSelect = combos()[2];
    chooseOption(dohSelect, 'Custom URL', 'Custom URL');
    const customInput = screen.getAllByRole('textbox').at(-1) as HTMLInputElement;
    fireEvent.change(customInput, { target: { value: 'tls://1.1.1.1/dns-query' } });

    expect(screen.getByText(/https:\/\/, h2c:\/\/ or udp:\/\//)).toBeTruthy();
    expect((screen.getByTestId('ech-dynamic-apply') as HTMLButtonElement).disabled).toBe(true);
  });

  it('warns before replacing a value that is not in the dynamic form', () => {
    // A bare DoH URL or a local base64 list is valid for Xray; overwriting it
    // silently would destroy a working config.
    renderWithProviders(<DynamicEchHelper onApply={vi.fn()} value="AEX+DQBB" />);
    expect(screen.getByTestId('ech-existing-not-dynamic')).toBeTruthy();
  });

  it('shows no such warning when the current value is already dynamic', () => {
    renderWithProviders(
      <DynamicEchHelper
        onApply={vi.fn()}
        value="cloudflare-ech.com+https://dns.google/dns-query"
      />,
    );
    expect(screen.queryByTestId('ech-existing-not-dynamic')).toBeNull();
    expect(screen.getByTestId('ech-dynamic-preview').textContent).toBe(
      'cloudflare-ech.com+https://dns.google/dns-query',
    );
  });

  it('asks for confirmation before replacing an existing local ECH keypair', () => {
    const onApply = vi.fn();
    const onClearLocalKeys = vi.fn();
    renderWithProviders(
      <DynamicEchHelper hasLocalKeys onApply={onApply} onClearLocalKeys={onClearLocalKeys} />,
    );
    fireEvent.click(screen.getByTestId('ech-dynamic-apply'));
    expect(screen.getByTestId('ech-dynamic-overwrite-warning')).toBeTruthy();
    expect(onApply).not.toHaveBeenCalled();

    fireEvent.click(screen.getByText('Replace and clear local key'));
    expect(onApply).toHaveBeenCalledWith('cloudflare-ech.com+https://dns.alidns.com/dns-query');
    expect(onClearLocalKeys).toHaveBeenCalled();
  });
});
