import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';

import { HttpUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { keys } from '@/api/queryKeys';
import {
  LinkListSchema,
  LinkTargetListSchema,
  LinkViewListSchema,
  type LinkRecord,
  type LinkTarget,
  type LinkView,
} from '@/schemas/api/link';

async function fetchLinks(): Promise<LinkRecord[]> {
  const msg = await HttpUtil.get('/panel/api/links/list', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to load links');
  const validated = parseMsg(msg, LinkListSchema, 'links/list');
  return validated.obj ?? [];
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

async function fetchTargets(linkId: number): Promise<LinkTarget[]> {
  const msg = await HttpUtil.get(`/panel/api/links/targets/${linkId}`, undefined, {
    silent: true,
  });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to load link targets');
  const validated = parseMsg(msg, LinkTargetListSchema, 'links/targets');
  return validated.obj ?? [];
}

export function useLinkTargetsQuery(linkId: number | null) {
  const query = useQuery({
    queryKey: keys.links.targets(linkId ?? 0),
    queryFn: () => fetchTargets(linkId as number),
    enabled: linkId !== null,
  });

  const targets = useMemo(() => query.data ?? [], [query.data]);

  return { targets, loading: query.isFetching, refetch: query.refetch };
}

async function fetchClientLinks(clientId: number): Promise<LinkView[]> {
  const msg = await HttpUtil.get(`/panel/api/links/client/${clientId}`, undefined, {
    silent: true,
  });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to load client links');
  const validated = parseMsg(msg, LinkViewListSchema, 'links/client');
  return validated.obj ?? [];
}

export function useClientLinksQuery(clientId: number | null) {
  const query = useQuery({
    queryKey: keys.links.client(clientId ?? 0),
    queryFn: () => fetchClientLinks(clientId as number),
    enabled: clientId !== null && clientId > 0,
  });

  const rows = useMemo(() => query.data ?? [], [query.data]);

  return { rows, loading: query.isFetching, refetch: query.refetch };
}
