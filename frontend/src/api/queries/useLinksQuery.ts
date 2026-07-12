import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';

import { keys } from '@/api/queryKeys';
import { ManagedLinkListSchema, type ManagedLinkRecord } from '@/schemas/api/link';
import { HttpUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';

export type { ManagedLinkRecord };

async function fetchLinks(): Promise<ManagedLinkRecord[]> {
  const msg = await HttpUtil.get('/panel/api/links/list', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch links');
  const validated = parseMsg(msg, ManagedLinkListSchema, 'links/list');
  return Array.isArray(validated.obj) ? validated.obj : [];
}

export function useLinksQuery() {
  const query = useQuery({
    queryKey: keys.links.list(),
    queryFn: fetchLinks,
  });

  const links = useMemo(() => query.data ?? [], [query.data]);

  return {
    links,
    loading: query.isFetching,
    fetched: query.data !== undefined || query.isError,
    fetchError: query.error ? (query.error as Error).message : '',
    refetch: query.refetch,
  };
}
