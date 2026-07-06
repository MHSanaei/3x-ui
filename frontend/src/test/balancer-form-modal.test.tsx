import { describe, it, expect, vi } from 'vitest';
import { act, fireEvent, screen, waitFor } from '@testing-library/react';

import BalancerFormModal from '@/pages/xray/balancers/BalancerFormModal';
import { HttpUtil, Msg } from '@/utils';
import type { BalancerFormValue } from '@/pages/xray/balancers/BalancerFormModal';
import { renderWithProviders } from './test-utils';

function renderModal(onConfirm = vi.fn(), balancer: BalancerFormValue | null = null) {
  const result = renderWithProviders(
    <BalancerFormModal
      open
      balancer={balancer}
      outboundTags={['proxy', 'direct']}
      otherTags={['existing']}
      onClose={() => {}}
      onConfirm={onConfirm}
    />,
  );
  return { onConfirm, ...result };
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

  it('blocks submit when a least-load cost regex fails backend validation', async () => {
    vi.mocked(HttpUtil.post).mockResolvedValueOnce({
      success: false,
      msg: 'error parsing regexp: missing closing ]',
      obj: null,
    });
    const onConfirm = vi.fn();
    renderModal(onConfirm, {
      tag: 'load-balancer',
      strategy: 'leastLoad',
      selector: ['proxy'],
      fallbackTag: '',
      settings: {
        costs: [{ regexp: true, match: '[', value: 1 }],
      },
    });

    fireEvent.click(createButton());

    await waitFor(() => expect(screen.getByText(/missing closing/)).toBeTruthy());
    expect(HttpUtil.post).toHaveBeenCalledWith(
      '/panel/api/setting/validateRegex',
      { regex: '[' },
      { silent: true },
    );
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it('does not confirm after a pending regex validation is cancelled by unmounting', async () => {
    let resolveValidation!: (value: Msg) => void;
    const pendingValidation = new Promise<Msg>((resolve) => {
      resolveValidation = resolve;
    });
    vi.mocked(HttpUtil.post).mockReturnValueOnce(pendingValidation);
    const onConfirm = vi.fn();
    const { unmount } = renderModal(onConfirm, {
      tag: 'load-balancer',
      strategy: 'leastLoad',
      selector: ['proxy'],
      fallbackTag: '',
      settings: {
        costs: [{ regexp: true, match: '^proxy-', value: 1 }],
      },
    });

    fireEvent.click(createButton());

    await waitFor(() => {
      const cancelButton = document.querySelector('.ant-modal-footer .ant-btn-default') as HTMLButtonElement | null;
      expect(cancelButton?.disabled).toBe(true);
      expect(document.querySelector('.ant-modal-close')).toBeNull();
    });
    unmount();
    await act(async () => {
      resolveValidation(new Msg(true));
      await pendingValidation;
    });

    expect(onConfirm).not.toHaveBeenCalled();
  });

  it('prevents edits while regex validation is pending', async () => {
    let resolveValidation!: (value: Msg) => void;
    const pendingValidation = new Promise<Msg>((resolve) => {
      resolveValidation = resolve;
    });
    vi.mocked(HttpUtil.post).mockReturnValueOnce(pendingValidation);
    const onConfirm = vi.fn();
    renderModal(onConfirm, {
      tag: 'load-balancer',
      strategy: 'leastLoad',
      selector: ['proxy'],
      fallbackTag: '',
      settings: {
        costs: [{ regexp: true, match: '^proxy-', value: 1 }],
      },
    });
    const tagInput = screen.getByDisplayValue('load-balancer') as HTMLInputElement;

    fireEvent.click(createButton());

    await waitFor(() => expect(tagInput.disabled).toBe(true));
    fireEvent.change(tagInput, { target: { value: 'edited-while-pending' } });
    expect(tagInput.value).toBe('load-balancer');

    await act(async () => {
      resolveValidation(new Msg(true));
      await pendingValidation;
    });

    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(onConfirm.mock.calls[0][0].tag).toBe('load-balancer');
  });
});
