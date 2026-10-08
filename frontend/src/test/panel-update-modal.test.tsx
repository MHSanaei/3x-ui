import { fireEvent, screen } from '@testing-library/react';
import { Modal } from 'antd';
import { afterEach, describe, expect, it, vi } from 'vitest';

import PanelUpdateModal from '@/pages/index/PanelUpdateModal';
import { HttpUtil } from '@/utils';
import { renderWithProviders } from './test-utils';

let restoreUseModal = () => {};

describe('panel update modal', () => {
  afterEach(() => {
    restoreUseModal();
    restoreUseModal = () => {};
    vi.mocked(HttpUtil.post).mockReset();
  });

  it('lets the confirmation close before the update request finishes', () => {
    const confirm = vi.fn();
    const useModal = vi.spyOn(Modal, 'useModal').mockReturnValue([
      {
        confirm,
        error: vi.fn(),
        warning: vi.fn(),
      } as unknown as ReturnType<typeof Modal.useModal>[0],
      <span key="modal-context" />,
    ]);
    restoreUseModal = () => useModal.mockRestore();
    vi.mocked(HttpUtil.post).mockReturnValue(new Promise(() => {}));
    const onClose = vi.fn();
    const onBusy = vi.fn();
    renderWithProviders(
      <PanelUpdateModal
        open
        info={{
          channel: 'dev',
          currentVersion: 'dev+old',
          latestVersion: 'dev+new',
          currentCommit: 'old',
          latestCommit: 'new',
          updateAvailable: true,
        }}
        devChannelEnable
        onClose={onClose}
        onBusy={onBusy}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: /Update Panel/ }));
    expect(confirm).toHaveBeenCalledOnce();
    const config = confirm.mock.calls[0]?.[0];
    if (!config?.onOk) throw new Error('Confirmation onOk handler was not registered');

    expect(config.onOk()).toBeUndefined();
    expect(onClose).toHaveBeenCalledOnce();
    expect(HttpUtil.post).toHaveBeenCalledWith('/panel/api/server/updatePanel');
    expect(onBusy).toHaveBeenCalledWith({
      busy: true,
      tip: 'Installation is in progress, please do not refresh this page (dev+new)',
    });
  });
});
