// Loads the inbound list + sidecar data the page needs (online users,
// last-online-map, default settings) and computes the per-inbound client
// roll-ups the legacy panel surfaces in the popovers.
//
// Live-update model: initial GET on mount, then the WebSocket delta path
// keeps the table fresh — the page subscribes to the server's `traffic`,
// `client_stats`, and `invalidate` events and merges them into local
// refs in-place. The manual refresh button is kept as a fallback.

import { computed, ref, shallowRef } from 'vue';
import { HttpUtil, ObjectUtil } from '@/utils';
import { DBInbound } from '@/models/dbinbound.js';
import { Protocols } from '@/models/inbound.js';
import { setDatepicker } from '@/composables/useDatepicker.js';

export function useInbounds() {
  const fetched = ref(false);
  const refreshing = ref(false);

  // shallowRef because each refresh swaps the array; per-row reactivity is
  // unnecessary at the page level (modals work on copies).
  const dbInbounds = shallowRef([]);
  const clientCount = ref({});
  const onlineClients = ref([]);
  const lastOnlineMap = ref({});

  // Default-settings sidecar fields the table needs for color/expiry math.
  const expireDiff = ref(0);
  const trafficDiff = ref(0);
  const subSettings = ref({
    enable: false,
    subTitle: '',
    subURI: '',
    subJsonURI: '',
    subJsonEnable: false,
  });
  const remarkModel = ref('-ieo');
  const datepicker = ref('gregorian');
  const tgBotEnable = ref(false);
  const ipLimitEnable = ref(false);
  const pageSize = ref(0);

  function isClientOnline(email) {
    return onlineClients.value.includes(email);
  }

  // Roll-up of {clients, active, deactive, depleted, expiring, online,
  // comments} for a single inbound. Mirrors getClientCounts in the legacy
  // template. Skipped for protocols that don't have multi-user clients
  // (HTTP, MIXED, WireGuard) since their settings have no client list.
  function rollupClients(dbInbound, inbound) {
    const clientStats = Array.isArray(dbInbound.clientStats) ? dbInbound.clientStats : [];
    const clients = inbound?.clients || [];
    const active = [];
    const deactive = [];
    const depleted = [];
    const expiring = [];
    const online = [];
    const comments = new Map();
    const now = Date.now();

    if (dbInbound.enable) {
      for (const client of clients) {
        if (client.comment) comments.set(client.email, client.comment);
        if (client.enable) {
          active.push(client.email);
          if (isClientOnline(client.email)) online.push(client.email);
        } else {
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
            (stats.expiryTime > 0 && stats.expiryTime - now < expireDiff.value) ||
            (stats.total > 0 && stats.total - (stats.up + stats.down) < trafficDiff.value);
          if (expiringSoon) expiring.push(stats.email);
        }
      }
    } else {
      for (const client of clients) deactive.push(client.email);
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
  }

  function setInbounds(rows) {
    const next = [];
    const counts = {};
    for (const row of rows) {
      const dbInbound = new DBInbound(row);
      const parsed = dbInbound.toInbound();
      next.push(dbInbound);
      const tracked = [
        Protocols.VMESS,
        Protocols.VLESS,
        Protocols.TROJAN,
        Protocols.SHADOWSOCKS,
        Protocols.HYSTERIA,
      ];
      if (tracked.includes(row.protocol)) {
        if (dbInbound.isSS && !parsed.isSSMultiUser) continue;
        counts[row.id] = rollupClients(dbInbound, parsed);
      }
    }
    dbInbounds.value = next;
    clientCount.value = counts;
    fetched.value = true;
  }

  async function fetchOnlineUsers() {
    const msg = await HttpUtil.post('/panel/api/inbounds/onlines');
    if (msg?.success) onlineClients.value = msg.obj || [];
  }

  async function fetchLastOnlineMap() {
    const msg = await HttpUtil.post('/panel/api/inbounds/lastOnline');
    if (msg?.success && msg.obj) lastOnlineMap.value = msg.obj;
  }

  async function fetchDefaultSettings() {
    const msg = await HttpUtil.post('/panel/setting/defaultSettings');
    if (!msg?.success) return;
    const s = msg.obj || {};
    expireDiff.value = (s.expireDiff ?? 0) * 86400000;
    trafficDiff.value = (s.trafficDiff ?? 0) * 1073741824;
    tgBotEnable.value = !!s.tgBotEnable;
    subSettings.value = {
      enable: !!s.subEnable,
      subTitle: s.subTitle || '',
      subURI: s.subURI || '',
      subJsonURI: s.subJsonURI || '',
      subJsonEnable: !!s.subJsonEnable,
    };
    pageSize.value = s.pageSize ?? 0;
    remarkModel.value = s.remarkModel || '-ieo';
    datepicker.value = s.datepicker || 'gregorian';
    // Mirror into the global composable so date-pickers in modals can
    // pick the right calendar without re-fetching the settings.
    setDatepicker(datepicker.value);
    ipLimitEnable.value = !!s.ipLimitEnable;
  }

  // ============ WebSocket live-update merge ===========================
  // The xray traffic job and the node traffic sync job each broadcast
  // a `traffic` payload every ~10s. We merge it into onlineClients +
  // lastOnlineMap; per-inbound counters arrive in the parallel
  // client_stats event below.
  function applyTrafficEvent(payload) {
    if (!payload || typeof payload !== 'object') return;
    if (Array.isArray(payload.onlineClients)) {
      onlineClients.value = payload.onlineClients;
    }
    if (payload.lastOnlineMap && typeof payload.lastOnlineMap === 'object') {
      // Merge so a subsequent payload that drops a quiet client doesn't
      // wipe their last-seen timestamp.
      lastOnlineMap.value = { ...lastOnlineMap.value, ...payload.lastOnlineMap };
    }
    // Recompute per-inbound rollups so the "online" badges in the
    // expand-row table flip without waiting for a full refresh.
    rebuildClientCount();
  }

  // The client_stats payload carries absolute traffic counters for the
  // clients that had activity in the latest window plus per-inbound
  // totals. Both are absolute (not deltas), so we overwrite in place.
  function applyClientStatsEvent(payload) {
    if (!payload || typeof payload !== 'object') return;
    let touched = false;

    if (Array.isArray(payload.inbounds) && payload.inbounds.length > 0) {
      const byId = new Map();
      for (const row of payload.inbounds) {
        if (row && row.id != null) byId.set(row.id, row);
      }
      for (const ib of dbInbounds.value) {
        const upd = byId.get(ib.id);
        if (!upd) continue;
        if (typeof upd.up === 'number') ib.up = upd.up;
        if (typeof upd.down === 'number') ib.down = upd.down;
        if (typeof upd.allTime === 'number') ib.allTime = upd.allTime;
        if (typeof upd.total === 'number') ib.total = upd.total;
        if (typeof upd.enable === 'boolean') ib.enable = upd.enable;
        touched = true;
      }
    }

    if (Array.isArray(payload.clients) && payload.clients.length > 0) {
      const byEmail = new Map();
      for (const row of payload.clients) {
        if (row && row.email) byEmail.set(row.email, row);
      }
      for (const ib of dbInbounds.value) {
        if (!Array.isArray(ib.clientStats)) continue;
        for (let i = 0; i < ib.clientStats.length; i++) {
          const stat = ib.clientStats[i];
          const upd = byEmail.get(stat.email);
          if (!upd) continue;
          if (typeof upd.up === 'number') stat.up = upd.up;
          if (typeof upd.down === 'number') stat.down = upd.down;
          if (typeof upd.total === 'number') stat.total = upd.total;
          if (typeof upd.allTime === 'number') stat.allTime = upd.allTime;
          if (typeof upd.expiryTime === 'number') stat.expiryTime = upd.expiryTime;
          if (typeof upd.enable === 'boolean') stat.enable = upd.enable;
          touched = true;
        }
      }
    }

    if (touched) {
      dbInbounds.value = [...dbInbounds.value];
      rebuildClientCount();
    }
  }

  // The hub may decide a payload is too large to push directly and emit
  // an `invalidate` event with the affected dataType instead. For the
  // inbounds page that means "the inbound list changed elsewhere — go
  // re-fetch via REST".
  function applyInvalidate(payload) {
    if (!payload || typeof payload !== 'object') return;
    if (payload.type === 'inbounds') {
      refresh();
    }
  }

  function applyInboundsEvent(payload) {
    if (!Array.isArray(payload)) return;
    setInbounds(payload);
  }

  // Recompute the per-inbound roll-up after any in-place mutation.
  // Cheap because rollupClients only iterates a single inbound's
  // clients + clientStats arrays.
  function rebuildClientCount() {
    const counts = {};
    const tracked = [
      Protocols.VMESS,
      Protocols.VLESS,
      Protocols.TROJAN,
      Protocols.SHADOWSOCKS,
      Protocols.HYSTERIA,
    ];
    for (const dbInbound of dbInbounds.value) {
      const parsed = dbInbound.toInbound();
      if (!tracked.includes(dbInbound.protocol)) continue;
      if (dbInbound.isSS && !parsed.isSSMultiUser) continue;
      counts[dbInbound.id] = rollupClients(dbInbound, parsed);
    }
    clientCount.value = counts;
  }

  async function refresh() {
    refreshing.value = true;
    try {
      const msg = await HttpUtil.get('/panel/api/inbounds/list');
      if (!msg?.success) return;
      await fetchLastOnlineMap();
      await fetchOnlineUsers();
      setInbounds(Array.isArray(msg.obj) ? msg.obj : []);
    } finally {
      // Match legacy: keep the spinning-icon state visible briefly so
      // a fast network doesn't make the button feel like it didn't fire.
      setTimeout(() => { refreshing.value = false; }, 500);
    }
  }

  // Aggregate totals shown in the dashboard summary card. allTime falls
  // back to up+down when the per-inbound counter isn't populated yet.
  const totals = computed(() => {
    let up = 0;
    let down = 0;
    let allTime = 0;
    let clients = 0;
    const deactive = [];
    const depleted = [];
    const expiring = [];
    const online = [];
    for (const ib of dbInbounds.value) {
      up += ib.up || 0;
      down += ib.down || 0;
      allTime += ib.allTime || (ib.up + ib.down) || 0;
      const c = clientCount.value[ib.id];
      if (c) {
        clients += c.clients;
        deactive.push(...c.deactive);
        depleted.push(...c.depleted);
        expiring.push(...c.expiring);
        online.push(...c.online);
      }
    }
    return { up, down, allTime, clients, deactive, depleted, expiring, online };
  });

  // ObjectUtil reference is wired at module load — keeping a no-op import
  // here so the linter doesn't drop it; the legacy search uses it.
  void ObjectUtil;

  return {
    fetched,
    refreshing,
    dbInbounds,
    clientCount,
    onlineClients,
    lastOnlineMap,
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
    fetchDefaultSettings,
    applyTrafficEvent,
    applyClientStatsEvent,
    applyInvalidate,
    applyInboundsEvent,
  };
}
