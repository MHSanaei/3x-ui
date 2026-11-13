import { useEffect, useMemo, useState } from 'react';

import { HttpUtil } from '@/utils';
import type { Status } from '@/models/status';

// The server stores raw samples in compact rollup tiers. The selected bucket
// returns at most 60 points, so even a 12-hour view stays small in the browser.
const OVERVIEW_WINDOW = 60;
const MIN_REFRESH_INTERVAL_MS = 10_000;

const SERIES_KEYS = [
  'cpu',
  'mem',
  'swap',
  'diskUsage',
  'netUp',
  'netDown',
  'tcpCount',
  'udpCount',
] as const;

export type OverviewSeriesKey = (typeof SERIES_KEYS)[number];

export interface OverviewHistory {
  series: Record<OverviewSeriesKey, number[]>;
  labels: string[];
}

interface HistoryPoint {
  t: number;
  v: number;
}

interface HistoryWindow {
  series: Record<OverviewSeriesKey, number[]>;
  times: number[];
  scope: string;
  bucketSeconds: number;
}

function emptySeries(): Record<OverviewSeriesKey, number[]> {
  return Object.fromEntries(SERIES_KEYS.map((key) => [key, [] as number[]])) as Record<
    OverviewSeriesKey,
    number[]
  >;
}

function emptyWindow(scope = '', bucketSeconds = 0): HistoryWindow {
  return { series: emptySeries(), times: [], scope, bucketSeconds };
}

function formatTimeAxis(times: number[]): string[] {
  if (times.length === 0) return [];
  const first = new Date(times[0] * 1000);
  const last = new Date(times[times.length - 1] * 1000);
  const crossesDay =
    first.getFullYear() !== last.getFullYear() ||
    first.getMonth() !== last.getMonth() ||
    first.getDate() !== last.getDate();

  return times.map((timestamp) => {
    const d = new Date(timestamp * 1000);
    const hh = String(d.getHours()).padStart(2, '0');
    const mm = String(d.getMinutes()).padStart(2, '0');
    if (!crossesDay) return `${hh}:${mm}`;
    const MM = String(d.getMonth() + 1).padStart(2, '0');
    const DD = String(d.getDate()).padStart(2, '0');
    return `${MM}/${DD} ${hh}:${mm}`;
  });
}

function mergeHistory(
  previous: HistoryWindow,
  responses: Map<OverviewSeriesKey, HistoryPoint[]>,
  scope: string,
  bucketSeconds: number,
): HistoryWindow {
  const times = Array.from(
    new Set(Array.from(responses.values()).flatMap((points) => points.map((p) => p.t))),
  )
    .filter(Number.isFinite)
    .sort((a, b) => a - b)
    .slice(-OVERVIEW_WINDOW);
  if (times.length === 0) return emptyWindow(scope, bucketSeconds);

  const merged = emptyWindow(scope, bucketSeconds);
  merged.times = times;
  for (const key of SERIES_KEYS) {
    const points = responses.get(key);
    if (!points) {
      merged.series[key] = previous.series[key];
      continue;
    }
    const byTime = new Map(points.map((point) => [point.t, point.v]));
    // CPU and swap are scoped to the container when the panel runs there. A
    // missing scoped series should stay empty instead of borrowing host data.
    const scoped = key === 'cpu' || key === 'swap';
    merged.series[key] =
      scoped && points.length === 0 ? [] : times.map((time) => byTime.get(time) ?? 0);
  }
  return merged;
}

export function mean(values: number[]): number {
  if (values.length === 0) return 0;
  let total = 0;
  for (const v of values) total += v;
  return total / values.length;
}

export function peak(values: number[]): number {
  let max = 0;
  for (const v of values) if (v > max) max = v;
  return max;
}

export function useOverviewHistory(
  status: Status,
  hasData: boolean,
  bucketSeconds = 2,
): OverviewHistory {
  const [trend, setTrend] = useState<HistoryWindow>(() =>
    emptyWindow(status.resourceScope, bucketSeconds),
  );

  useEffect(() => {
    if (!hasData) return undefined;
    let cancelled = false;
    let timeout: number | undefined;

    const refresh = async () => {
      const responses = new Map<OverviewSeriesKey, HistoryPoint[]>();
      try {
        const msg = await HttpUtil.get<Record<OverviewSeriesKey, HistoryPoint[]>>(
          `/panel/api/server/overviewHistory/${bucketSeconds}`,
          undefined,
          { silent: true },
        );
        if (!cancelled && msg?.success && msg.obj) {
          for (const key of SERIES_KEYS) {
            const points = msg.obj[key];
            if (!Array.isArray(points)) continue;
            responses.set(
              key,
              points
                .map((point) => ({ t: Number(point.t), v: Number(point.v) }))
                .filter((point) => Number.isFinite(point.t) && Number.isFinite(point.v)),
            );
          }
        }
      } catch {
        // Keep the last successful window when a history request fails.
      }
      if (cancelled) return;
      if (responses.size === SERIES_KEYS.length) {
        setTrend((previous) =>
          mergeHistory(previous, responses, status.resourceScope, bucketSeconds),
        );
      }
      const refreshInterval = Math.max(
        MIN_REFRESH_INTERVAL_MS,
        Math.min(60_000, bucketSeconds * 1000),
      );
      timeout = window.setTimeout(refresh, refreshInterval);
    };

    void refresh();
    return () => {
      cancelled = true;
      if (timeout !== undefined) window.clearTimeout(timeout);
    };
  }, [hasData, status.resourceScope, bucketSeconds]);

  const visibleTrend =
    trend.scope === status.resourceScope && trend.bucketSeconds === bucketSeconds
      ? trend
      : emptyWindow(status.resourceScope, bucketSeconds);
  const labels = useMemo(() => formatTimeAxis(visibleTrend.times), [visibleTrend.times]);
  return useMemo(() => ({ series: visibleTrend.series, labels }), [visibleTrend.series, labels]);
}
