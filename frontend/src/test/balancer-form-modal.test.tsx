import { describe, it, expect, vi } from 'vitest';
import { fireEvent } from '@testing-library/react';

import BalancerFormModal from '@/pages/xray/balancers/BalancerFormModal';
import { renderWithProviders } from './test-utils';

function renderModal(onConfirm = vi.fn()) {
  renderWithProviders(
    <BalancerFormModal
      open
      balancer={null}
      outboundTags={['proxy', 'direct']}
      otherTags={['existing']}
      onClose={() => {}}
      onConfirm={onConfirm}
    />,
  );
  return { onConfirm };
}

function erroredItemCount(): number {
  return document.querySelectorAll('.ant-form-item-has-error').length;
}

function explainText(): string {
  return Array.from(document.querySelectorAll('.ant-form-item-explain'))
    .map((el) => (el.textContent ?? '').trim())
    .join(' | ');
}

function createButton(): HTMLElement {
  const btn = document.querySelector('.ant-modal-footer .ant-btn-primary');
  if (!btn) throw new Error('Create button not found');
  return btn as HTMLElement;
}

describe('BalancerFormModal', () => {
  it('shows no validation errors when freshly opened in add mode', () => {
    renderModal();
    expect(document.querySelector('.ant-modal')).toBeTruthy();
    expect(erroredItemCount()).toBe(0);
    expect(explainText()).not.toContain('Tag is required');
    expect(explainText()).not.toContain('Pick at least one outbound');
    expect(createButton().hasAttribute('disabled')).toBe(false);
  });

  it('reveals required-field errors only after a save attempt, without confirming', () => {
    const { onConfirm } = renderModal();
    expect(erroredItemCount()).toBe(0);

    fireEvent.click(createButton());

    expect(erroredItemCount()).toBe(2);
    expect(explainText()).toContain('Tag is required');
    expect(explainText()).toContain('Pick at least one outbound');
    expect(onConfirm).not.toHaveBeenCalled();
  });
});
