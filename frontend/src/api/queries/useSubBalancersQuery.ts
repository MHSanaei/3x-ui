import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';

import { HttpUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { keys } from '@/api/queryKeys';
import { SubBalancerListSchema, type SubBalancer } from '@/schemas/subBalancer';

async function fetchSubBalancers(): Promise<SubBalancer[]> {
  const msg = await HttpUtil.get('/panel/api/sub-balancers', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch subscription balancers');
  const validated = parseMsg(msg, SubBalancerListSchema, 'sub-balancers');
  return Array.isArray(validated.obj) ? validated.obj : [];
}

export function useSubBalancersQuery() {
  const query = useQuery({
    queryKey: keys.subBalancers.list(),
    queryFn: fetchSubBalancers,
  });

  const balancers = useMemo(() => query.data ?? [], [query.data]);

  return {
    balancers,
    loading: query.isFetching,
    fetched: query.data !== undefined || query.isError,
    fetchError: query.error ? (query.error as Error).message : '',
    refetch: query.refetch,
  };
}
