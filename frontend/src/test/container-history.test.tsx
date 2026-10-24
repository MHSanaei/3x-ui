import { act, renderHook, waitFor } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';

import { Status } from '@/models/status';
import { useOverviewHistory } from '@/pages/index/useOverviewHistory';

const { get } = vi.hoisted(() => ({ get: vi.fn() }));
vi.mock('@/utils', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/utils')>();
  return { ...original, HttpUtil: { get } };
});

beforeEach(() => {
  get.mockReset();
});

it('keeps missing samples out of trends and resets a changed metric scope', async () => {
  get.mockImplementation(async (path: string) => ({
    success: true,
    obj: path.includes('/cpu/')
      ? [{ t: 1, v: 10 }]
      : [
          { t: 1, v: 5 },
          { t: 2, v: 6 },
        ],
  }));
  const unavailable = new Status({
    resourceScope: 'container',
    cpu: 0,
    unavailableMetrics: ['cpu'],
  });
  const { result, rerender } = renderHook(({ status }) => useOverviewHistory(status, true), {
    initialProps: { status: unavailable },
  });
  await waitFor(() => expect(result.current.series.cpu).toEqual([10]));
  rerender({ status: new Status({ resourceScope: 'container', cpu: 20 }) });
  expect(result.current.series.cpu).toEqual([10, 20]);
  rerender({ status: new Status({ cpu: 50 }) });
  expect(result.current.series.cpu).toEqual([50]);
});

it('ignores a legacy seed that resolves after the scope changes', async () => {
  let release!: (value: { success: boolean; obj: { t: number; v: number }[] }) => void;
  const legacy = new Promise<{ success: boolean; obj: { t: number; v: number }[] }>((resolve) => {
    release = resolve;
  });
  let calls = 0;
  get.mockImplementation(() => {
    calls += 1;
    return calls <= 8 ? legacy : Promise.resolve({ success: true, obj: [] });
  });
  const { result, rerender } = renderHook(({ status }) => useOverviewHistory(status, true), {
    initialProps: { status: new Status({ cpu: 90 }) },
  });
  rerender({ status: new Status({ resourceScope: 'container', cpu: 10 }) });
  await act(async () => release({ success: true, obj: [{ t: 1, v: 90 }] }));
  expect(result.current.series.cpu).toEqual([10]);
});

it('preserves unrelated samples and their seed when only CPU/swap scope changes', async () => {
  get.mockImplementation(async (path: string) => ({
    success: true,
    obj: [{ t: 1, v: path.includes('/netUp/') ? 7 : 5 }],
  }));
  const { result, rerender } = renderHook(({ status }) => useOverviewHistory(status, true), {
    initialProps: { status: new Status({ cpu: 90, netIO: { up: 8, down: 0 } }) },
  });
  await waitFor(() => expect(result.current.series.netUp).toEqual([7, 8]));
  get.mockClear();
  rerender({
    status: new Status({ resourceScope: 'container', cpu: 10, netIO: { up: 9, down: 0 } }),
  });
  expect(result.current.series.netUp).toEqual([7, 8, 9]);
  expect(result.current.series.cpu).toEqual([10]);
  await waitFor(() => expect(get).toHaveBeenCalledTimes(2));
  expect(get.mock.calls.map((args) => args[0]).sort()).toEqual([
    '/panel/api/server/history/cpu/2',
    '/panel/api/server/history/swap/2',
  ]);
  await waitFor(() => expect(result.current.series.cpu).toEqual([5, 10]));
  expect(result.current.series.netUp).toEqual([7, 8, 9]);
});

it('retains legacy padding for unrelated seed gaps while skipping missing container CPU', async () => {
  get.mockImplementation(async (path: string) => ({
    success: true,
    obj: path.includes('/netUp/')
      ? [
          { t: 1, v: 7 },
          { t: 2, v: 8 },
        ]
      : [{ t: 1, v: 5 }],
  }));
  const status = new Status({
    resourceScope: 'container',
    unavailableMetrics: ['cpu', 'swap'],
    netIO: { up: 9, down: 0 },
  });
  const { result } = renderHook(() => useOverviewHistory(status, true));
  await waitFor(() => expect(result.current.series.netUp).toEqual([7, 8, 9]));
  expect(result.current.series.mem).toEqual([5, 0, 0]);
  expect(result.current.series.cpu).toEqual([5]);
  expect(result.current.series.swap).toEqual([5]);
});

