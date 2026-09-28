import { useQuery } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';
import type { SponsorList } from '@/generated/types';

const EMPTY: SponsorList = { sponsors: [] };

async function fetchSponsors(): Promise<SponsorList> {
  const msg = await HttpUtil.get<SponsorList>('/sponsors', undefined, { silent: true });
  if (!msg?.success || !msg.obj) return EMPTY;
  return { contact: msg.obj.contact, sponsors: msg.obj.sponsors ?? [] };
}

export function useSponsorsQuery() {
  const query = useQuery({
    queryKey: keys.sponsors(),
    queryFn: fetchSponsors,
    staleTime: 60 * 60 * 1000,
    retry: false,
  });
  return { data: query.data ?? EMPTY, fetched: query.isFetched };
}
