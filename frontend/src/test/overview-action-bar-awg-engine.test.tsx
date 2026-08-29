import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { Status } from '@/models/status';
import OverviewActionBar from '@/pages/index/OverviewActionBar';

function baseProps() {
  return {
    status: new Status(),
    isMobile: false,
    accessLogEnable: false,
    panelVersion: '3.7.0-awg.14',
    latestVersion: '3.7.0-awg.14',
    updateAvailable: false,
    onStopXray: vi.fn(),
    onRestartXray: vi.fn(),
    onOpenLogs: vi.fn(),
    onOpenXrayLogs: vi.fn(),
    onOpenAmneziaWGLogs: vi.fn(),
    onOpenConfig: vi.fn(),
    onOpenBackup: vi.fn(),
    onOpenSystemHistory: vi.fn(),
    onOpenXrayMetrics: vi.fn(),
    onOpenPanelUpdate: vi.fn(),
    onOpenVersionSwitch: vi.fn(),
  };
}

describe('OverviewActionBar AmneziaWG engine badge', () => {
  it('shows nothing when the backend has no engine version', () => {
    render(<OverviewActionBar {...baseProps()} />);
    expect(screen.queryByText(/AmneziaWG/)).toBeNull();
  });

  it('shows the current version plainly when it is already the latest known', () => {
    const props = baseProps();
    render(
      <OverviewActionBar
        {...props}
        awgEngineVersion="v3.1.20260814"
        awgEngineLatestVersion="v3.1.20260814"
      />,
    );
    const badge = screen.getByText('AmneziaWG v3.1.20260814');
    expect(badge.closest('.ov-update-tag')).toBeNull();
    fireEvent.click(badge);
    expect(props.onOpenPanelUpdate).toHaveBeenCalled();
  });

  it('highlights when a newer engine version is known', () => {
    const props = baseProps();
    render(
      <OverviewActionBar
        {...props}
        awgEngineVersion="v3.1.20260814"
        awgEngineLatestVersion="v3.1.20260828"
      />,
    );
    const badge = screen.getByText('AmneziaWG v3.1.20260814');
    expect(badge.closest('.ov-update-tag')).toBeTruthy();
    fireEvent.click(badge);
    expect(props.onOpenPanelUpdate).toHaveBeenCalled();
  });
});
