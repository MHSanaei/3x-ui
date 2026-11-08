import { fireEvent, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { Status } from '@/models/status';
import OverviewActionBar from '@/pages/index/OverviewActionBar';

import { renderWithProviders } from './test-utils';

const noop = () => {};

function renderBar(
  state: string,
  {
    onStopXray = noop,
    onRestartXray = noop,
  }: { onStopXray?: () => void; onRestartXray?: () => void } = {},
) {
  const status = new Status();
  status.xray.state = state;
  renderWithProviders(
    <OverviewActionBar
      status={status}
      isMobile={false}
      accessLogEnable={false}
      panelVersion="v3.10.1"
      latestVersion="v3.10.1"
      updateAvailable={false}
      onStopXray={onStopXray}
      onRestartXray={onRestartXray}
      onOpenLogs={noop}
      onOpenXrayLogs={noop}
      onOpenAmneziaWGLogs={noop}
      onOpenConfig={noop}
      onOpenBackup={noop}
      onOpenSystemHistory={noop}
      onOpenXrayMetrics={noop}
      onOpenPanelUpdate={noop}
      onOpenVersionSwitch={noop}
    />,
  );
}

// Stop used to fire on the first click, so a mis-click killed every proxy connection (#7103).
describe('OverviewActionBar stop confirmation', () => {
  it('does not stop Xray until the dialog is confirmed', async () => {
    const onStopXray = vi.fn();
    renderBar('running', { onStopXray });

    fireEvent.click(screen.getByRole('button', { name: 'Stop' }));
    expect(await screen.findAllByText('Stop Xray?')).not.toHaveLength(0);
    expect(onStopXray).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(onStopXray).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'Stop' }));
    await waitFor(() =>
      expect(screen.getAllByRole('button', { name: 'Stop Xray' })).not.toHaveLength(0),
    );
    fireEvent.click(screen.getAllByRole('button', { name: 'Stop Xray' }).at(-1)!);
    await waitFor(() => expect(onStopXray).toHaveBeenCalledOnce());
  });
});
