import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { HttpUtil, Msg } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { keys } from '@/api/queryKeys';
import { markLocalInvalidate } from '@/api/invalidationTracker';
import {
  ClientHydrateSchema,
  ClientPageResponseSchema,
  InboundOptionsSchema,
  OnlinesSchema,
  BulkAdjustResultSchema,
  BulkAttachResultSchema,
  BulkCreateResultSchema,
  BulkDeleteResultSchema,
  BulkDetachResultSchema,
  DelDepletedResultSchema,
  type ClientHydrate,
  type ClientRecord,
  type ClientTraffic,
  type ClientsSummary,
  type ClientPageResponse,
  type InboundOption,
  type ExternalLink,
  type BulkAdjustResult,
  type BulkAttachResult,
  type BulkCreateResult,
  type BulkDeleteResult,
  type BulkDetachResult,
} from '@/schemas/client';
import { DefaultsPayloadSchema } from '@/schemas/defaults';

// One row sent to POST /clients/:email/externalLinks.
export type ExternalLinkInput = { kind: 'link' | 'subscription'; value: string; remark: string };

export type { ClientRecord, ClientTraffic, ClientsSummary, InboundOption, ExternalLink };

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } } as const;

interface SubSettings {
  enable: boolean;
  subURI: string;
  subJsonURI: string;
  subJsonEnable: boolean;
  subClashURI: string;
  subClashEnable: boolean;
}

export interface ClientQueryParams {
  page: number;
  pageSize: number;
  search?: string;
  // CSV strings — frontend joins arrays on ',', backend splits the same way.
  filter?: string;
  protocol?: string;
  inbound?: string;
  sort?: string;
  order?: 'ascend' | 'descend';
  expiryFrom?: number;
  expiryTo?: number;
  usageFrom?: number;
  usageTo?: number;
  autoRenew?: 'on' | 'off' | '';
  hasTgId?: 'yes' | 'no' | '';
  hasComment?: 'yes' | 'no' | '';
  group?: string;
}

const DEFAULT_QUERY: ClientQueryParams = { page: 1, pageSize: 25 };
const DEFAULT_SUMMARY: ClientsSummary = {
  total: 0, active: 0, online: [], depleted: [], expiring: [], deactive: [],
};

type ClientStatRow = ClientTraffic & { email?: string };

// Mirror of the server's buildClientsSummary (web/service/client.go). The
// client_stats WS event already carries every client's traffic, so the
// summary card can be recomputed live from it instead of waiting for a list
// refetch — keep the two in lockstep.
export function computeClientsSummary(
  stats: ClientStatRow[],
  onlineSet: Set<string>,
  expireDiffMs: number,
  trafficDiffBytes: number,
): ClientsSummary {
  const now = Date.now();
  const online: string[] = [];
  const depleted: string[] = [];
  const expiring: string[] = [];
  const deactive: string[] = [];
  let active = 0;
  for (const c of stats) {
    const email = c.email;
    if (!email) continue;
    const used = (c.up || 0) + (c.down || 0);
    const total = c.total || 0;
    const exhausted = total > 0 && used >= total;
    const expired = (c.expiryTime || 0) > 0 && (c.expiryTime || 0) <= now;
    if (c.enable && onlineSet.has(email)) online.push(email);
    if (exhausted || expired) { depleted.push(email); continue; }
    if (!c.enable) { deactive.push(email); continue; }
    const nearExpiry = (c.expiryTime || 0) > 0 && (c.expiryTime || 0) - now < expireDiffMs;
    const nearLimit = total > 0 && total - used < trafficDiffBytes;
    if (nearExpiry || nearLimit) expiring.push(email);
    else active += 1;
  }
  return { total: stats.length, active, online, depleted, expiring, deactive };
}

function buildQS(p: ClientQueryParams): string {
  const sp = new URLSearchParams();
  sp.set('page', String(p.page || 1));
  sp.set('pageSize', String(p.pageSize || DEFAULT_QUERY.pageSize));
  if (p.search) sp.set('search', p.search);
  if (p.filter) sp.set('filter', p.filter);
  if (p.protocol) sp.set('protocol', p.protocol);
  if (p.inbound) sp.set('inbound', p.inbound);
  if (p.sort) sp.set('sort', p.sort);
  if (p.order) sp.set('order', p.order);
  if (p.expiryFrom && p.expiryFrom > 0) sp.set('expiryFrom', String(p.expiryFrom));
  if (p.expiryTo && p.expiryTo > 0) sp.set('expiryTo', String(p.expiryTo));
  if (p.usageFrom && p.usageFrom > 0) sp.set('usageFrom', String(p.usageFrom));
  if (p.usageTo && p.usageTo > 0) sp.set('usageTo', String(p.usageTo));
  if (p.autoRenew) sp.set('autoRenew', p.autoRenew);
  if (p.hasTgId) sp.set('hasTgId', p.hasTgId);
  if (p.hasComment) sp.set('hasComment', p.hasComment);
  if (p.group) sp.set('group', p.group);
  return sp.toString();
}

