import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { DBInbound } from '@/models/dbinbound.js';
import { Protocols } from '@/models/inbound.js';
import { setDatepicker } from '@/hooks/useDatepicker';
import { keys } from '@/api/queryKeys';

export interface SubSettings {
  enable: boolean;
  subTitle: string;
  subURI: string;
  subJsonURI: string;
  subJsonEnable: boolean;
}

type DBInboundInstance = InstanceType<typeof DBInbound>;

interface ClientRollup {
  clients: number;
  active: string[];
  deactive: string[];
  depleted: string[];
  expiring: string[];
  online: string[];
  comments: Map<string, string>;
}

interface ApiMsg<T = unknown> {
  success?: boolean;
  obj?: T;
  msg?: string;
}

interface DefaultsPayload {
  expireDiff?: number;
  trafficDiff?: number;
  tgBotEnable?: boolean;
  subEnable?: boolean;
  subTitle?: string;
  subURI?: string;
  subJsonURI?: string;
  subJsonEnable?: boolean;
  pageSize?: number;
  remarkModel?: string;
  datepicker?: string;
  ipLimitEnable?: boolean;
}

const TRACKED_PROTOCOLS = [
  Protocols.VMESS,
  Protocols.VLESS,
  Protocols.TROJAN,
  Protocols.SHADOWSOCKS,
  Protocols.HYSTERIA,
];

async function fetchSlimInbounds(): Promise<unknown[]> {
  const msg = await HttpUtil.get('/panel/api/inbounds/list/slim', undefined, { silent: true }) as ApiMsg<unknown[]>;
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch inbounds');
  return Array.isArray(msg.obj) ? msg.obj : [];
}

async function fetchOnlineClients(): Promise<string[]> {
  const msg = await HttpUtil.post('/panel/api/clients/onlines', undefined, { silent: true }) as ApiMsg<string[]>;
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch onlines');
  return Array.isArray(msg.obj) ? msg.obj : [];
}

async function fetchLastOnlineMap(): Promise<Record<string, number>> {
  const msg = await HttpUtil.post('/panel/api/clients/lastOnline', undefined, { silent: true }) as ApiMsg<Record<string, number>>;
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch lastOnline');
  return (msg.obj && typeof msg.obj === 'object') ? msg.obj : {};
}

async function fetchDefaultSettings(): Promise<DefaultsPayload> {
  const msg = await HttpUtil.post('/panel/setting/defaultSettings', undefined, { silent: true }) as ApiMsg<DefaultsPayload>;
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch defaults');
  return (msg.obj as DefaultsPayload) || {};
}

