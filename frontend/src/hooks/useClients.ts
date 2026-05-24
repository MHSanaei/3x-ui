import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } } as const;

export interface ClientTraffic {
  up?: number;
  down?: number;
  total?: number;
  expiryTime?: number;
  enable?: boolean;
  lastOnline?: number;
}

export interface ClientRecord {
  email: string;
  subId?: string;
  uuid?: string;
  password?: string;
  auth?: string;
  flow?: string;
  totalGB?: number;
  expiryTime?: number;
  limitIp?: number;
  tgId?: number | string;
  comment?: string;
  enable?: boolean;
  inboundIds?: number[];
  traffic?: ClientTraffic;
  reverse?: { tag?: string };
  createdAt?: number;
  updatedAt?: number;
  [key: string]: unknown;
}

export interface InboundOption {
  id: number;
  remark?: string;
  protocol?: string;
  port?: number;
  tlsFlowCapable?: boolean;
}

interface ApiMsg<T = unknown> {
  success?: boolean;
  msg?: string;
  obj?: T;
}

interface SubSettings {
  enable: boolean;
  subURI: string;
  subJsonURI: string;
  subJsonEnable: boolean;
}

export interface ClientQueryParams {
  page: number;
  pageSize: number;
  search?: string;
  filter?: string;
  protocol?: string;
  inbound?: number;
  sort?: string;
  order?: 'ascend' | 'descend';
}

export interface ClientsSummary {
  total: number;
  active: number;
  online: string[];
  depleted: string[];
  expiring: string[];
  deactive: string[];
}

interface ClientPageResponse {
  items: ClientRecord[];
  total: number;
  filtered: number;
  page: number;
  pageSize: number;
  summary?: ClientsSummary;
}

const DEFAULT_QUERY: ClientQueryParams = { page: 1, pageSize: 25 };
const DEFAULT_SUMMARY: ClientsSummary = {
  total: 0, active: 0, online: [], depleted: [], expiring: [], deactive: [],
};

function buildQS(p: ClientQueryParams): string {
  const sp = new URLSearchParams();
  sp.set('page', String(p.page || 1));
  sp.set('pageSize', String(p.pageSize || DEFAULT_QUERY.pageSize));
  if (p.search) sp.set('search', p.search);
  if (p.filter) sp.set('filter', p.filter);
  if (p.protocol) sp.set('protocol', p.protocol);
  if (p.inbound && p.inbound > 0) sp.set('inbound', String(p.inbound));
  if (p.sort) sp.set('sort', p.sort);
  if (p.order) sp.set('order', p.order);
  return sp.toString();
}

async function fetchClientPage(params: ClientQueryParams): Promise<ClientPageResponse> {
  const qs = buildQS(params);
  const msg = await HttpUtil.get(`/panel/api/clients/list/paged?${qs}`, undefined, { silent: true }) as ApiMsg<ClientPageResponse>;
  if (!msg?.success || !msg.obj) throw new Error(msg?.msg || 'Failed to fetch clients');
  return msg.obj;
}

async function fetchInboundOptions(): Promise<InboundOption[]> {
  const msg = await HttpUtil.get('/panel/api/inbounds/options', undefined, { silent: true }) as ApiMsg<InboundOption[]>;
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch inbound options');
  return Array.isArray(msg.obj) ? msg.obj : [];
}

async function fetchDefaults(): Promise<Record<string, unknown>> {
  const msg = await HttpUtil.post('/panel/setting/defaultSettings', undefined, { silent: true }) as ApiMsg<Record<string, unknown>>;
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch defaults');
  return msg.obj || {};
}