async function fetchClientPage(params: ClientQueryParams): Promise<ClientPageResponse> {
  const qs = buildQS(params);
  const msg = await HttpUtil.get(`/panel/api/clients/list/paged?${qs}`, undefined, { silent: true });
  if (!msg?.success || !msg.obj) throw new Error(msg?.msg || 'Failed to fetch clients');
  const validated = parseMsg(msg, ClientPageResponseSchema, 'clients/list/paged');
  if (!validated.obj) throw new Error('Empty clients response');
  return validated.obj;
}

async function fetchInboundOptions(): Promise<InboundOption[]> {
  const msg = await HttpUtil.get('/panel/api/inbounds/options', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch inbound options');
  const validated = parseMsg(msg, InboundOptionsSchema, 'inbounds/options');
  return Array.isArray(validated.obj) ? validated.obj : [];
}

async function fetchDefaults(): Promise<Record<string, unknown>> {
  const msg = await HttpUtil.post('/panel/api/setting/defaultSettings', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch defaults');
  const validated = parseMsg(msg, DefaultsPayloadSchema, 'setting/defaultSettings');
  return validated.obj || {};
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
        && (prev.inbound ?? '') === (next.inbound ?? '')
        && (prev.sort ?? '') === (next.sort ?? '')
        && (prev.order ?? '') === (next.order ?? '')
        && (prev.expiryFrom ?? 0) === (next.expiryFrom ?? 0)
        && (prev.expiryTo ?? 0) === (next.expiryTo ?? 0)
        && (prev.usageFrom ?? 0) === (next.usageFrom ?? 0)
        && (prev.usageTo ?? 0) === (next.usageTo ?? 0)
        && (prev.autoRenew ?? '') === (next.autoRenew ?? '')
        && (prev.hasTgId ?? '') === (next.hasTgId ?? '')
        && (prev.hasComment ?? '') === (next.hasComment ?? '')
        && (prev.group ?? '') === (next.group ?? '')
      ) return prev;
      return next;
    });
  }, []);

  const listQuery = useQuery({
    queryKey: keys.clients.list(query),
    queryFn: () => fetchClientPage(query),
    staleTime: Infinity,
    // List is sorted/paged server-side, so the WS patch can't add new or
    // re-sort rows; poll the current page to keep it live (pauses when hidden).
    refetchInterval: 5000,
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
      const msg = await HttpUtil.post('/panel/api/clients/onlines', undefined, { silent: true });
      if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch onlines');
      const validated = parseMsg(msg, OnlinesSchema, 'clients/onlines');
      return Array.isArray(validated.obj) ? validated.obj : [];
    },
    staleTime: Infinity,
  });

  const clients = listQuery.data?.items ?? [];
  const total = listQuery.data?.total ?? 0;
  const filtered = listQuery.data?.filtered ?? 0;
  const allGroups = listQuery.data?.groups ?? [];
  const fetched = listQuery.data !== undefined || listQuery.isError;
  const fetchError = listQuery.error ? (listQuery.error as Error).message : '';
  const loading = listQuery.isFetching;
  // Showing kept-previous data for a new key (filter/sort/page) — drives the
  // table overlay so the 5s background poll doesn't flash it.
  const transitioning = listQuery.isPlaceholderData;

  const inbounds = inboundOptionsQuery.data ?? [];
  const onlines = useMemo(() => onlinesQuery.data ?? [], [onlinesQuery.data]);

  const defaults = defaultsQuery.data ?? {};
  const subSettings: SubSettings = useMemo(() => ({
    enable: !!defaults.subEnable,
    subURI: (defaults.subURI as string) || '',
    subJsonURI: (defaults.subJsonURI as string) || '',
    subJsonEnable: !!defaults.subJsonEnable,
    subClashURI: (defaults.subClashURI as string) || '',
    subClashEnable: !!defaults.subClashEnable,
  }), [
    defaults.subEnable,
    defaults.subURI,
    defaults.subJsonURI,
    defaults.subJsonEnable,
    defaults.subClashURI,
    defaults.subClashEnable,
  ]);

  const ipLimitEnable = !!defaults.ipLimitEnable;
  const tgBotEnable = !!defaults.tgBotEnable;
  const expireDiff = ((defaults.expireDiff as number) ?? 0) * 86400000;
  const trafficDiff = ((defaults.trafficDiff as number) ?? 0) * 1073741824;
  const pageSize = (defaults.pageSize as number) ?? 0;

  // Live summary: the client_stats WS event refreshes allClientStats every few
  // seconds, so the top counters track reality without a page refresh. Falls
  // back to the server-computed summary until the first event lands, and keeps
  // the server's authoritative total for the headline count.
  const [allClientStats, setAllClientStats] = useState<ClientStatRow[]>([]);
  const summary = useMemo<ClientsSummary>(() => {
    const serverSummary = listQuery.data?.summary ?? DEFAULT_SUMMARY;
    if (allClientStats.length === 0) return serverSummary;
    const live = computeClientsSummary(allClientStats, new Set(onlines), expireDiff, trafficDiff);
    return { ...live, total: serverSummary.total || live.total };
  }, [allClientStats, onlines, expireDiff, trafficDiff, listQuery.data?.summary]);

  const invalidateAll = useCallback(
    () => {
      markLocalInvalidate();
      setAllClientStats([]);
      return Promise.all([
        queryClient.invalidateQueries({ queryKey: keys.clients.root() }),
        queryClient.invalidateQueries({ queryKey: keys.inbounds.root() }),
        queryClient.invalidateQueries({ queryKey: keys.xray.config() }),
      ]);
    },
    [queryClient],
  );

  const refresh = useCallback(async () => {
    await invalidateAll();
  }, [invalidateAll]);

  const hydrate = useCallback(async (email: string): Promise<ClientHydrate | null> => {
    if (!email) return null;
    const msg = await HttpUtil.get(`/panel/api/clients/get/${encodeURIComponent(email)}`);
    if (!msg?.success || !msg.obj) return null;
    const validated = parseMsg(msg, ClientHydrateSchema, 'clients/get');
    return validated.obj;
  }, []);

  const createMut = useMutation({
    mutationFn: (payload: unknown) =>
      HttpUtil.post('/panel/api/clients/add', payload, JSON_HEADERS),
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const bulkAddToGroupMut = useMutation({
    mutationFn: (body: { emails: string[]; group: string }) =>
      HttpUtil.post('/panel/api/clients/groups/bulkAdd', body, JSON_HEADERS),
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const bulkRemoveFromGroupMut = useMutation({
    mutationFn: (body: { emails: string[] }) =>
      HttpUtil.post('/panel/api/clients/groups/bulkRemove', body, JSON_HEADERS),
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const updateMut = useMutation({
    mutationFn: ({ email, client }: { email: string; client: unknown }) =>
      HttpUtil.post(`/panel/api/clients/update/${encodeURIComponent(email)}`, client, JSON_HEADERS),
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const removeMut = useMutation({
    mutationFn: ({ email, keepTraffic }: { email: string; keepTraffic?: boolean }) => {
      const url = keepTraffic
        ? `/panel/api/clients/del/${encodeURIComponent(email)}?keepTraffic=1`
        : `/panel/api/clients/del/${encodeURIComponent(email)}`;
      return HttpUtil.post(url);
    },
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const bulkDeleteMut = useMutation({
    mutationFn: async (payload: { emails: string[]; keepTraffic?: boolean }): Promise<Msg<BulkDeleteResult>> => {
      const raw = await HttpUtil.post('/panel/api/clients/bulkDel', payload, JSON_HEADERS);
      return parseMsg(raw, BulkDeleteResultSchema, 'clients/bulkDel');
    },
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const bulkCreateMut = useMutation({
    mutationFn: async (payloads: unknown[]): Promise<Msg<BulkCreateResult>> => {
      const raw = await HttpUtil.post('/panel/api/clients/bulkCreate', payloads, JSON_HEADERS);
      return parseMsg(raw, BulkCreateResultSchema, 'clients/bulkCreate');
    },
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const bulkAdjustMut = useMutation({
    mutationFn: async (payload: { emails: string[]; addDays: number; addBytes: number }): Promise<Msg<BulkAdjustResult>> => {
      const raw = await HttpUtil.post('/panel/api/clients/bulkAdjust', payload, JSON_HEADERS);
      return parseMsg(raw, BulkAdjustResultSchema, 'clients/bulkAdjust');
    },
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const attachMut = useMutation({
    mutationFn: ({ email, inboundIds }: { email: string; inboundIds: number[] }) =>
      HttpUtil.post(`/panel/api/clients/${encodeURIComponent(email)}/attach`, { inboundIds }, JSON_HEADERS),
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const setExternalLinksMut = useMutation({
    mutationFn: ({ email, externalLinks }: { email: string; externalLinks: ExternalLinkInput[] }) =>
      HttpUtil.post(`/panel/api/clients/${encodeURIComponent(email)}/externalLinks`, { externalLinks }, JSON_HEADERS),
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const bulkAttachMut = useMutation({
    mutationFn: async (payload: { emails: string[]; inboundIds: number[] }): Promise<Msg<BulkAttachResult>> => {
      const raw = await HttpUtil.post('/panel/api/clients/bulkAttach', payload, JSON_HEADERS);
      return parseMsg(raw, BulkAttachResultSchema, 'clients/bulkAttach');
    },
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const detachMut = useMutation({
    mutationFn: ({ email, inboundIds }: { email: string; inboundIds: number[] }) =>
      HttpUtil.post(`/panel/api/clients/${encodeURIComponent(email)}/detach`, { inboundIds }, JSON_HEADERS),
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });


  const bulkDetachMut = useMutation({
    mutationFn: async (payload: { emails: string[]; inboundIds: number[] }): Promise<Msg<BulkDetachResult>> => {
      const raw = await HttpUtil.post('/panel/api/clients/bulkDetach', payload, JSON_HEADERS);
      return parseMsg(raw, BulkDetachResultSchema, 'clients/bulkDetach');
    },
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const resetTrafficMut = useMutation({
    mutationFn: (email: string) =>
      HttpUtil.post(`/panel/api/clients/resetTraffic/${encodeURIComponent(email)}`),
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const resetAllTrafficsMut = useMutation({
    mutationFn: () => HttpUtil.post('/panel/api/clients/resetAllTraffics'),
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const delDepletedMut = useMutation({
    mutationFn: async () => {
      const raw = await HttpUtil.post('/panel/api/clients/delDepleted');
      return parseMsg(raw, DelDepletedResultSchema, 'clients/delDepleted');
    },
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const delOrphansMut = useMutation({
    mutationFn: async () => {
      const raw = await HttpUtil.post('/panel/api/clients/delOrphans');
      return parseMsg(raw, DelDepletedResultSchema, 'clients/delOrphans');
    },
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const importClientsMut = useMutation({
    mutationFn: async (data: string): Promise<Msg<BulkCreateResult>> => {
      const raw = await HttpUtil.post('/panel/api/clients/import', { data }, JSON_HEADERS);
      return parseMsg(raw, BulkCreateResultSchema, 'clients/import');
    },
    onSuccess: (msg) => { if (msg?.success) invalidateAll(); },
  });

  const create = useCallback((payload: unknown) => createMut.mutateAsync(payload), [createMut]);
  const update = useCallback((email: string, client: unknown) => {
    if (!email) return Promise.resolve(null as unknown as Msg<unknown>);
    return updateMut.mutateAsync({ email, client });
  }, [updateMut]);
  const remove = useCallback((email: string, keepTraffic = false) => {
    if (!email) return Promise.resolve(null as unknown as Msg<unknown>);
    return removeMut.mutateAsync({ email, keepTraffic });
  }, [removeMut]);
  const bulkDelete = useCallback((emails: string[], keepTraffic = false) => {
    if (!Array.isArray(emails) || emails.length === 0) return Promise.resolve(null as unknown as Msg<BulkDeleteResult>);
    return bulkDeleteMut.mutateAsync({ emails, keepTraffic });
  }, [bulkDeleteMut]);
  const bulkCreate = useCallback((payloads: unknown[]) => {
    if (!Array.isArray(payloads) || payloads.length === 0) return Promise.resolve(null as unknown as Msg<BulkCreateResult>);
    return bulkCreateMut.mutateAsync(payloads);
  }, [bulkCreateMut]);
  const bulkAdjust = useCallback((emails: string[], addDays: number, addBytes: number) => {
    if (!Array.isArray(emails) || emails.length === 0) return Promise.resolve(null);
    return bulkAdjustMut.mutateAsync({ emails, addDays, addBytes });
  }, [bulkAdjustMut]);
  const bulkAddToGroup = useCallback((emails: string[], group: string) => {
    if (!Array.isArray(emails) || emails.length === 0) return Promise.resolve(null);
    return bulkAddToGroupMut.mutateAsync({ emails, group });
  }, [bulkAddToGroupMut]);
  const bulkRemoveFromGroup = useCallback((emails: string[]) => {
    if (!Array.isArray(emails) || emails.length === 0) return Promise.resolve(null);
    return bulkRemoveFromGroupMut.mutateAsync({ emails });
  }, [bulkRemoveFromGroupMut]);
  const attach = useCallback((email: string, inboundIds: number[]) => {
    if (!email) return Promise.resolve(null as unknown as Msg<unknown>);
    return attachMut.mutateAsync({ email, inboundIds });
  }, [attachMut]);
  const setExternalLinks = useCallback((email: string, externalLinks: ExternalLinkInput[]) => {
    if (!email) return Promise.resolve(null as unknown as Msg<unknown>);
    return setExternalLinksMut.mutateAsync({ email, externalLinks });
  }, [setExternalLinksMut]);
  const bulkAttach = useCallback((emails: string[], inboundIds: number[]) => {
    if (!Array.isArray(emails) || emails.length === 0) return Promise.resolve(null as unknown as Msg<BulkAttachResult>);
    if (!Array.isArray(inboundIds) || inboundIds.length === 0) return Promise.resolve(null as unknown as Msg<BulkAttachResult>);
    return bulkAttachMut.mutateAsync({ emails, inboundIds });
  }, [bulkAttachMut]);
  const detach = useCallback((email: string, inboundIds: number[]) => {
    if (!email) return Promise.resolve(null as unknown as Msg<unknown>);
    return detachMut.mutateAsync({ email, inboundIds });
  }, [detachMut]);
  const bulkDetach = useCallback((emails: string[], inboundIds: number[]) => {
    if (!Array.isArray(emails) || emails.length === 0) return Promise.resolve(null as unknown as Msg<BulkDetachResult>);
    if (!Array.isArray(inboundIds) || inboundIds.length === 0) return Promise.resolve(null as unknown as Msg<BulkDetachResult>);
    return bulkDetachMut.mutateAsync({ emails, inboundIds });
  }, [bulkDetachMut]);
  const resetTraffic = useCallback((client: ClientRecord) => {
    if (!client?.email) return Promise.resolve(null as unknown as Msg<unknown>);
    return resetTrafficMut.mutateAsync(client.email);
  }, [resetTrafficMut]);
  const resetAllTraffics = useCallback(() => resetAllTrafficsMut.mutateAsync(), [resetAllTrafficsMut]);
  const delDepleted = useCallback(() => delDepletedMut.mutateAsync(), [delDepletedMut]);
  const delOrphans = useCallback(() => delOrphansMut.mutateAsync(), [delOrphansMut]);
  const importClients = useCallback((data: string) => importClientsMut.mutateAsync(data), [importClientsMut]);
  // Fetch the exported clients so the page can show them in a CodeMirror viewer
  // (Copy / Download), rather than triggering an immediate browser download.
  const exportClients = useCallback(async (): Promise<unknown[] | null> => {
    const msg = await HttpUtil.get('/panel/api/clients/export');
    if (!msg?.success) return null;
    return Array.isArray(msg.obj) ? msg.obj : [];
  }, []);

  const setEnable = useCallback(async (client: ClientRecord, enable: boolean) => {
    if (!client?.email) return null;
    const full = await hydrate(client.email);
    const base = full?.client;
    if (!base) return null;
    const payload: Record<string, unknown> = {
      email: base.email,
      subId: base.subId,
      id: base.uuid,
      password: base.password,
      auth: base.auth,
      flow: base.flow || '',
      security: base.security || 'auto',
      totalGB: base.totalGB || 0,
      expiryTime: base.expiryTime || 0,
      limitIp: base.limitIp || 0,
      tgId: Number(base.tgId) || 0,
      reset: Number(base.reset) || 0,
      group: base.group || '',
      comment: base.comment || '',
      enable: !!enable,
    };
    if (base.reverse?.tag) {
      payload.reverse = { tag: base.reverse.tag };
    }
    return update(client.email, payload);
  }, [hydrate, update]);

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
    const p = payload as { clients?: ClientStatRow[] };
    if (!Array.isArray(p.clients) || p.clients.length === 0) return;
    setAllClientStats(p.clients);
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
    allGroups,
    hydrate,
    query,
    setQuery,
    inbounds,
    onlines,
    loading,
    transitioning,
    fetched,
    fetchError,
    subSettings,
    ipLimitEnable,
    tgBotEnable,
    expireDiff,
    trafficDiff,
    pageSize,
    refresh,
    create,
    bulkCreate,
    update,
    remove,
    bulkDelete,
    bulkAdjust,
    bulkAddToGroup,
    bulkRemoveFromGroup,
    attach,
    setExternalLinks,
    bulkAttach,
    detach,
    bulkDetach,
    resetTraffic,
    resetAllTraffics,
    delDepleted,
    delOrphans,
    exportClients,
    importClients,
    setEnable,
    applyTrafficEvent,
    applyClientStatsEvent,
  };
}
