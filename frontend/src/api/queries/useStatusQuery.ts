import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';

import { HttpUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { Status } from '@/models/status';
import { StatusSchema } from '@/schemas/status';
import { keys } from '@/api/queryKeys';

const POLL_INTERVAL_MS = 2000;

async function fetchStatus(): Promise<Status> {
  const msg = await HttpUtil.get('/panel/api/server/status', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch status');
  const validated = parseMsg(msg, StatusSchema, 'server/status');
  return new Status(validated.obj);
}

export function useStatusQuery() {
  const query = useQuery({
    queryKey: keys.server.status(),
    queryFn: fetchStatus,
    refetchInterval: POLL_INTERVAL_MS,
    refetchIntervalInBackground: false,
    staleTime: 0,
  });

  const status = useMemo(() => query.data ?? new Status(), [query.data]);
  const refresh = async () => { await query.refetch(); };

  return {
    status,
    fetched: query.data !== undefined,
    refresh,
  };
}
