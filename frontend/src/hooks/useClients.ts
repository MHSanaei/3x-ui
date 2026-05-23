import { useCallback, useEffect, useRef, useState } from 'react';
import { HttpUtil } from '@/utils';

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

export function useClients() {
  const [clients, setClients] = useState<ClientRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [filtered, setFiltered] = useState(0);
  const [summary, setSummary] = useState<ClientsSummary>({
    total: 0, active: 0, online: [], depleted: [], expiring: [], deactive: [],
  });
  const [inbounds, setInbounds] = useState<InboundOption[]>([]);
  const [onlines, setOnlines] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [fetched, setFetched] = useState(false);
  const [query, setQueryState] = useState<ClientQueryParams>(DEFAULT_QUERY);
  // Shallow-compare against the previous query so callers can pass a fresh
  // object on every render (the common React pattern) without triggering a
  // re-fetch when nothing actually changed.
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
  const [subSettings, setSubSettings] = useState<SubSettings>({
    enable: false, subURI: '', subJsonURI: '', subJsonEnable: false,
  });
  const [ipLimitEnable, setIpLimitEnable] = useState(false);
  const [tgBotEnable, setTgBotEnable] = useState(false);
  const [expireDiff, setExpireDiff] = useState(0);
  const [trafficDiff, setTrafficDiff] = useState(0);
  const [pageSize, setPageSize] = useState(0);

  const clientsRef = useRef<ClientRecord[]>([]);
  const queryRef = useRef<ClientQueryParams>(query);
  const invalidateTimerRef = useRef<number | null>(null);

  useEffect(() => { clientsRef.current = clients; }, [clients]);
  useEffect(() => { queryRef.current = query; }, [query]);

  const buildQS = (p: ClientQueryParams) => {
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
  };

  const refresh = useCallback(async (override?: ClientQueryParams) => {
    setLoading(true);
    try {
      const params = override ?? queryRef.current;
      const qs = buildQS(params);
      const msg = await HttpUtil.get(`/panel/api/clients/list/paged?${qs}`) as ApiMsg<ClientPageResponse>;
      if (msg?.success && msg.obj) {
        setClients(Array.isArray(msg.obj.items) ? msg.obj.items : []);
        setTotal(msg.obj.total ?? 0);
        setFiltered(msg.obj.filtered ?? 0);
        if (msg.obj.summary) setSummary(msg.obj.summary);
      }
      setFetched(true);
    } finally {
      setLoading(false);
    }
  }, []);

  // Inbound options are picker-shaped and don't depend on the clients query —
  // fetch them once on mount instead of every refresh.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      const msg = await HttpUtil.get('/panel/api/inbounds/options') as ApiMsg<InboundOption[]>;
      if (cancelled) return;
      if (msg?.success) setInbounds(Array.isArray(msg.obj) ? msg.obj : []);
    })();
    return () => { cancelled = true; };
  }, []);

  const fetchSubSettings = useCallback(async () => {
    const msg = await HttpUtil.post('/panel/setting/defaultSettings') as ApiMsg<Record<string, unknown>>;
    if (!msg?.success) return;
    const s = msg.obj || {};
    setSubSettings({
      enable: !!s.subEnable,
      subURI: (s.subURI as string) || '',
      subJsonURI: (s.subJsonURI as string) || '',
      subJsonEnable: !!s.subJsonEnable,
    });
    setIpLimitEnable(!!s.ipLimitEnable);
    setTgBotEnable(!!s.tgBotEnable);
    setExpireDiff(((s.expireDiff as number) ?? 0) * 86400000);
    setTrafficDiff(((s.trafficDiff as number) ?? 0) * 1073741824);
    setPageSize((s.pageSize as number) ?? 0);
  }, []);

  // hydrate fetches the full client record (uuid, password, flow, ...) for a
  // single email. The paged list endpoint omits these to keep the row payload
  // tiny; edit / info / qr / link modals call this to get a complete record
  // before opening.
  const hydrate = useCallback(async (email: string): Promise<{ client: ClientRecord; inboundIds: number[] } | null> => {
    if (!email) return null;
    const msg = await HttpUtil.get(`/panel/api/clients/get/${encodeURIComponent(email)}`) as ApiMsg<{ client: ClientRecord; inboundIds: number[] }>;
    if (!msg?.success || !msg.obj) return null;
    return msg.obj;
  }, []);

  const create = useCallback(async (payload: unknown) => {
    const msg = await HttpUtil.post('/panel/api/clients/add', payload, JSON_HEADERS) as ApiMsg;
    if (msg?.success) await refresh();
    return msg;
  }, [refresh]);

  const update = useCallback(async (email: string, client: unknown) => {
    if (!email) return null;
    const encoded = encodeURIComponent(email);
    const msg = await HttpUtil.post(`/panel/api/clients/update/${encoded}`, client, JSON_HEADERS) as ApiMsg;
    if (msg?.success) await refresh();
    return msg;
  }, [refresh]);

  const remove = useCallback(async (email: string, keepTraffic = false) => {
    if (!email) return null;
    const encoded = encodeURIComponent(email);
    const url = keepTraffic
      ? `/panel/api/clients/del/${encoded}?keepTraffic=1`
      : `/panel/api/clients/del/${encoded}`;
    const msg = await HttpUtil.post(url) as ApiMsg;
    if (msg?.success) await refresh();
    return msg;
  }, [refresh]);

  const removeMany = useCallback(async (emails: string[], keepTraffic = false) => {
    if (!Array.isArray(emails) || emails.length === 0) return [];
    const suffix = keepTraffic ? '?keepTraffic=1' : '';
    const results = await Promise.all(emails.map((email) => {
      const url = `/panel/api/clients/del/${encodeURIComponent(email)}${suffix}`;
      return HttpUtil.post(url, undefined, { silent: true }) as Promise<ApiMsg>;
    }));
    await refresh();
    return results;
  }, [refresh]);

  const bulkAdjust = useCallback(async (emails: string[], addDays: number, addBytes: number) => {
    if (!Array.isArray(emails) || emails.length === 0) return null;
    const msg = await HttpUtil.post(
      '/panel/api/clients/bulkAdjust',
      { emails, addDays, addBytes },
      JSON_HEADERS,
    ) as ApiMsg<{ adjusted: number; skipped?: { email: string; reason: string }[] }>;
    if (msg?.success) await refresh();
    return msg;
  }, [refresh]);

  const attach = useCallback(async (email: string, inboundIds: number[]) => {
    if (!email) return null;
    const encoded = encodeURIComponent(email);
    const msg = await HttpUtil.post(`/panel/api/clients/${encoded}/attach`, { inboundIds }, JSON_HEADERS) as ApiMsg;
    if (msg?.success) await refresh();
    return msg;
  }, [refresh]);

  const detach = useCallback(async (email: string, inboundIds: number[]) => {
    if (!email) return null;
    const encoded = encodeURIComponent(email);
    const msg = await HttpUtil.post(`/panel/api/clients/${encoded}/detach`, { inboundIds }, JSON_HEADERS) as ApiMsg;
    if (msg?.success) await refresh();
    return msg;
  }, [refresh]);

  const resetTraffic = useCallback(async (client: ClientRecord) => {
    if (!client?.email) return null;
    const url = `/panel/api/clients/resetTraffic/${encodeURIComponent(client.email)}`;
    const msg = await HttpUtil.post(url) as ApiMsg;
    if (msg?.success) await refresh();
    return msg;
  }, [refresh]);

  const resetAllTraffics = useCallback(async () => {
    const msg = await HttpUtil.post('/panel/api/clients/resetAllTraffics') as ApiMsg;
    if (msg?.success) await refresh();
    return msg;
  }, [refresh]);

  const delDepleted = useCallback(async () => {
    const msg = await HttpUtil.post('/panel/api/clients/delDepleted') as ApiMsg<{ deleted?: number }>;
    if (msg?.success) await refresh();
    return msg;
  }, [refresh]);

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

  const applyTrafficEvent = useCallback((payload: unknown) => {
    if (!payload || typeof payload !== 'object') return;
    const p = payload as { onlineClients?: string[] };
    if (Array.isArray(p.onlineClients)) {
      setOnlines(p.onlineClients);
    }
  }, []);

  const applyClientStatsEvent = useCallback((payload: unknown) => {
    if (!payload || typeof payload !== 'object') return;
    const p = payload as { clients?: ClientTraffic[] & { email?: string }[] };
    if (!Array.isArray(p.clients) || p.clients.length === 0) return;
    const byEmail = new Map<string, ClientTraffic>();
    for (const row of p.clients as (ClientTraffic & { email?: string })[]) {
      if (row && row.email) byEmail.set(row.email, row);
    }
    const cur = clientsRef.current || [];
    let touched = false;
    const next = cur.slice();
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
    if (touched) setClients(next);
  }, []);

  const applyInvalidate = useCallback((payload: unknown) => {
    if (!payload || typeof payload !== 'object') return;
    const p = payload as { type?: string };
    if (p.type !== 'inbounds' && p.type !== 'clients') return;
    if (invalidateTimerRef.current != null) clearTimeout(invalidateTimerRef.current);
    invalidateTimerRef.current = window.setTimeout(() => {
      invalidateTimerRef.current = null;
      refresh();
    }, 200);
  }, [refresh]);

  useEffect(() => {
    Promise.all([refresh(query), fetchSubSettings()]);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query, fetchSubSettings]);

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
    applyInvalidate,
  };
}
