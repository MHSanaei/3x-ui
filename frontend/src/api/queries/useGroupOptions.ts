import { useQuery } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { keys } from '@/api/queryKeys';
import { GroupSummaryListSchema } from '@/schemas/client';

async function fetchGroupNames(): Promise<string[]> {
  const msg = await HttpUtil.get('/panel/api/clients/groups', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to load groups');
  const validated = parseMsg(msg, GroupSummaryListSchema, 'clients/groups');
  return (validated.obj ?? []).map((group) => group.name);
}

export function useGroupOptions(enabled = true) {
  return useQuery({
    queryKey: keys.clients.groups(),
    queryFn: fetchGroupNames,
    enabled,
    staleTime: 30_000,
  });
}
