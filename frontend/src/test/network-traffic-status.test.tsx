import { act, renderHook, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { Status } from '@/models/status';
import { StatusSchema } from '@/schemas/status';
import ThroughputCard from '@/pages/index/ThroughputCard';
import { useOverviewHistory } from '@/pages/index/useOverviewHistory';
import { HttpUtil, Msg } from '@/utils';
import { renderWithProviders } from './test-utils';

vi.mock('@/components/viz', () => ({ Sparkline: () => null }));
afterEach(() => vi.restoreAllMocks());

describe('network traffic status', () => {
  it('preserves selected interface metadata and displays unavailable metrics as missing', () => {
    const status = new Status(
      StatusSchema.parse({
        networkInterface: {
          mode: 'manual',
          name: 'removed0',
          available: false,
          error: 'device removed',
        },
        unavailableMetrics: ['netTraffic', 'netUp', 'netDown'],
        netIO: { up: 0, down: 0 },
        netTraffic: { sent: 0, recv: 0 },
      }),
    );
    expect(status.networkInterface).toEqual({
      mode: 'manual',
      name: 'removed0',
      available: false,
      error: 'device removed',
    });
    renderWithProviders(
      <ThroughputCard
        status={status}
        up={[100]}
        down={[200]}
        labels={['12:00']}
        isMobile={false}
      />,
    );
    expect(
      screen.getByText(
        'The selected network interface is unavailable. Choose an available interface or another monitoring mode.',
      ),
    ).toBeTruthy();
    expect(screen.getAllByText('—')).toHaveLength(4);
  });

  it('does not turn unavailable network samples into real zero throughput history', async () => {
    vi.spyOn(HttpUtil, 'get').mockResolvedValue(new Msg(true, '', []));
    const { result, rerender } = renderHook(({ status }) => useOverviewHistory(status, true), {
      initialProps: { status: new Status({ netIO: { up: 100, down: 200 } }) },
    });
    await act(async () => {});
    rerender({
      status: new Status({ unavailableMetrics: ['netUp', 'netDown'], netIO: { up: 0, down: 0 } }),
    });
    expect(result.current.series.netUp).toEqual([100]);
    expect(result.current.series.netDown).toEqual([200]);
    rerender({ status: new Status({ netIO: { up: 300, down: 400 } }) });
    expect(result.current.series.netUp).toEqual([100, 300]);
  });
});
