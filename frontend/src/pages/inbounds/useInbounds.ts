import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { DBInbound, coerceInboundJsonField } from '@/models/dbinbound';
import type { ClientStats, DBInboundInit } from '@/models/dbinbound';
import { Protocols } from '@/schemas/primitives';
import { isSSMultiUser } from '@/lib/xray/protocol-capabilities';
import { setDatepicker } from '@/hooks/useDatepicker';
import { keys } from '@/api/queryKeys';
import { SlimInboundListSchema, LastOnlineMapSchema, InboundDetailSchema } from '@/schemas/inbound';
import { OnlinesSchema, OnlineByNodeSchema, ActiveInboundsByNodeSchema } from '@/schemas/client';
import { DefaultsPayloadSchema, type DefaultsPayload } from '@/schemas/defaults';

import type { InboundSpeedEntry } from './list/types';
import { TRAFFIC_POLL_INTERVAL_S } from '@/lib/traffic/poll-interval';

export interface SubSettings {
  enable: boolean;
  subTitle: string;
  subURI: string;
  subJsonURI: string;
  subJsonEnable: boolean;
  // Configured public host (Sub Domain, else Web Domain) used as the share/QR
  // link host when the panel is reached on a loopback address. Empty if neither
  // is set.
  publicHost: string;
}

type DBInboundInstance = InstanceType<typeof DBInbound>;

// Speed is delta-derived, so it can't be recomputed until the first poll after
// mount; navigating away and back would otherwise blank the column for up to one
// poll. Cache the last speed map across mounts (module scope) and reseed from it
// while recent, so returning to the page shows the last throughput immediately
// and the next poll refreshes it.
const SPEED_CACHE_TTL_MS = 15000;
let inboundSpeedCache: { at: number; data: Record<number, InboundSpeedEntry> } = {
  at: 0,
  data: {},
};

interface TrafficDelta {
  Tag: string;
  Up: number;
  Down: number;
  IsInbound?: boolean;
}

interface ClientRollup {
  clients: number;
  active: string[];
  deactive: string[];
  depleted: string[];
  expiring: string[];
  online: string[];
  comments: Map<string, string>;
}

const TRACKED_PROTOCOLS: readonly string[] = [
  Protocols.VMESS,
  Protocols.VLESS,
  Protocols.TROJAN,
  Protocols.SHADOWSOCKS,
  Protocols.HYSTERIA,
  Protocols.WIREGUARD,
  Protocols.MTPROTO,
  Protocols.AMNEZIAWG,
  Protocols.TUIC,
];

