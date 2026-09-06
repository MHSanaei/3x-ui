import { describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import PanelUpdateModal from '../pages/index/PanelUpdateModal';
import type { PanelUpdateInfo } from '../pages/index/PanelUpdateModal';
import { HttpUtil, Msg } from '@/utils';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, options?: Record<string, unknown>) => {
      if (options?.version) return `${key}:${String(options.version)}`;
      return key;
    },
  }),
}));

describe('PanelUpdateModal', () => {
  it('renders rollback button when on dev channel', () => {
    const devInfo: PanelUpdateInfo = {
      channel: 'dev',
      currentVersion: 'dev+12345678',
      currentCommit: '12345678',
      latestVersion: 'dev+87654321',
      latestCommit: '87654321',
      updateAvailable: true,
      hasLocalSnapshot: true,
      localSnapshotVersion: 'v2.4.9',
      latestStableVersion: 'v2.5.0',
    };

    render(
      <PanelUpdateModal
        open={true}
        info={devInfo}
        devChannelEnable={true}
        onClose={vi.fn()}
        onBusy={vi.fn()}
      />,
    );

    expect(screen.getByText('pages.index.rollbackToRelease')).toBeDefined();
  });

  it('does not render rollback button when on stable channel', () => {
    const stableInfo: PanelUpdateInfo = {
      channel: 'stable',
      currentVersion: '2.4.9',
      latestVersion: '2.5.0',
      updateAvailable: true,
    };

    render(
      <PanelUpdateModal
        open={true}
        info={stableInfo}
        devChannelEnable={false}
        onClose={vi.fn()}
        onBusy={vi.fn()}
      />,
    );

    expect(screen.queryByText('pages.index.rollbackToRelease')).toBeNull();
  });

  it('triggers rollbackPanel API call when rollback confirmed', async () => {
    const postSpy = vi
      .spyOn(HttpUtil, 'post')
      .mockResolvedValue(new Msg(true, '', { runId: '123' }) as Msg<{ runId: string }>);

    const devInfo: PanelUpdateInfo = {
      channel: 'dev',
      currentVersion: 'dev+12345678',
      latestVersion: 'dev+12345678',
      updateAvailable: false,
      hasLocalSnapshot: true,
      localSnapshotVersion: 'v2.4.9',
      latestStableVersion: 'v2.5.0',
    };

    render(
      <PanelUpdateModal
        open={true}
        info={devInfo}
        devChannelEnable={true}
        onClose={vi.fn()}
        onBusy={vi.fn()}
      />,
    );

    const rollbackBtn = screen.getByText('pages.index.rollbackToRelease');
    fireEvent.click(rollbackBtn);

    // Confirm dialog should appear
    await waitFor(() => {
      expect(screen.getAllByText('pages.index.rollbackDialogTitle').length).toBeGreaterThan(0);
    });

    const confirmBtn = screen.getByText('confirm');
    fireEvent.click(confirmBtn);

    await waitFor(() => {
      expect(postSpy).toHaveBeenCalledWith('/panel/api/server/rollbackPanel', expect.anything());
    });
  });
});