it('waits for the first real status before seeding and never doubles that window', async () => {
  get.mockImplementation(async (path: string) => ({
    success: true,
    obj: path.includes('/cpu/')
      ? [{ t: 1, v: 10 }]
      : path.includes('/swap/')
        ? [{ t: 2, v: 6 }]
        : [
            { t: 1, v: 5 },
            { t: 2, v: 6 },
          ],
  }));
  const { result, rerender } = renderHook(
    ({ status, hasData }) => useOverviewHistory(status, hasData),
    {
      initialProps: { status: new Status(), hasData: false },
    },
  );
  await act(async () => {
    await Promise.resolve();
  });
  expect(get).not.toHaveBeenCalled();
  rerender({
    status: new Status({ resourceScope: 'container', cpu: 20, swap: { current: 0, total: 0 } }),
    hasData: true,
  });
  await waitFor(() => expect(result.current.series.cpu).toEqual([10, 20]));
  expect(result.current.series.swap).toEqual([6, 0]);
  expect(get).toHaveBeenCalledTimes(8);
  rerender({ status: new Status({ resourceScope: 'container', cpu: 30 }), hasData: true });
  expect(result.current.series.cpu).toEqual([10, 20, 30]);
  expect(get).toHaveBeenCalledTimes(8);
});

it('does not prepend the same seed after a temporary status error', async () => {
  get.mockResolvedValue({ success: true, obj: [{ t: 1, v: 10 }] });
  const status = new Status({ resourceScope: 'container', cpu: 20 });
  const { result, rerender } = renderHook(({ hasData }) => useOverviewHistory(status, hasData), {
    initialProps: { hasData: true },
  });
  await waitFor(() => expect(result.current.series.cpu).toEqual([10, 20]));
  rerender({ hasData: false });
  rerender({ hasData: true });
  await act(async () => {
    await Promise.resolve();
  });
  expect(get).toHaveBeenCalledTimes(8);
  expect(result.current.series.cpu).toEqual([10, 20]);
});

it('does not reseed unrelated metrics after an empty initial history', async () => {
  get.mockResolvedValue({ success: true, obj: [] });
  const { rerender } = renderHook(({ status }) => useOverviewHistory(status, true), {
    initialProps: { status: new Status({ cpu: 50 }) },
  });
  await act(async () => {
    await Promise.resolve();
  });
  get.mockClear();
  rerender({ status: new Status({ resourceScope: 'container', cpu: 20 }) });
  await waitFor(() => expect(get).toHaveBeenCalledTimes(2));
});

it('can return to a previously seeded scope while a different seed is in flight', async () => {
  get.mockResolvedValue({ success: true, obj: [{ t: 1, v: 10 }] });
  const { result, rerender } = renderHook(({ status }) => useOverviewHistory(status, true), {
    initialProps: { status: new Status({ cpu: 50 }) },
  });
  await waitFor(() => expect(result.current.series.cpu).toEqual([10, 50]));
  let release!: (value: { success: boolean; obj: { t: number; v: number }[] }) => void;
  const pending = new Promise<{ success: boolean; obj: { t: number; v: number }[] }>((resolve) => {
    release = resolve;
  });
  get.mockReturnValue(pending);
  rerender({ status: new Status({ resourceScope: 'container', cpu: 20 }) });
  get.mockResolvedValue({ success: true, obj: [{ t: 1, v: 10 }] });
  rerender({ status: new Status({ cpu: 60 }) });
  await waitFor(() => expect(result.current.series.cpu).toEqual([10, 60]));
  await act(async () => release({ success: true, obj: [{ t: 1, v: 99 }] }));
  expect(result.current.series.cpu).toEqual([10, 60]);
});
