import { describe, it, expect, vi } from 'vitest';
import { fireEvent, waitFor } from '@testing-library/react';

import SubBalancerFormModal from '@/pages/settings/SubBalancerFormModal';
import type { SubBalancer } from '@/schemas/subBalancer';
import { renderWithProviders } from './test-utils';

vi.mock('@/api/queries/useInboundOptions', () => ({
  useInboundOptions: () => ({
    data: [
      { id: 1, tag: 'inb-vless', remark: 'First', protocol: 'vless', port: 443 },
      { id: 2, tag: 'inb-ws', remark: 'Second', protocol: 'vmess', port: 8443 },
    ],
    isLoading: false,
  }),
}));

function renderModal(balancer: SubBalancer | null, onConfirm = vi.fn()) {
  renderWithProviders(
    <SubBalancerFormModal
      open
      balancer={balancer}
      onClose={() => {}}
      onConfirm={onConfirm}
    />,
  );
  return { onConfirm };
}

function primaryButton(): HTMLElement {
  const btn = document.querySelector('.ant-modal-footer .ant-btn-primary');
  if (!btn) throw new Error('Primary button not found');
  return btn as HTMLElement;
}

function erroredItemCount(): number {
  return document.querySelectorAll('.ant-form-item-has-error').length;
}

function remarkInput(): HTMLInputElement {
  const el = Array.from(document.querySelectorAll('.ant-modal input'))
    .find((i) => (i as HTMLInputElement).placeholder.includes('Auto'));
  if (!el) throw new Error('Remark input not found');
  return el as HTMLInputElement;
}

function selectInbound(optionTitle: string) {
  const multi = document.querySelector('.ant-select-multiple');
  if (!multi) throw new Error('Inbound multi-select not found');
  // AntD 6 multiple selects have no .ant-select-selector; mousedown on the
  // root toggles the dropdown.
  fireEvent.mouseDown(multi as HTMLElement);
  const option = Array.from(document.querySelectorAll('.ant-select-item-option'))
    .find((o) => (o.getAttribute('title') ?? o.textContent ?? '').trim() === optionTitle);
  if (!option) throw new Error(`Option '${optionTitle}' not found`);
  fireEvent.click(option);
  fireEvent.keyDown(multi, { key: 'Escape' });
}

describe('SubBalancerFormModal', () => {
  it('shows no validation errors when freshly opened in add mode', () => {
    renderModal(null);
    expect(document.querySelector('.ant-modal')).toBeTruthy();
    expect(erroredItemCount()).toBe(0);
    expect(primaryButton().hasAttribute('disabled')).toBe(false);
  });

  it('reveals required-field errors after a save attempt, without confirming', async () => {
    const { onConfirm } = renderModal(null);
    fireEvent.click(primaryButton());
    await waitFor(() => expect(erroredItemCount()).toBe(2));
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it('confirms with parsed values once remark and an inbound are set', async () => {
    const { onConfirm } = renderModal(null);
    fireEvent.change(remarkInput(), { target: { value: '  auto  ' } });
    selectInbound('First');
    fireEvent.click(primaryButton());
    await waitFor(() => expect(onConfirm).toHaveBeenCalledTimes(1));
    expect(onConfirm).toHaveBeenCalledWith({
      remark: 'auto',
      strategy: 'random',
      inboundIds: [1],
      sortOrder: 1,
      enabled: true,
    });
  });

  it('seeds the form from the edited balancer', async () => {
    const { onConfirm } = renderModal({
      id: 7, remark: 'existing', strategy: 'leastPing', inboundIds: [2], sortOrder: 3, enabled: false,
    });
    expect(remarkInput().value).toBe('existing');
    fireEvent.click(primaryButton());
    await waitFor(() => expect(onConfirm).toHaveBeenCalledTimes(1));
    expect(onConfirm).toHaveBeenCalledWith({
      remark: 'existing',
      strategy: 'leastPing',
      inboundIds: [2],
      sortOrder: 3,
      enabled: false,
    });
  });
});
