import { useQuery } from '@tanstack/react-query';

import { keys } from '@/api/queryKeys';
import {
  normalizeClientUsageHistory,
  type ClientUsageHistoryInfo,
} from '@/lib/clients/usage-history';
import { HttpUtil } from '@/utils';

async function fetchClientUsageHistory(email: string): Promise<ClientUsageHistoryInfo> {
  const msg = await HttpUtil.get<unknown>(
    `/panel/api/clients/usageHistory/${encodeURIComponent(email)}`,
    undefined,
    { silent: true },
  );
  if (!msg?.success || !msg.obj) {
    throw new Error(msg?.msg || 'Failed to load the usage history');
  }
  return normalizeClientUsageHistory(msg.obj);
}

// The edit modal's usage-history fetch: runs only while the modal is open
// for a real client, and caches per email so a reopen is instant.
export function useClientUsageHistoryQuery(email: string | undefined, enabled: boolean) {
  return useQuery({
    queryKey: keys.clients.usageHistory(email ?? ''),
    enabled: enabled && !!email,
    queryFn: () => fetchClientUsageHistory(email as string),
  });
}
