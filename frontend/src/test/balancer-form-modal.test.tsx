import { describe, it, expect, vi } from 'vitest';
import { fireEvent } from '@testing-library/react';

import BalancerFormModal from '@/pages/xray/balancers/BalancerFormModal';
import type { BalancerFormValue } from '@/pages/xray/balancers/BalancerFormModal';
import type { BalancerObject } from '@/schemas/routing';
import { renderWithProviders } from './test-utils';

function renderModal(onConfirm = vi.fn()) {
  renderWithProviders(
    <BalancerFormModal
      open
      balancer={null}
      outboundTags={['proxy', 'direct']}
      balancerTags={[]}
      balancers={[]}
      templateSettings={null}
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

function primaryButton(): HTMLElement {
  const btn = document.querySelector('.ant-modal-footer .ant-btn-primary');
  if (!btn) throw new Error('Primary button not found');
  return btn as HTMLElement;
}

describe('BalancerFormModal', () => {
  it('shows no validation errors when freshly opened in add mode', () => {
    renderModal();
    expect(document.querySelector('.ant-modal')).toBeTruthy();
    expect(erroredItemCount()).toBe(0);
    expect(explainText()).not.toContain('Tag is required');
    expect(explainText()).not.toContain('Pick at least one outbound');
    expect(primaryButton().hasAttribute('disabled')).toBe(false);
  });

  it('reveals required-field errors only after a save attempt, without confirming', () => {
    const { onConfirm } = renderModal();
    expect(erroredItemCount()).toBe(0);

    fireEvent.click(primaryButton());

    expect(erroredItemCount()).toBe(2);
    expect(explainText()).toContain('Tag is required');
    expect(explainText()).toContain('Pick at least one outbound');
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it('disables save and warns when the chosen fallback would create a balancer cycle', () => {
    const editing: BalancerFormValue = { tag: 'A', strategy: 'random', selector: ['proxy'], fallbackTag: 'B' };
    const others: BalancerObject[] = [{ tag: 'B', selector: ['direct'], fallbackTag: 'A' }];
    const onConfirm = vi.fn();
    renderWithProviders(
      <BalancerFormModal
        open
        balancer={editing}
        outboundTags={['proxy', 'direct']}
        balancerTags={['A', 'B']}
        balancers={others}
        templateSettings={null}
        otherTags={['B']}
        onClose={() => {}}
        onConfirm={onConfirm}
      />,
    );

    expect(document.querySelector('.ant-alert-error')).toBeTruthy();
    expect(primaryButton().hasAttribute('disabled')).toBe(true);

    fireEvent.click(primaryButton());
    expect(onConfirm).not.toHaveBeenCalled();
  });
});
