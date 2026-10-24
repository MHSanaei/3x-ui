import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';
import { z } from 'zod';

import { HttpUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { keys } from '@/api/queryKeys';

// One row of the raw hosts table (GET /panel/api/hosts/raw), carrying the
// numeric id that per-client host rule bindings reference.
export const RawHostSchema = z
  .object({
    id: z.number(),
    inboundId: z.number().optional(),
    remark: z.string().optional(),
    address: z.string().optional(),
    port: z.number().optional(),
    isDisabled: z.boolean().optional(),
  })
  .loose();

export type RawHost = z.infer<typeof RawHostSchema>;

const RawHostListSchema = z
  .array(RawHostSchema)
  .nullable()
  .transform((v) => v ?? []);

async function fetchRawHosts(): Promise<RawHost[]> {
  const msg = await HttpUtil.get('/panel/api/hosts/raw', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch hosts');
  const validated = parseMsg(msg, RawHostListSchema, 'hosts/raw');
  return Array.isArray(validated.obj) ? validated.obj : [];
}

export function useRawHostsQuery() {
  const query = useQuery({
    queryKey: keys.hosts.raw(),
    queryFn: fetchRawHosts,
  });

  const hosts = useMemo(() => query.data ?? [], [query.data]);

  return {
    hosts,
    loading: query.isFetching,
    fetched: query.data !== undefined || query.isError,
    fetchError: query.error ? (query.error as Error).message : '',
    refetch: query.refetch,
  };
}
