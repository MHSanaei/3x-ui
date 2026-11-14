import { act, renderHook, waitFor } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';

import { Status } from '@/models/status';
import { useOverviewHistory } from '@/pages/index/useOverviewHistory';

const { get } = vi.hoisted(() => ({ get: vi.fn() }));
vi.mock('@/utils', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/utils')>();
  return { ...original, HttpUtil: { get } };
});

const metricKeys = [
  'cpu',
  'mem',
  'swap',
  'diskUsage',
  'netUp',
  'netDown',
  'tcpCount',
  'udpCount',
] as const;

function historyResponse(cpu = 10) {
  return Object.fromEntries(metricKeys.map((key) => [key, [{ t: 1, v: key === 'cpu' ? cpu : 5 }]]));
}

beforeEach(() => {
  get.mockReset();
  get.mockResolvedValue({ success: true, obj: historyResponse() });
});

it('waits for status data and requests the default two-minute range once', async () => {
  const { result, rerender } = renderHook(
    ({ status, hasData }) => useOverviewHistory(status, hasData),
    { initialProps: { status: new Status(), hasData: false } },
  );

  expect(get).not.toHaveBeenCalled();
  rerender({ status: new Status(), hasData: true });

  await waitFor(() => expect(result.current.series.cpu).toEqual([10]));
  expect(get).toHaveBeenCalledTimes(1);
  expect(get.mock.calls[0][0]).toBe('/panel/api/server/overviewHistory/2');
});

it('loads a new bucket when the range changes and ignores the previous range', async () => {
  let releaseOld!: (value: { success: boolean; obj: ReturnType<typeof historyResponse> }) => void;
  const pendingOld = new Promise<{ success: boolean; obj: ReturnType<typeof historyResponse> }>(
    (resolve) => {
      releaseOld = resolve;
    },
  );
  get.mockReturnValueOnce(pendingOld).mockResolvedValueOnce({
    success: true,
    obj: historyResponse(90),
  });

  const { result, rerender } = renderHook(
    ({ bucket }) => useOverviewHistory(new Status(), true, bucket),
    { initialProps: { bucket: 2 } },
  );
  rerender({ bucket: 720 });

  await waitFor(() => expect(result.current.series.cpu).toEqual([90]));
  await act(async () => {
    releaseOld({ success: true, obj: historyResponse(20) });
  });

  expect(get.mock.calls.map((call) => call[0])).toEqual([
    '/panel/api/server/overviewHistory/2',
    '/panel/api/server/overviewHistory/720',
  ]);
  expect(result.current.series.cpu).toEqual([90]);
});

it('keeps container-scoped CPU history empty when that metric is unavailable', async () => {
  const obj = historyResponse();
  obj.cpu = [];
  const status = new Status({ resourceScope: 'container', unavailableMetrics: ['cpu'] });
  get.mockResolvedValue({ success: true, obj });

  const { result } = renderHook(() => useOverviewHistory(status, true));
  await waitFor(() => {
    expect(get).toHaveBeenCalledTimes(1);
    expect(result.current.series.cpu).toEqual([]);
    expect(result.current.series.mem).toEqual([5]);
  });
});
