import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';

import { HttpUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { HostListSchema, type HostRecord } from '@/schemas/api/host';
import { keys } from '@/api/queryKeys';

export type { HostRecord };

async function fetchHosts(): Promise<HostRecord[]> {
  const msg = await HttpUtil.get('/panel/api/hosts/list', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch hosts');
  const validated = parseMsg(msg, HostListSchema, 'hosts/list');
  return Array.isArray(validated.obj) ? validated.obj : [];
}

export function useHostsQuery() {
  const query = useQuery({
    queryKey: keys.hosts.list(),
    queryFn: fetchHosts,
  });

  const hosts = useMemo(() => query.data ?? [], [query.data]);

  return {
    hosts,
    loading: query.isFetching,
    fetched: query.data !== undefined || query.isError,
    fetchError: query.error ? (query.error as Error).message : '',
    refetch: query.refetch,
  };
}
