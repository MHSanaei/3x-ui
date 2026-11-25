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

type Rollups = Record<(typeof metricKeys)[number], { t: number; v: number }[]>;

function rollups(cpu: number): Rollups {
  return Object.fromEntries(
    metricKeys.map((key) => [key, [{ t: 1, v: key === 'cpu' ? cpu : 5 }]]),
  ) as Rollups;
}

const ranged = (path: unknown) => String(path).includes('/overviewHistory/');

beforeEach(() => {
  get.mockReset();
});

it('ignores an answer that arrives after the range was changed', async () => {
  let releaseSixHours!: (value: { success: boolean; obj: Rollups }) => void;
  const sixHours = new Promise<{ success: boolean; obj: Rollups }>((resolve) => {
    releaseSixHours = resolve;
  });
  get.mockImplementation((path: string) =>
    path.endsWith('/overviewHistory/360')
      ? sixHours
      : Promise.resolve({ success: true, obj: ranged(path) ? rollups(90) : [] }),
  );

  const status = new Status({ cpu: 1 });
  const { result, rerender } = renderHook(
    ({ bucket }) => useOverviewHistory(status, true, bucket),
    { initialProps: { bucket: 360 } },
  );
  rerender({ bucket: 720 });
  await waitFor(() => expect(result.current.series.cpu).toEqual([90]));
  await act(async () => releaseSixHours({ success: true, obj: rollups(20) }));

  expect(result.current.series.cpu).toEqual([90]);
  expect(get.mock.calls.map((call) => call[0]).filter(ranged)).toEqual([
    '/panel/api/server/overviewHistory/360',
    '/panel/api/server/overviewHistory/720',
  ]);
});

it('clears the chart on a range change instead of showing the old range under the new label', async () => {
  let releaseTwelveHours!: (value: { success: boolean; obj: Rollups }) => void;
  const twelveHours = new Promise<{ success: boolean; obj: Rollups }>((resolve) => {
    releaseTwelveHours = resolve;
  });
  get.mockImplementation((path: string) =>
    path.endsWith('/overviewHistory/720')
      ? twelveHours
      : Promise.resolve({ success: true, obj: ranged(path) ? rollups(20) : [] }),
  );

  const status = new Status({ cpu: 1 });
  const { result, rerender } = renderHook(
    ({ bucket }) => useOverviewHistory(status, true, bucket),
    { initialProps: { bucket: 360 } },
  );
  await waitFor(() => expect(result.current.series.cpu).toEqual([20]));
  rerender({ bucket: 720 });
  expect(result.current.series.cpu).toEqual([]);
  await act(async () => releaseTwelveHours({ success: true, obj: rollups(90) }));

  expect(result.current.series.cpu).toEqual([90]);
});

it('leaves a metric the host cannot report empty instead of drawing zeros', async () => {
  const obj = rollups(10);
  obj.netUp = [];
  get.mockImplementation((path: string) =>
    Promise.resolve({ success: true, obj: ranged(path) ? obj : [] }),
  );

  const status = new Status();
  const { result } = renderHook(() => useOverviewHistory(status, true, 60));
  await waitFor(() => expect(result.current.series.mem).toEqual([5]));
  expect(result.current.series.netUp).toEqual([]);
});

it('keeps sampling the live trend while a longer range is shown', async () => {
  get.mockImplementation((path: string) =>
    Promise.resolve({ success: true, obj: ranged(path) ? rollups(90) : [] }),
  );

  const { result, rerender } = renderHook(
    ({ status, bucket }) => useOverviewHistory(status, true, bucket),
    { initialProps: { status: new Status({ cpu: 11 }), bucket: 720 } },
  );
  await waitFor(() => expect(result.current.series.cpu).toEqual([90]));
  rerender({ status: new Status({ cpu: 12 }), bucket: 720 });
  rerender({ status: new Status({ cpu: 13 }), bucket: 2 });

  expect(result.current.series.cpu).toEqual([11, 12, 13]);
});
