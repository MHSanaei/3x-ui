import { useQuery } from '@tanstack/react-query';
import { z } from 'zod';

import { keys } from '@/api/queryKeys';
import { HttpUtil } from '@/utils';

async function fetchSponsorVisibility({ signal }: { signal: AbortSignal }): Promise<boolean> {
  const msg = await HttpUtil.get('/sponsors/enabled', undefined, { silent: true, signal });
  if (!msg.success) throw new Error('Failed to fetch sponsor visibility');
  return z.boolean().parse(msg.obj);
}

export function useSponsorVisibility() {
  const query = useQuery({
    queryKey: keys.sponsorVisibility(),
    queryFn: fetchSponsorVisibility,
    staleTime: 0,
    refetchInterval: 60_000,
    retry: false,
  });
  return { enabled: query.data === true && !query.isError, fetched: query.isFetched };
}
