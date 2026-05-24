import { useCallback, useMemo, useRef, useState } from 'react';

import { HttpUtil } from '@/utils';
import { DBInbound } from '@/models/dbinbound.js';
import { Protocols } from '@/models/inbound.js';
import { setDatepicker } from '@/hooks/useDatepicker';

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

const TRACKED_PROTOCOLS = [
  Protocols.VMESS,
  Protocols.VLESS,
  Protocols.TROJAN,
  Protocols.SHADOWSOCKS,
  Protocols.HYSTERIA,
];

export function useInbounds() {
  const [fetched, setFetched] = useState(false);
  const refreshingRef = useRef(false);
  const [dbInbounds, setDbInbounds] = useState<DBInboundInstance[]>([]);
  const dbInboundsRef = useRef<DBInboundInstance[]>([]);
  dbInboundsRef.current = dbInbounds;

  const [clientCount, setClientCount] = useState<Record<number, ClientRollup>>({});
  const [onlineClients, setOnlineClients] = useState<string[]>([]);
  const onlineClientsRef = useRef<string[]>([]);
  onlineClientsRef.current = onlineClients;

  const [lastOnlineMap, setLastOnlineMap] = useState<Record<string, number>>({});
  const [statsVersion, setStatsVersion] = useState(0);

  const [expireDiff, setExpireDiff] = useState(0);
  const expireDiffRef = useRef(0);
  expireDiffRef.current = expireDiff;
  const [trafficDiff, setTrafficDiff] = useState(0);
  const trafficDiffRef = useRef(0);
  trafficDiffRef.current = trafficDiff;

  const [subSettings, setSubSettings] = useState<SubSettings>({
    enable: false,
    subTitle: '',
    subURI: '',
    subJsonURI: '',
    subJsonEnable: false,
  });
  const [remarkModel, setRemarkModel] = useState('-ieo');
  const [datepicker, setDatepickerState] = useState('gregorian');
  const [tgBotEnable, setTgBotEnable] = useState(false);
  const [ipLimitEnable, setIpLimitEnable] = useState(false);
  const [pageSize, setPageSize] = useState(0);

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

  const setInbounds = useCallback(
    (rows: unknown[]) => {
      const next: DBInboundInstance[] = [];
      const counts: Record<number, ClientRollup> = {};
      for (const row of rows as { protocol: string; id: number }[]) {
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
      setFetched(true);
    },
    [rollupClients],
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

  const fetchOnlineUsers = useCallback(async () => {
    const msg = await HttpUtil.post('/panel/api/clients/onlines');
    if (msg?.success) {
      const list = (msg.obj || []) as string[];
      onlineClientsRef.current = list;
      setOnlineClients(list);
    }
  }, []);

  const fetchLastOnlineMap = useCallback(async () => {
    const msg = await HttpUtil.post('/panel/api/clients/lastOnline');
    if (msg?.success && msg.obj) {
      setLastOnlineMap(msg.obj as Record<string, number>);
    }
  }, []);

  const fetchDefaultSettings = useCallback(async () => {
    const msg = await HttpUtil.post('/panel/setting/defaultSettings');
    if (!msg?.success) return;
    const s = (msg.obj || {}) as Record<string, unknown>;
    setExpireDiff((s.expireDiff as number ?? 0) * 86400000);
    setTrafficDiff((s.trafficDiff as number ?? 0) * 1073741824);
    setTgBotEnable(!!s.tgBotEnable);
    setSubSettings({
      enable: !!s.subEnable,
      subTitle: (s.subTitle as string) || '',
      subURI: (s.subURI as string) || '',
      subJsonURI: (s.subJsonURI as string) || '',
      subJsonEnable: !!s.subJsonEnable,
    });
    setPageSize((s.pageSize as number) ?? 0);
    setRemarkModel((s.remarkModel as string) || '-ieo');
    const dp = ((s.datepicker as string) || 'gregorian') as 'gregorian' | 'jalalian';
    setDatepickerState(dp);
    setDatepicker(dp);
    setIpLimitEnable(!!s.ipLimitEnable);
  }, []);

  const refresh = useCallback(async () => {
    if (refreshingRef.current) return;
    refreshingRef.current = true;
    try {
      const msg = await HttpUtil.get('/panel/api/inbounds/list/slim');
      if (!msg?.success) return;
      await fetchLastOnlineMap();
      await fetchOnlineUsers();
      setInbounds(Array.isArray(msg.obj) ? msg.obj : []);
    } finally {
      window.setTimeout(() => { refreshingRef.current = false; }, 500);
    }
  }, [fetchLastOnlineMap, fetchOnlineUsers, setInbounds]);

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

  const applyInvalidate = useCallback(
    (payload: unknown) => {
      if (!payload || typeof payload !== 'object') return;
      const p = payload as { type?: string };
      if (p.type === 'inbounds') {
        refresh();
      }
    },
    [refresh],
  );

  const applyInboundsEvent = useCallback(
    (payload: unknown) => {
      if (!Array.isArray(payload)) return;
      setInbounds(payload);
    },
    [setInbounds],
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
    fetchDefaultSettings,
    applyTrafficEvent,
    applyClientStatsEvent,
    applyInvalidate,
    applyInboundsEvent,
  };
}