export function useClients() {
  const queryClient = useQueryClient();

  const [query, setQueryState] = useState<ClientQueryParams>(DEFAULT_QUERY);
  // setQuery shallow-compares so callers can pass a fresh object every render
  // (the common React pattern) without triggering a re-fetch when nothing
  // actually changed.
  const setQuery = useCallback((next: ClientQueryParams) => {
    setQueryState((prev) => {
      if (
        prev.page === next.page
        && prev.pageSize === next.pageSize
        && (prev.search ?? '') === (next.search ?? '')
        && (prev.filter ?? '') === (next.filter ?? '')
        && (prev.protocol ?? '') === (next.protocol ?? '')
        && (prev.inbound ?? 0) === (next.inbound ?? 0)
        && (prev.sort ?? '') === (next.sort ?? '')
        && (prev.order ?? '') === (next.order ?? '')
      ) return prev;
      return next;
    });
  }, []);

  const listQuery = useQuery({
    queryKey: keys.clients.list(query),
    queryFn: () => fetchClientPage(query),
    staleTime: Infinity,
    placeholderData: keepPreviousData,
  });

  const inboundOptionsQuery = useQuery({
    queryKey: keys.inbounds.options(),
    queryFn: fetchInboundOptions,
    staleTime: Infinity,
  });

  const defaultsQuery = useQuery({
    queryKey: keys.settings.defaults(),
    queryFn: fetchDefaults,
    staleTime: Infinity,
  });

  const onlinesQuery = useQuery({
    queryKey: keys.clients.onlines(),
    queryFn: async () => {
      const msg = await HttpUtil.post('/panel/api/clients/onlines', undefined, { silent: true }) as ApiMsg<string[]>;
      if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch onlines');
      return Array.isArray(msg.obj) ? msg.obj : [];
    },
    staleTime: Infinity,
  });

  const clients = listQuery.data?.items ?? [];
  const total = listQuery.data?.total ?? 0;
  const filtered = listQuery.data?.filtered ?? 0;
  const summary = listQuery.data?.summary ?? DEFAULT_SUMMARY;
  const fetched = listQuery.data !== undefined;
  const loading = listQuery.isFetching;

  const inbounds = inboundOptionsQuery.data ?? [];
  const onlines = onlinesQuery.data ?? [];

  const defaults = defaultsQuery.data ?? {};
  const subSettings: SubSettings = useMemo(() => ({
    enable: !!defaults.subEnable,
    subURI: (defaults.subURI as string) || '',
    subJsonURI: (defaults.subJsonURI as string) || '',
    subJsonEnable: !!defaults.subJsonEnable,
  }), [defaults.subEnable, defaults.subURI, defaults.subJsonURI, defaults.subJsonEnable]);

  const ipLimitEnable = !!defaults.ipLimitEnable;
  const tgBotEnable = !!defaults.tgBotEnable;
  const expireDiff = ((defaults.expireDiff as number) ?? 0) * 86400000;
  const trafficDiff = ((defaults.trafficDiff as number) ?? 0) * 1073741824;
  const pageSize = (defaults.pageSize as number) ?? 0;

  const invalidateAll = useCallback(
    () => queryClient.invalidateQueries({ queryKey: keys.clients.root() }),
    [queryClient],
  );

  const refresh = useCallback(async () => {
    await invalidateAll();
  }, [invalidateAll]);

  const hydrate = useCallback(async (email: string): Promise<{ client: ClientRecord; inboundIds: number[] } | null> => {
    if (!email) return null;
    const msg = await HttpUtil.get(`/panel/api/clients/get/${encodeURIComponent(email)}`) as ApiMsg<{ client: ClientRecord; inboundIds: number[] }>;
    if (!msg?.success || !msg.obj) return null;
    return msg.obj;
  }, []);

  const createMut = useMutation({
    mutationFn: (payload: unknown) =>
      HttpUtil.post('/panel/api/clients/add', payload, JSON_HEADERS) as Promise<ApiMsg>,
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const updateMut = useMutation({
    mutationFn: ({ email, client }: { email: string; client: unknown }) =>
      HttpUtil.post(`/panel/api/clients/update/${encodeURIComponent(email)}`, client, JSON_HEADERS) as Promise<ApiMsg>,
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const removeMut = useMutation({
    mutationFn: ({ email, keepTraffic }: { email: string; keepTraffic?: boolean }) => {
      const url = keepTraffic
        ? `/panel/api/clients/del/${encodeURIComponent(email)}?keepTraffic=1`
        : `/panel/api/clients/del/${encodeURIComponent(email)}`;
      return HttpUtil.post(url) as Promise<ApiMsg>;
    },
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const removeManyMut = useMutation({
    mutationFn: async ({ emails, keepTraffic }: { emails: string[]; keepTraffic?: boolean }) => {
      const suffix = keepTraffic ? '?keepTraffic=1' : '';
      const results = await Promise.all(emails.map((email) => {
        const url = `/panel/api/clients/del/${encodeURIComponent(email)}${suffix}`;
        return HttpUtil.post(url, undefined, { silent: true }) as Promise<ApiMsg>;
      }));
      return results;
    },
    onSuccess: () => invalidateAll(),
  });

  const bulkAdjustMut = useMutation({
    mutationFn: (payload: { emails: string[]; addDays: number; addBytes: number }) =>
      HttpUtil.post(
        '/panel/api/clients/bulkAdjust',
        payload,
        JSON_HEADERS,
      ) as Promise<ApiMsg<{ adjusted: number; skipped?: { email: string; reason: string }[] }>>,
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const attachMut = useMutation({
    mutationFn: ({ email, inboundIds }: { email: string; inboundIds: number[] }) =>
      HttpUtil.post(`/panel/api/clients/${encodeURIComponent(email)}/attach`, { inboundIds }, JSON_HEADERS) as Promise<ApiMsg>,
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const detachMut = useMutation({
    mutationFn: ({ email, inboundIds }: { email: string; inboundIds: number[] }) =>
      HttpUtil.post(`/panel/api/clients/${encodeURIComponent(email)}/detach`, { inboundIds }, JSON_HEADERS) as Promise<ApiMsg>,
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const resetTrafficMut = useMutation({
    mutationFn: (email: string) =>
      HttpUtil.post(`/panel/api/clients/resetTraffic/${encodeURIComponent(email)}`) as Promise<ApiMsg>,
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const resetAllTrafficsMut = useMutation({
    mutationFn: () => HttpUtil.post('/panel/api/clients/resetAllTraffics') as Promise<ApiMsg>,
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const delDepletedMut = useMutation({
    mutationFn: () => HttpUtil.post('/panel/api/clients/delDepleted') as Promise<ApiMsg<{ deleted?: number }>>,
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const create = useCallback((payload: unknown) => createMut.mutateAsync(payload), [createMut]);
  const update = useCallback((email: string, client: unknown) => {
    if (!email) return Promise.resolve(null as unknown as ApiMsg);
    return updateMut.mutateAsync({ email, client });
  }, [updateMut]);
  const remove = useCallback((email: string, keepTraffic = false) => {
    if (!email) return Promise.resolve(null as unknown as ApiMsg);
    return removeMut.mutateAsync({ email, keepTraffic });
  }, [removeMut]);
  const removeMany = useCallback((emails: string[], keepTraffic = false) => {
    if (!Array.isArray(emails) || emails.length === 0) return Promise.resolve([] as ApiMsg[]);
    return removeManyMut.mutateAsync({ emails, keepTraffic });
  }, [removeManyMut]);
  const bulkAdjust = useCallback((emails: string[], addDays: number, addBytes: number) => {
    if (!Array.isArray(emails) || emails.length === 0) return Promise.resolve(null);
    return bulkAdjustMut.mutateAsync({ emails, addDays, addBytes });
  }, [bulkAdjustMut]);
  const attach = useCallback((email: string, inboundIds: number[]) => {
    if (!email) return Promise.resolve(null as unknown as ApiMsg);
    return attachMut.mutateAsync({ email, inboundIds });
  }, [attachMut]);
  const detach = useCallback((email: string, inboundIds: number[]) => {
    if (!email) return Promise.resolve(null as unknown as ApiMsg);
    return detachMut.mutateAsync({ email, inboundIds });
  }, [detachMut]);
  const resetTraffic = useCallback((client: ClientRecord) => {
    if (!client?.email) return Promise.resolve(null as unknown as ApiMsg);
    return resetTrafficMut.mutateAsync(client.email);
  }, [resetTrafficMut]);
  const resetAllTraffics = useCallback(() => resetAllTrafficsMut.mutateAsync(), [resetAllTrafficsMut]);
  const delDepleted = useCallback(() => delDepletedMut.mutateAsync(), [delDepletedMut]);

  const setEnable = useCallback(async (client: ClientRecord, enable: boolean) => {
    if (!client?.email) return null;
    const payload = {
      email: client.email,
      subId: client.subId,
      id: client.uuid,
      password: client.password,
      auth: client.auth,
      totalGB: client.totalGB || 0,
      expiryTime: client.expiryTime || 0,
      limitIp: client.limitIp || 0,
      comment: client.comment || '',
      enable: !!enable,
    };
    return update(client.email, payload);
  }, [update]);

  // WS-driven in-place merges. Page wires these via useWebSocket; the bridge
  // covers coarse 'invalidate' and 'inbounds' events centrally.
  const queryRef = useRef(query);
  queryRef.current = query;

  const applyTrafficEvent = useCallback((payload: unknown) => {
    if (!payload || typeof payload !== 'object') return;
    const p = payload as { onlineClients?: string[] };
    if (Array.isArray(p.onlineClients)) {
      queryClient.setQueryData(keys.clients.onlines(), p.onlineClients);
    }
  }, [queryClient]);

  const applyClientStatsEvent = useCallback((payload: unknown) => {
    if (!payload || typeof payload !== 'object') return;
    const p = payload as { clients?: (ClientTraffic & { email?: string })[] };
    if (!Array.isArray(p.clients) || p.clients.length === 0) return;
    const byEmail = new Map<string, ClientTraffic>();
    for (const row of p.clients) {
      if (row && row.email) byEmail.set(row.email, row);
    }
    queryClient.setQueryData<ClientPageResponse>(keys.clients.list(queryRef.current), (prev) => {
      if (!prev) return prev;
      let touched = false;
      const next = prev.items.slice();
      for (let i = 0; i < next.length; i++) {
        const row = next[i];
        const upd = byEmail.get(row?.email);
        if (!upd) continue;
        const merged: ClientTraffic = { ...(row.traffic || {}) };
        if (typeof upd.up === 'number') merged.up = upd.up;
        if (typeof upd.down === 'number') merged.down = upd.down;
        if (typeof upd.total === 'number') merged.total = upd.total;
        if (typeof upd.expiryTime === 'number') merged.expiryTime = upd.expiryTime;
        if (typeof upd.enable === 'boolean') merged.enable = upd.enable;
        if (typeof upd.lastOnline === 'number') merged.lastOnline = upd.lastOnline;
        next[i] = { ...row, traffic: merged };
        touched = true;
      }
      if (!touched) return prev;
      return { ...prev, items: next };
    });
  }, [queryClient]);

  useEffect(() => {
    queryRef.current = query;
  }, [query]);

  return {
    clients,
    total,
    filtered,
    summary,
    hydrate,
    query,
    setQuery,
    inbounds,
    onlines,
    loading,
    fetched,
    subSettings,
    ipLimitEnable,
    tgBotEnable,
    expireDiff,
    trafficDiff,
    pageSize,
    refresh,
    create,
    update,
    remove,
    removeMany,
    bulkAdjust,
    attach,
    detach,
    resetTraffic,
    resetAllTraffics,
    delDepleted,
    setEnable,
    applyTrafficEvent,
    applyClientStatsEvent,
  };
}
