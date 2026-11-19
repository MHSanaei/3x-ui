import { keepPreviousData, useInfiniteQuery, useQuery } from '@tanstack/react-query';

import { keys } from '@/api/queryKeys';
import type { LogFileList, LogPage, LogStats } from '@/generated/types';
import { LogFileListSchema, LogPageSchema, LogStatsSchema } from '@/generated/zod';
import { logQueryParams, type LogFilters } from '@/lib/logs/logQuery';
import { HttpUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';

export const LOG_PAGE_SIZE = 200;

interface LogQueryOptions {
  file: string | null;
  filters: LogFilters;
  refetchInterval: number | false;
  enabled: boolean;
}

// jsonObj wraps a failure as " (reason)"; the UI shows the reason alone.
function failure(msg: { msg?: string }): Error {
  return new Error((msg.msg ?? '').replace(/^\s*\((.*)\)$/, '$1'));
}

async function fetchLogFiles(): Promise<LogFileList> {
  const msg = await HttpUtil.get('/panel/api/logs/files', undefined, { silent: true });
  if (!msg?.success) throw failure(msg);
  return parseMsg(msg, LogFileListSchema, 'logs/files', { strict: true }).obj as LogFileList;
}

export function useLogFilesQuery(refetchInterval: number | false) {
  return useQuery({
    queryKey: keys.logs.files(),
    queryFn: fetchLogFiles,
    refetchInterval,
    retry: false,
  });
}

async function fetchLogPage(file: string, filters: LogFilters, cursor: number): Promise<LogPage> {
  const params = { ...logQueryParams(file, filters, Date.now()), limit: LOG_PAGE_SIZE, cursor };
  const msg = await HttpUtil.get('/panel/api/logs/read', params, { silent: true });
  if (!msg?.success) throw failure(msg);
  return parseMsg(msg, LogPageSchema, 'logs/read', { strict: true }).obj as LogPage;
}

export function useLogLinesQuery({ file, filters, refetchInterval, enabled }: LogQueryOptions) {
  return useInfiniteQuery({
    queryKey: keys.logs.lines(file, filters),
    queryFn: ({ pageParam }) => fetchLogPage(file as string, filters, pageParam),
    initialPageParam: 0,
    getNextPageParam: (last: LogPage) => (last.nextCursor > 0 ? last.nextCursor : undefined),
    placeholderData: keepPreviousData,
    enabled: enabled && file !== null,
    refetchInterval,
    retry: false,
  });
}

async function fetchLogStats(file: string, filters: LogFilters): Promise<LogStats> {
  const params = logQueryParams(file, filters, Date.now());
  const msg = await HttpUtil.get('/panel/api/logs/stats', params, { silent: true });
  if (!msg?.success) throw failure(msg);
  return parseMsg(msg, LogStatsSchema, 'logs/stats', { strict: true }).obj as LogStats;
}

export function useLogStatsQuery({ file, filters, refetchInterval, enabled }: LogQueryOptions) {
  return useQuery({
    queryKey: keys.logs.stats(file, filters),
    queryFn: () => fetchLogStats(file as string, filters),
    placeholderData: keepPreviousData,
    enabled: enabled && file !== null,
    refetchInterval,
    retry: false,
  });
}
