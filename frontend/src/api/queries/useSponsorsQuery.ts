import { useQuery } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';
import type { SponsorList } from '@/generated/types';
import { useSponsorVisibility } from './useSponsorVisibility';

const EMPTY: SponsorList = { sponsors: [] };

async function fetchSponsors({ signal }: { signal: AbortSignal }): Promise<SponsorList> {
  const msg = await HttpUtil.get<SponsorList>('/sponsors', undefined, { silent: true, signal });
  if (!msg?.success || !msg.obj) return EMPTY;
  return { contact: msg.obj.contact, sponsors: msg.obj.sponsors ?? [] };
}

export function useSponsorsQuery() {
  const { enabled, fetched: visibilityFetched } = useSponsorVisibility();
  const query = useQuery({
    queryKey: keys.sponsors(),
    queryFn: fetchSponsors,
    staleTime: 60 * 60 * 1000,
    retry: false,
    enabled,
  });
  return {
    data: enabled ? (query.data ?? EMPTY) : EMPTY,
    fetched: query.isFetched,
    enabled,
    visibilityFetched,
  };
}
