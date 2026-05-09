// Loads the inbound list + sidecar data the page needs (online users,
// last-online-map, default settings) and computes the per-inbound client
// roll-ups the legacy panel surfaces in the popovers.
//
// 5f-i scope: plain GET on mount + a manual refresh; auto-refresh and the
// WebSocket delta path are deferred to a later subphase.

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
      }
    }
    return { up, down, allTime, clients, deactive, depleted, expiring };
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
  };
}
