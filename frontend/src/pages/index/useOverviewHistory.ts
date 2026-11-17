import { useEffect, useMemo, useRef, useState } from 'react';

import { HttpUtil, TimeFormatter } from '@/utils';
import type { Status } from '@/models/status';

const OVERVIEW_WINDOW = 72;
const SEED_BUCKET_SECONDS = 2;

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
}

function emptySeries(): Record<OverviewSeriesKey, number[]> {
  return Object.fromEntries(SERIES_KEYS.map((key) => [key, [] as number[]])) as Record<
    OverviewSeriesKey,
    number[]
  >;
}

function emptyWindow(): HistoryWindow {
  return { series: emptySeries(), times: [] };
}

function sampleOf(status: Status): Record<OverviewSeriesKey, number> {
  return {
    cpu: status.cpu.percent,
    mem: status.mem.percent,
    swap: status.swap.percent,
    diskUsage: status.disk.percent,
    netUp: status.netIO.up,
    netDown: status.netIO.down,
    tcpCount: status.tcpCount,
    udpCount: status.udpCount,
  };
}

function tailWindow<T>(values: T[]): T[] {
  return values.slice(-OVERVIEW_WINDOW);
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

/* the seed bucket must be in the backend's allowedHistoryBuckets whitelist;
   2s is the smallest and matches the status poll cadence */
export function useOverviewHistory(status: Status, hasData: boolean): OverviewHistory {
  const [trend, setTrend] = useState<HistoryWindow>(emptyWindow);

  const seededUnscoped = useRef(false);
  const seedState = useRef<{ scope: string; complete: boolean } | null>(null);

  useEffect(() => {
    // The placeholder has no resource scope. Wait for the first real status,
    // then seed each scope once even if polling temporarily loses data.
    if (
      !hasData ||
      (seedState.current?.scope === status.resourceScope && seedState.current.complete)
    ) {
      return;
    }
    seedState.current = { scope: status.resourceScope, complete: false };
    let cancelled = false;
    const seedUnscoped = !seededUnscoped.current;
    const keys = seedUnscoped ? SERIES_KEYS : (['cpu', 'swap'] as const);

    const seed = async () => {
      const responses = new Map<OverviewSeriesKey, HistoryPoint[]>();
      await Promise.all(
        keys.map(async (key) => {
          const msg = await HttpUtil.get<HistoryPoint[]>(
            `/panel/api/server/history/${key}/${SEED_BUCKET_SECONDS}`,
            undefined,
            { silent: true },
          );
          if (msg?.success && Array.isArray(msg.obj)) responses.set(key, msg.obj);
        }),
      );
      if (cancelled || responses.size === 0) return;
      seedState.current = { scope: status.resourceScope, complete: true };
      seededUnscoped.current = true;

      let axis: HistoryPoint[] = [];
      for (const points of responses.values()) {
        if (points.length > axis.length) axis = points;
      }
      axis = tailWindow(axis);
      if (axis.length === 0) return;

      const seedTimes = axis.map((p) => Number(p.t) || 0);
      const seedSeries = emptySeries();
      for (const key of SERIES_KEYS) {
        const byTs = new Map<number, number>();
        for (const p of responses.get(key) ?? []) byTs.set(Number(p.t) || 0, Number(p.v) || 0);
        seedSeries[key] =
          status.resourceScope && (key === 'cpu' || key === 'swap')
            ? seedTimes.flatMap((ts) => (byTs.has(ts) ? [byTs.get(ts)!] : []))
            : seedTimes.map((ts) => byTs.get(ts) ?? 0);
      }

      setTrend((prev) => {
        const merged = emptyWindow();
        merged.times = seedUnscoped ? tailWindow(seedTimes.concat(prev.times)) : prev.times;
        for (const key of SERIES_KEYS) {
          merged.series[key] =
            seedUnscoped || key === 'cpu' || key === 'swap'
              ? tailWindow(seedSeries[key].concat(prev.series[key]))
              : prev.series[key];
        }
        return merged;
      });
    };

    seed().catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [hasData, status.resourceScope]);

  // Each polled status is appended during render; an effect would show the
  // chart one sample behind the numbers beside it.
  const [sampledStatus, setSampledStatus] = useState<Status | null>(null);
  if (hasData && status !== sampledStatus) {
    setSampledStatus(status);
    setTrend((prev) => {
      const scopeChanged = sampledStatus && sampledStatus.resourceScope !== status.resourceScope;
      const point = sampleOf(status);
      const next = emptyWindow();
      next.times = tailWindow(prev.times.concat(Math.floor(Date.now() / 1000)));
      for (const key of SERIES_KEYS) {
        const scoped = key === 'cpu' || key === 'swap';
        const previous = scoped && scopeChanged ? [] : prev.series[key];
        next.series[key] =
          !scoped || status.metricAvailable(key)
            ? tailWindow(previous.concat(point[key]))
            : previous;
      }
      return next;
    });
  }

  const labels = useMemo(() => trend.times.map(TimeFormatter.formatClock), [trend.times]);

  return useMemo(() => ({ series: trend.series, labels }), [trend.series, labels]);
}