export function useInbounds() {
  const queryClient = useQueryClient();

  const slimQuery = useQuery({
    queryKey: keys.inbounds.slim(),
    queryFn: fetchSlimInbounds,
    staleTime: Infinity,
  });

  const onlinesQuery = useQuery({
    queryKey: keys.clients.onlines(),
    queryFn: fetchOnlineClients,
    staleTime: Infinity,
  });

  const lastOnlineQuery = useQuery({
    queryKey: keys.clients.lastOnline(),
    queryFn: fetchLastOnlineMap,
    staleTime: Infinity,
  });

  const defaultsQuery = useQuery({
    queryKey: keys.settings.defaults(),
    queryFn: fetchDefaultSettings,
    staleTime: Infinity,
  });

  const defaults = defaultsQuery.data ?? {};
  const expireDiff = (defaults.expireDiff ?? 0) * 86400000;
  const trafficDiff = (defaults.trafficDiff ?? 0) * 1073741824;
  const tgBotEnable = !!defaults.tgBotEnable;
  const ipLimitEnable = !!defaults.ipLimitEnable;
  const pageSize = defaults.pageSize ?? 0;
  const remarkModel = defaults.remarkModel || '-ieo';
  const datepicker = (defaults.datepicker as 'gregorian' | 'jalalian') || 'gregorian';

  const subSettings: SubSettings = useMemo(() => ({
    enable: !!defaults.subEnable,
    subTitle: defaults.subTitle || '',
    subURI: defaults.subURI || '',
    subJsonURI: defaults.subJsonURI || '',
    subJsonEnable: !!defaults.subJsonEnable,
  }), [defaults.subEnable, defaults.subTitle, defaults.subURI, defaults.subJsonURI, defaults.subJsonEnable]);

  useEffect(() => {
    if (defaults.datepicker) setDatepicker(datepicker);
  }, [datepicker, defaults.datepicker]);

  const expireDiffRef = useRef(expireDiff);
  expireDiffRef.current = expireDiff;
  const trafficDiffRef = useRef(trafficDiff);
  trafficDiffRef.current = trafficDiff;

  // dbInbounds mirrors the slim query data wrapped as DBInbound instances, but
  // stays mutable so the WS-driven applyClientStatsEvent / applyTrafficEvent
  // can merge per-row updates without invalidating the entire query.
  const [dbInbounds, setDbInbounds] = useState<DBInboundInstance[]>([]);
  const dbInboundsRef = useRef<DBInboundInstance[]>([]);
  dbInboundsRef.current = dbInbounds;

  const [clientCount, setClientCount] = useState<Record<number, ClientRollup>>({});
  const [statsVersion, setStatsVersion] = useState(0);

  const [onlineClients, setOnlineClients] = useState<string[]>([]);
  const onlineClientsRef = useRef<string[]>([]);
  onlineClientsRef.current = onlineClients;

  const [lastOnlineMap, setLastOnlineMap] = useState<Record<string, number>>({});

  const rollupClients = useCallback(
    (dbInbound: DBInboundInstance, inbound: { clients?: { email?: string; enable?: boolean; comment?: string }[] }): ClientRollup => {
      const clientStats = Array.isArray((dbInbound as { clientStats?: unknown }).clientStats)
        ? (dbInbound as unknown as { clientStats: { email: string; total: number; up: number; down: number; expiryTime: number }[] }).clientStats
        : [];
      const allClients = inbound?.clients || [];
      const statsEmails = new Set<string>();
      for (const s of clientStats) {
        if (s && s.email) statsEmails.add(s.email);
      }
      const clients = clientStats.length > 0
        ? allClients.filter((c) => c && c.email && statsEmails.has(c.email))
        : allClients;
      const active: string[] = [];
      const deactive: string[] = [];
      const depleted: string[] = [];
      const expiring: string[] = [];
      const online: string[] = [];
      const comments = new Map<string, string>();
      const now = Date.now();

      if (dbInbound.enable) {
        for (const client of clients) {
          if (client.comment && client.email) comments.set(client.email, client.comment);
          if (client.enable) {
            if (client.email) active.push(client.email);
            if (client.email && onlineClientsRef.current.includes(client.email)) online.push(client.email);
          } else if (client.email) {
            deactive.push(client.email);
          }
        }
        for (const stats of clientStats) {
          const exhausted = stats.total > 0 && stats.up + stats.down >= stats.total;
          const expired = stats.expiryTime > 0 && stats.expiryTime <= now;
          if (expired || exhausted) {
            depleted.push(stats.email);
          } else {
            const expiringSoon =
              (stats.expiryTime > 0 && stats.expiryTime - now < expireDiffRef.current) ||
              (stats.total > 0 && stats.total - (stats.up + stats.down) < trafficDiffRef.current);
            if (expiringSoon) expiring.push(stats.email);
          }
        }
      } else {
        for (const client of clients) {
          if (client.email) deactive.push(client.email);
        }
      }

      return {
        clients: clients.length,
        active,
        deactive,
        depleted,
        expiring,
        online,
        comments,
      };
    },
    [],
  );

  const rebuildClientCount = useCallback(() => {
    const counts: Record<number, ClientRollup> = {};
    for (const dbInbound of dbInboundsRef.current) {
      const parsed = (dbInbound as unknown as { toInbound: () => { clients?: unknown[]; isSSMultiUser?: boolean }; isSS: boolean; protocol: string }).toInbound();
      const protocol = (dbInbound as unknown as { protocol: string }).protocol;
      if (!TRACKED_PROTOCOLS.includes(protocol)) continue;
      const isSS = (dbInbound as unknown as { isSS: boolean }).isSS;
      if (isSS && !parsed.isSSMultiUser) continue;
      counts[(dbInbound as unknown as { id: number }).id] = rollupClients(dbInbound, parsed as { clients?: { email?: string; enable?: boolean; comment?: string }[] });
    }
    setClientCount(counts);
  }, [rollupClients]);

  // Seed dbInbounds + clientCount from the slim query. Runs on first fetch and
  // again every time the query refetches (e.g. invalidate from WS bridge).
  useEffect(() => {
    if (!slimQuery.data) return;
    const next: DBInboundInstance[] = [];
    const counts: Record<number, ClientRollup> = {};
    for (const row of slimQuery.data as { protocol: string; id: number }[]) {
      const dbInbound = new DBInbound(row) as DBInboundInstance;
      const parsed = (dbInbound as unknown as { toInbound: () => { clients?: unknown[]; isSSMultiUser?: boolean } }).toInbound();
      next.push(dbInbound);
      if (TRACKED_PROTOCOLS.includes(row.protocol)) {
        if ((dbInbound as unknown as { isSS: boolean }).isSS && !parsed.isSSMultiUser) continue;
        counts[row.id] = rollupClients(dbInbound, parsed as { clients?: { email?: string; enable?: boolean; comment?: string }[] });
      }
    }
    dbInboundsRef.current = next;
    setDbInbounds(next);
    setClientCount(counts);
  }, [slimQuery.data, rollupClients]);

  useEffect(() => {
    if (onlinesQuery.data) {
      onlineClientsRef.current = onlinesQuery.data;
      setOnlineClients(onlinesQuery.data);
    }
  }, [onlinesQuery.data]);

  useEffect(() => {
    if (lastOnlineQuery.data) setLastOnlineMap(lastOnlineQuery.data);
  }, [lastOnlineQuery.data]);

  const fetched = slimQuery.data !== undefined && defaultsQuery.data !== undefined;

  const refresh = useCallback(async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: keys.inbounds.slim() }),
      queryClient.invalidateQueries({ queryKey: keys.clients.onlines() }),
      queryClient.invalidateQueries({ queryKey: keys.clients.lastOnline() }),
    ]);
  }, [queryClient]);

  // hydrateInbound fetches the full inbound (including settings.clients with
  // uuid/password/flow/etc.) and swaps it into the cached list. Use this
  // before opening edit / info / qr / export / clone flows — refresh() loads
  // the slim list which doesn't carry per-client secrets.
  const hydrateInbound = useCallback(async (id: number) => {
    const msg = await HttpUtil.get(`/panel/api/inbounds/get/${id}`);
    if (!msg?.success || !msg.obj) return null;
    const full = msg.obj as { id: number; protocol: string };
    const dbInbound = new DBInbound(full) as DBInboundInstance;
    setDbInbounds((prev) => {
      const next = prev.map((row) => (
        (row as unknown as { id: number }).id === id ? dbInbound : row
      ));
      dbInboundsRef.current = next;
      return next;
    });
    rebuildClientCount();
    return dbInbound;
  }, [rebuildClientCount]);

  const applyTrafficEvent = useCallback(
    (payload: unknown) => {
      if (!payload || typeof payload !== 'object') return;
      const p = payload as { onlineClients?: string[]; lastOnlineMap?: Record<string, number> };
      if (Array.isArray(p.onlineClients)) {
        onlineClientsRef.current = p.onlineClients;
        setOnlineClients(p.onlineClients);
      }
      if (p.lastOnlineMap && typeof p.lastOnlineMap === 'object') {
        setLastOnlineMap((prev) => ({ ...prev, ...p.lastOnlineMap! }));
      }
      rebuildClientCount();
    },
    [rebuildClientCount],
  );

  const applyClientStatsEvent = useCallback(
    (payload: unknown) => {
      if (!payload || typeof payload !== 'object') return;
      const p = payload as {
        inbounds?: { id: number; up?: number; down?: number; total?: number; enable?: boolean }[];
        clients?: { email: string; up?: number; down?: number; total?: number; expiryTime?: number; enable?: boolean }[];
      };
      let touched = false;

      if (Array.isArray(p.inbounds) && p.inbounds.length > 0) {
        const byId = new Map<number, { id: number; up?: number; down?: number; total?: number; enable?: boolean }>();
        for (const row of p.inbounds) {
          if (row && row.id != null) byId.set(row.id, row);
        }
        for (const ib of dbInboundsRef.current) {
          const upd = byId.get((ib as unknown as { id: number }).id);
          if (!upd) continue;
          const ibRec = ib as unknown as { up: number; down: number; total: number; enable: boolean };
          if (typeof upd.up === 'number') ibRec.up = upd.up;
          if (typeof upd.down === 'number') ibRec.down = upd.down;
          if (typeof upd.total === 'number') ibRec.total = upd.total;
          if (typeof upd.enable === 'boolean') ibRec.enable = upd.enable;
          touched = true;
        }
      }

      if (Array.isArray(p.clients) && p.clients.length > 0) {
        const byEmail = new Map<string, { email: string; up?: number; down?: number; total?: number; expiryTime?: number; enable?: boolean }>();
        for (const row of p.clients) {
          if (row && row.email) byEmail.set(row.email, row);
        }
        for (const ib of dbInboundsRef.current) {
          const stats = (ib as unknown as { clientStats: { email: string; up: number; down: number; total: number; expiryTime: number; enable: boolean }[] }).clientStats;
          if (!Array.isArray(stats)) continue;
          for (let i = 0; i < stats.length; i++) {
            const stat = stats[i];
            const upd = byEmail.get(stat.email);
            if (!upd) continue;
            if (typeof upd.up === 'number') stat.up = upd.up;
            if (typeof upd.down === 'number') stat.down = upd.down;
            if (typeof upd.total === 'number') stat.total = upd.total;
            if (typeof upd.expiryTime === 'number') stat.expiryTime = upd.expiryTime;
            if (typeof upd.enable === 'boolean') stat.enable = upd.enable;
            touched = true;
          }
        }
      }

      if (touched) {
        setStatsVersion((v) => v + 1);
        setDbInbounds((prev) => {
          const next = [...prev];
          dbInboundsRef.current = next;
          return next;
        });
        rebuildClientCount();
      }
    },
    [rebuildClientCount],
  );

  const totals = useMemo(() => {
    let up = 0;
    let down = 0;
    for (const ib of dbInbounds) {
      const rec = ib as unknown as { up?: number; down?: number };
      up += rec.up || 0;
      down += rec.down || 0;
    }
    return { up, down };
  }, [dbInbounds]);

  return {
    fetched,
    dbInbounds,
    clientCount,
    onlineClients,
    lastOnlineMap,
    statsVersion,
    totals,
    expireDiff,
    trafficDiff,
    subSettings,
    remarkModel,
    datepicker,
    tgBotEnable,
    ipLimitEnable,
    pageSize,
    refresh,
    hydrateInbound,
    applyTrafficEvent,
    applyClientStatsEvent,
  };
}