async function fetchSlimInbounds(): Promise<unknown[]> {
  const msg = await HttpUtil.get('/panel/api/inbounds/list/slim', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch inbounds');
  const validated = parseMsg(msg, SlimInboundListSchema, 'inbounds/list/slim');
  return Array.isArray(validated.obj) ? validated.obj : [];
}

async function fetchOnlineClients(): Promise<string[]> {
  const msg = await HttpUtil.post('/panel/api/clients/onlines', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch onlines');
  const validated = parseMsg(msg, OnlinesSchema, 'clients/onlines');
  return Array.isArray(validated.obj) ? validated.obj : [];
}

// Online emails grouped by the panelGuid of the node that physically hosts each
// client, used to scope the per-inbound online rollup so a client online on one
// node is not shown online on every node's inbounds — and a client on a
// sub-node is attributed to that sub-node, not the node it syncs through (#4983).
async function fetchOnlineClientsByGuid(): Promise<Record<string, string[]>> {
  const msg = await HttpUtil.post('/panel/api/clients/onlinesByGuid', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch onlinesByGuid');
  const validated = parseMsg(msg, OnlineByNodeSchema, 'clients/onlinesByGuid');
  return validated.obj && typeof validated.obj === 'object'
    ? (validated.obj as Record<string, string[]>)
    : {};
}

// Inbound tags that carried traffic recently, grouped by node (local = key 0).
// Pairs with the per-node online map so a client attached to several inbounds
// is only marked online on the ones that actually moved bytes — Xray's
// user-level stat can't attribute traffic to a single inbound on its own.
async function fetchActiveInboundsByNode(): Promise<Record<string, string[]>> {
  const msg = await HttpUtil.post('/panel/api/clients/activeInbounds', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch activeInbounds');
  const validated = parseMsg(msg, ActiveInboundsByNodeSchema, 'clients/activeInbounds');
  return validated.obj && typeof validated.obj === 'object'
    ? (validated.obj as Record<string, string[]>)
    : {};
}

function toGuidOnlineMap(data: Record<string, string[]>): Map<string, Set<string>> {
  const map = new Map<string, Set<string>>();
  for (const [key, emails] of Object.entries(data)) {
    if (!Array.isArray(emails)) continue;
    map.set(key, new Set(emails));
  }
  return map;
}

async function fetchLastOnlineMap(): Promise<Record<string, number>> {
  const msg = await HttpUtil.post('/panel/api/clients/lastOnline', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch lastOnline');
  const validated = parseMsg(msg, LastOnlineMapSchema, 'clients/lastOnline');
  return validated.obj && typeof validated.obj === 'object' ? validated.obj : {};
}

async function fetchDefaultSettings(): Promise<DefaultsPayload> {
  const msg = await HttpUtil.post('/panel/api/setting/defaultSettings', undefined, {
    silent: true,
  });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch defaults');
  const validated = parseMsg(msg, DefaultsPayloadSchema, 'setting/defaultSettings');
  return validated.obj ?? {};
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

  const onlinesByGuidQuery = useQuery({
    queryKey: keys.clients.onlinesByGuid(),
    queryFn: fetchOnlineClientsByGuid,
    staleTime: Infinity,
  });

  const activeInboundsQuery = useQuery({
    queryKey: keys.clients.activeInbounds(),
    queryFn: fetchActiveInboundsByNode,
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
  const datepicker = (defaults.datepicker as 'gregorian' | 'jalalian') || 'gregorian';

  const subSettings: SubSettings = useMemo(
    () => ({
      enable: !!defaults.subEnable,
      subTitle: defaults.subTitle || '',
      subURI: defaults.subURI || '',
      subJsonURI: defaults.subJsonURI || '',
      subJsonEnable: !!defaults.subJsonEnable,
      publicHost: defaults.subDomain || defaults.webDomain || '',
    }),
    [
      defaults.subEnable,
      defaults.subTitle,
      defaults.subURI,
      defaults.subJsonURI,
      defaults.subJsonEnable,
      defaults.subDomain,
      defaults.webDomain,
    ],
  );

  useEffect(() => {
    if (defaults.datepicker) setDatepicker(datepicker);
  }, [datepicker, defaults.datepicker]);

  // dbInbounds mirrors the slim query data wrapped as DBInbound instances. The
  // WS handlers rebuild only the rows they touch, so no refetch is needed.
  const [dbInbounds, setDbInbounds] = useState<DBInboundInstance[]>([]);
  const dbInboundsRef = useRef<DBInboundInstance[]>([]);
  useEffect(() => {
    dbInboundsRef.current = dbInbounds;
  });

  const [inboundSpeed, setInboundSpeed] = useState<Record<number, InboundSpeedEntry>>(() =>
    Date.now() - inboundSpeedCache.at < SPEED_CACHE_TTL_MS ? inboundSpeedCache.data : {},
  );
  useEffect(() => {
    inboundSpeedCache = { at: Date.now(), data: inboundSpeed };
  }, [inboundSpeed]);

  const [onlineClients, setOnlineClients] = useState<string[]>([]);

  // Online emails keyed by the hosting node's panelGuid. The rollup reads this
  // so each inbound only counts clients online on the node that physically
  // hosts it, attributing a sub-node's clients to that sub-node (#4983).
  const [onlineByGuid, setOnlineByGuid] = useState<Map<string, Set<string>>>(() => new Map());

  // Recently-active inbound tags keyed by the hosting node's panelGuid. A GUID
  // missing from this map means "no per-inbound activity reported" (e.g. remote
  // nodes), so the rollup leaves that node's inbounds ungated and falls back to
  // the email signal. A present GUID gates: a client only counts online on an
  // inbound whose tag carried traffic this window.
  const [activeByGuid, setActiveByGuid] = useState<Map<string, Set<string>>>(() => new Map());

  const [lastOnlineMap, setLastOnlineMap] = useState<Record<string, number>>({});

  const rollupClients = useCallback(
    (
      dbInbound: DBInboundInstance,
      inbound: { clients?: { email?: string; enable?: boolean; comment?: string }[] },
    ): ClientRollup => {
      const clientStats = Array.isArray((dbInbound as { clientStats?: unknown }).clientStats)
        ? (
            dbInbound as unknown as {
              clientStats: {
                email: string;
                total: number;
                up: number;
                down: number;
                expiryTime: number;
              }[];
            }
          ).clientStats
        : [];
      const clients = inbound?.clients || [];
      const active: string[] = [];
      const deactive: string[] = [];
      const depleted: string[] = [];
      const expiring: string[] = [];
      const online: string[] = [];
      const comments = new Map<string, string>();
      const now = Date.now();

      // Attribution key: the GUID of the node that physically hosts this
      // inbound. Local inbounds carry the panel's own GUID (filled server-side);
      // a node-managed inbound carries its origin node's GUID, or falls back to
      // the master-local synthetic id for an old-build node without one (#4983).
      const guid =
        dbInbound.originNodeGuid || (dbInbound.nodeId != null ? `node:${dbInbound.nodeId}` : '');
      const nodeOnline = onlineByGuid.get(guid);
      // A node absent from the active map reports no per-inbound activity, so
      // leave its inbounds ungated. When present, only mark a client online on
      // this inbound if its tag actually carried traffic — that's what stops a
      // multi-inbound client lighting up every inbound it's attached to.
      const activeForNode = activeByGuid.get(guid);
      const inboundActive =
        activeForNode === undefined || !dbInbound.tag || activeForNode.has(dbInbound.tag);

      if (dbInbound.enable) {
        const statsByEmail = new Map<
          string,
          { email: string; total: number; up: number; down: number; expiryTime: number }
        >();
        for (const stats of clientStats) {
          if (stats.email) statsByEmail.set(stats.email.toLowerCase(), stats);
        }
        for (const client of clients) {
          if (client.comment && client.email) comments.set(client.email, client.comment);
          if (!client.email) continue;
          const stats = statsByEmail.get(client.email.toLowerCase());
          const exhausted =
            stats != null && stats.total > 0 && stats.up + stats.down >= stats.total;
          const expired = stats != null && stats.expiryTime > 0 && stats.expiryTime <= now;
          if (expired || exhausted) {
            depleted.push(client.email);
            continue;
          }
          if (!client.enable) {
            deactive.push(client.email);
            continue;
          }
          active.push(client.email);
          if (inboundActive && nodeOnline?.has(client.email)) online.push(client.email);
          if (stats) {
            const expiringSoon =
              (stats.expiryTime > 0 && stats.expiryTime - now < expireDiff) ||
              (stats.total > 0 && stats.total - (stats.up + stats.down) < trafficDiff);
            if (expiringSoon) expiring.push(client.email);
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
    [onlineByGuid, activeByGuid, expireDiff, trafficDiff],
  );

  // Every write to a DBInbound row also replaces the dbInbounds array, so this
  // recomputes on both a refetch and a WS-merged stats update.
  const clientCount = useMemo(() => {
    const counts: Record<number, ClientRollup> = {};
    for (const dbInbound of dbInbounds) {
      const protocol = dbInbound.protocol;
      if (!TRACKED_PROTOCOLS.includes(protocol)) continue;
      const settings = coerceInboundJsonField(dbInbound.settings) as {
        method?: string;
        clients?: Array<{ email?: string; enable?: boolean; comment?: string }>;
      };
      if (protocol === Protocols.SHADOWSOCKS && !isSSMultiUser({ protocol, settings })) continue;
      counts[dbInbound.id] = rollupClients(dbInbound, { clients: settings.clients });
    }
    return counts;
  }, [dbInbounds, rollupClients]);

  // Adopting fetched data during render (rather than in an effect) keeps the
  // list from painting one frame of the previous data after a refetch.
  const [syncedSlim, setSyncedSlim] = useState<unknown>();
  if (slimQuery.data && slimQuery.data !== syncedSlim) {
    setSyncedSlim(slimQuery.data);
    setDbInbounds(
      (slimQuery.data as { protocol: string; id: number }[]).map(
        (row) => new DBInbound(row) as DBInboundInstance,
      ),
    );
  }

  const [syncedOnlines, setSyncedOnlines] = useState<unknown>();
  if (onlinesQuery.data && onlinesQuery.data !== syncedOnlines) {
    setSyncedOnlines(onlinesQuery.data);
    setOnlineClients(onlinesQuery.data);
  }

  const [syncedOnlinesByGuid, setSyncedOnlinesByGuid] = useState<unknown>();
  if (onlinesByGuidQuery.data && onlinesByGuidQuery.data !== syncedOnlinesByGuid) {
    setSyncedOnlinesByGuid(onlinesByGuidQuery.data);
    setOnlineByGuid(toGuidOnlineMap(onlinesByGuidQuery.data));
  }

  const [syncedActiveInbounds, setSyncedActiveInbounds] = useState<unknown>();
  if (activeInboundsQuery.data && activeInboundsQuery.data !== syncedActiveInbounds) {
    setSyncedActiveInbounds(activeInboundsQuery.data);
    setActiveByGuid(toGuidOnlineMap(activeInboundsQuery.data));
  }

  const [syncedLastOnline, setSyncedLastOnline] = useState<unknown>();
  if (lastOnlineQuery.data && lastOnlineQuery.data !== syncedLastOnline) {
    setSyncedLastOnline(lastOnlineQuery.data);
    setLastOnlineMap(lastOnlineQuery.data);
  }

  const fetched =
    (slimQuery.data !== undefined || slimQuery.isError) &&
    (defaultsQuery.data !== undefined || defaultsQuery.isError);
  const fetchErrorSource = slimQuery.error || defaultsQuery.error;
  const fetchError = fetchErrorSource ? (fetchErrorSource as Error).message : '';

  const refresh = useCallback(async () => {
    // Invalidate at the inbounds root so both `slim` (this page's list)
    // and `options` (the Clients page's inbound picker) refetch. Without
    // the options bucket, a freshly-created inbound stays invisible in
    // the client add/edit modal until a full page reload. The xray config
    // response carries inboundTags for the routing-rule tag picker, so it
    // needs invalidating too or that list stays stale until a hard refresh.
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: keys.inbounds.root() }),
      queryClient.invalidateQueries({ queryKey: keys.clients.onlines() }),
      queryClient.invalidateQueries({ queryKey: keys.clients.onlinesByGuid() }),
      queryClient.invalidateQueries({ queryKey: keys.clients.activeInbounds() }),
      queryClient.invalidateQueries({ queryKey: keys.clients.lastOnline() }),
      queryClient.invalidateQueries({ queryKey: keys.xray.config() }),
    ]);
  }, [queryClient]);

  // hydrateInbound fetches the full inbound (including settings.clients with
  // uuid/password/flow/etc.) and swaps it into the cached list. Use this
  // before opening edit / info / qr / export / clone flows — refresh() loads
  // the slim list which doesn't carry per-client secrets.
  const hydrateInbound = useCallback(async (id: number) => {
    const msg = await HttpUtil.get(`/panel/api/inbounds/get/${id}`);
    if (!msg?.success || !msg.obj) return null;
    const validated = parseMsg(msg, InboundDetailSchema, `inbounds/get/${id}`);
    if (!validated.obj) return null;
    const dbInbound = new DBInbound(validated.obj) as DBInboundInstance;
    setDbInbounds((prev) => {
      const next = prev.map((row) =>
        (row as unknown as { id: number }).id === id ? dbInbound : row,
      );
      dbInboundsRef.current = next;
      return next;
    });
    return dbInbound;
  }, []);

  const applyTrafficEvent = useCallback((payload: unknown) => {
    if (!payload || typeof payload !== 'object') return;
    const p = payload as {
      traffics?: TrafficDelta[];
      nodeTraffics?: TrafficDelta[];
      onlineClients?: string[];
      onlineByGuid?: Record<string, string[]>;
      activeInbounds?: Record<string, string[]>;
      lastOnlineMap?: Record<string, number>;
    };
    if (Array.isArray(p.onlineClients)) {
      setOnlineClients(p.onlineClients);
    }
    if (p.onlineByGuid && typeof p.onlineByGuid === 'object') {
      setOnlineByGuid(toGuidOnlineMap(p.onlineByGuid));
    }
    if (p.activeInbounds && typeof p.activeInbounds === 'object') {
      setActiveByGuid(toGuidOnlineMap(p.activeInbounds));
    }
    if (p.lastOnlineMap && typeof p.lastOnlineMap === 'object') {
      setLastOnlineMap((prev) => ({ ...prev, ...p.lastOnlineMap! }));
    }
    // Speed arrives from two independent 5s polls: the local Xray poll sends
    // `traffics` (local inbounds) and the node sync sends `nodeTraffics` (node
    // inbounds). Each replaces speed only within its own scope so the two don't
    // clobber each other; an idle in-scope inbound — absent from its payload —
    // clears instead of showing a stale value.
    const applyTraffics = (
      traffics: TrafficDelta[],
      inScope: (ib: DBInboundInstance) => boolean,
    ) => {
      const byTag = new Map<string, TrafficDelta>();
      for (const tr of traffics) {
        if (!tr || typeof tr.Tag !== 'string') continue;
        if (tr.IsInbound === false) continue;
        byTag.set(tr.Tag, tr);
      }
      setInboundSpeed((prev) => {
        const next = { ...prev };
        for (const ib of dbInboundsRef.current) {
          if (!inScope(ib)) continue;
          const delta = byTag.get(ib.tag);
          if (delta) {
            next[ib.id] = {
              up: (delta.Up || 0) / TRAFFIC_POLL_INTERVAL_S,
              down: (delta.Down || 0) / TRAFFIC_POLL_INTERVAL_S,
            };
          } else {
            delete next[ib.id];
          }
        }
        return next;
      });
    };
    if (Array.isArray(p.traffics)) applyTraffics(p.traffics, (ib) => ib.nodeId == null);
    if (Array.isArray(p.nodeTraffics)) applyTraffics(p.nodeTraffics, (ib) => ib.nodeId != null);
  }, []);

  const applyClientStatsEvent = useCallback((payload: unknown) => {
    if (!payload || typeof payload !== 'object') return;
    const p = payload as {
      inbounds?: { id: number; up?: number; down?: number; total?: number; enable?: boolean }[];
      clients?: {
        email: string;
        up?: number;
        down?: number;
        total?: number;
        expiryTime?: number;
        enable?: boolean;
      }[];
    };

    const byId = new Map<
      number,
      { id: number; up?: number; down?: number; total?: number; enable?: boolean }
    >();
    if (Array.isArray(p.inbounds)) {
      for (const row of p.inbounds) {
        if (row && row.id != null) byId.set(row.id, row);
      }
    }
    const byEmail = new Map<
      string,
      {
        email: string;
        up?: number;
        down?: number;
        total?: number;
        expiryTime?: number;
        enable?: boolean;
      }
    >();
    if (Array.isArray(p.clients)) {
      for (const row of p.clients) {
        if (row && row.email) byEmail.set(row.email, row);
      }
    }
    if (byId.size === 0 && byEmail.size === 0) return;

    // Rows carrying an update are rebuilt rather than patched in place: the
    // derived clientCount only recomputes when a row's identity changes.
    let touched = false;
    const next = dbInboundsRef.current.map((ib) => {
      const upd = byId.get(ib.id);
      const stats = Array.isArray(ib.clientStats) ? ib.clientStats : null;
      let statsTouched = false;
      const nextStats =
        stats && byEmail.size > 0
          ? stats.map((stat) => {
              const su = byEmail.get(stat.email);
              if (!su) return stat;
              statsTouched = true;
              return {
                ...stat,
                up: typeof su.up === 'number' ? su.up : stat.up,
                down: typeof su.down === 'number' ? su.down : stat.down,
                total: typeof su.total === 'number' ? su.total : stat.total,
                expiryTime: typeof su.expiryTime === 'number' ? su.expiryTime : stat.expiryTime,
                enable: typeof su.enable === 'boolean' ? su.enable : stat.enable,
              } as ClientStats;
            })
          : null;
      if (!upd && !statsTouched) return ib;
      touched = true;
      const row = new DBInbound(ib as DBInboundInit) as DBInboundInstance;
      if (upd) {
        if (typeof upd.up === 'number') row.up = upd.up;
        if (typeof upd.down === 'number') row.down = upd.down;
        if (typeof upd.total === 'number') row.total = upd.total;
        if (typeof upd.enable === 'boolean') row.enable = upd.enable;
      }
      if (statsTouched && nextStats) row.clientStats = nextStats;
      return row;
    });

    if (!touched) return;
    dbInboundsRef.current = next;
    setDbInbounds(next);
  }, []);

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
    fetchError,
    dbInbounds,
    clientCount,
    onlineClients,
    lastOnlineMap,
    inboundSpeed,
    totals,
    expireDiff,
    trafficDiff,
    subSettings,
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
