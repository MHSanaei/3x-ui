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

function createButton(): HTMLElement {
  const btn = document.querySelector('.ant-modal-footer .ant-btn-primary');
  if (!btn) throw new Error('Create button not found');
  return btn as HTMLElement;
}

describe('BalancerFormModal', () => {
  it('opens with create button disabled (tag and selector required)', () => {
    renderModal();
    expect(document.querySelector('.ant-modal')).toBeTruthy();
    expect(createButton().hasAttribute('disabled')).toBe(true);
  });

  it('keeps button disabled when only tag is filled (selector still empty)', () => {
    renderModal();
    const tagInput = document.querySelector('.ant-modal input') as HTMLInputElement;
    fireEvent.change(tagInput, { target: { value: 'my-bal' } });
    expect(createButton().hasAttribute('disabled')).toBe(true);
  });

  it('disables button for duplicate tag', () => {
    renderModal();
    const tagInput = document.querySelector('.ant-modal input') as HTMLInputElement;
    fireEvent.change(tagInput, { target: { value: 'existing' } });
    expect(createButton().hasAttribute('disabled')).toBe(true);
  });

  it('does not call onConfirm when form is invalid', () => {
    const { onConfirm } = renderModal();
    fireEvent.click(createButton());
    expect(onConfirm).not.toHaveBeenCalled();
  });
});
