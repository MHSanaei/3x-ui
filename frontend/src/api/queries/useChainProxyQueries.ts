import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';

import { HttpUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { keys } from '@/api/queryKeys';
import {
  ChainProxyListSchema,
  ChainSelectorListSchema,
  type ChainProxy,
  type ChainSelector,
} from '@/schemas/chainProxy';

async function fetchChainProxies(): Promise<ChainProxy[]> {
  const msg = await HttpUtil.get('/panel/api/chain-proxies', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch chain proxies');
  const validated = parseMsg(msg, ChainProxyListSchema, 'chain-proxies');
  return Array.isArray(validated.obj) ? validated.obj : [];
}

async function fetchChainSelectors(): Promise<ChainSelector[]> {
  const msg = await HttpUtil.get('/panel/api/chain-selectors', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch chain selectors');
  const validated = parseMsg(msg, ChainSelectorListSchema, 'chain-selectors');
  return Array.isArray(validated.obj) ? validated.obj : [];
}

export function useChainProxiesQuery() {
  const query = useQuery({ queryKey: keys.chainProxies.list(), queryFn: fetchChainProxies });
  const proxies = useMemo(() => query.data ?? [], [query.data]);
  return {
    proxies,
    loading: query.isFetching,
    fetched: query.data !== undefined || query.isError,
    fetchError: query.error ? (query.error as Error).message : '',
    refetch: query.refetch,
  };
}

export function useChainSelectorsQuery() {
  const query = useQuery({ queryKey: keys.chainSelectors.list(), queryFn: fetchChainSelectors });
  const selectors = useMemo(() => query.data ?? [], [query.data]);
  return {
    selectors,
    loading: query.isFetching,
    fetched: query.data !== undefined || query.isError,
    fetchError: query.error ? (query.error as Error).message : '',
    refetch: query.refetch,
  };
}
