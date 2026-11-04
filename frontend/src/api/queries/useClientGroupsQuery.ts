import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';

import { HttpUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { GroupSummaryListSchema, type GroupSummary } from '@/schemas/client';
import { keys } from '@/api/queryKeys';

async function fetchClientGroups(): Promise<GroupSummary[]> {
  const msg = await HttpUtil.get('/panel/api/clients/groups', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch client groups');
  const validated = parseMsg(msg, GroupSummaryListSchema, 'clients/groups');
  return validated.obj ?? [];
}

export function useClientGroupsQuery() {
  const query = useQuery({
    queryKey: keys.clients.groups(),
    queryFn: fetchClientGroups,
  });

  const groups = useMemo(() => (query.data ?? []).filter((group) => group.id > 0), [query.data]);

  return {
    groups,
    loading: query.isFetching,
  };
}
