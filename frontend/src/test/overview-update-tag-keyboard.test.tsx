import { fireEvent, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { Status } from '@/models/status';
import OverviewActionBar from '@/pages/index/OverviewActionBar';

import { renderWithProviders } from './test-utils';

const noop = () => {};

// The "update available" Tag only had onClick: keyboard and screen-reader users
// could neither reach nor press it (s-ui#1293).
describe('OverviewActionBar update tag', () => {
  it.each(['Enter', ' '])('opens the panel update from the keyboard (%j)', (key) => {
    const onOpenPanelUpdate = vi.fn();
    renderWithProviders(
      <OverviewActionBar
        status={new Status()}
        isMobile={false}
        accessLogEnable={false}
        panelVersion="v3.10.1"
        latestVersion="v3.10.2"
        updateAvailable
        onStopXray={noop}
        onRestartXray={noop}
        onOpenLogs={noop}
        onOpenXrayLogs={noop}
        onOpenAmneziaWGLogs={noop}
        onOpenConfig={noop}
        onOpenBackup={noop}
        onOpenSystemHistory={noop}
        onOpenXrayMetrics={noop}
        onOpenPanelUpdate={onOpenPanelUpdate}
        onOpenVersionSwitch={noop}
      />,
    );

    const tag = screen.getByRole('button', { name: /3\.10\.2/ });
    expect(tag.tabIndex).toBe(0);
    fireEvent.keyDown(tag, { key });

    expect(onOpenPanelUpdate).toHaveBeenCalledOnce();
  });
});
