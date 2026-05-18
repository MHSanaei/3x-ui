import { onMounted, ref, shallowRef } from 'vue';
import { HttpUtil } from '@/utils';

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };

export function useClients() {
  const clients = shallowRef([]);
  const inbounds = shallowRef([]);
  const onlines = ref([]);
  const loading = ref(false);
  const fetched = ref(false);
  const subSettings = ref({ enable: false, subURI: '', subJsonURI: '', subJsonEnable: false });
  const ipLimitEnable = ref(false);
  const tgBotEnable = ref(false);
  const expireDiff = ref(0);
  const trafficDiff = ref(0);

  async function refresh() {
    loading.value = true;
    try {
      const [clientsMsg, inboundsMsg] = await Promise.all([
        HttpUtil.get('/panel/api/clients/list'),
        HttpUtil.get('/panel/api/inbounds/options'),
      ]);
      if (clientsMsg?.success) {
        clients.value = Array.isArray(clientsMsg.obj) ? clientsMsg.obj : [];
      }
      if (inboundsMsg?.success) {
        inbounds.value = Array.isArray(inboundsMsg.obj) ? inboundsMsg.obj : [];
      }
      fetched.value = true;
    } finally {
      loading.value = false;
    }
  }

  async function fetchSubSettings() {
    const msg = await HttpUtil.post('/panel/setting/defaultSettings');
    if (!msg?.success) return;
    const s = msg.obj || {};
    subSettings.value = {
      enable: !!s.subEnable,
      subURI: s.subURI || '',
      subJsonURI: s.subJsonURI || '',
      subJsonEnable: !!s.subJsonEnable,
    };
    ipLimitEnable.value = !!s.ipLimitEnable;
    tgBotEnable.value = !!s.tgBotEnable;
    expireDiff.value = (s.expireDiff ?? 0) * 86400000;
    trafficDiff.value = (s.trafficDiff ?? 0) * 1073741824;
  }

  async function create(payload) {
    const msg = await HttpUtil.post('/panel/api/clients/add', payload, JSON_HEADERS);
    if (msg?.success) await refresh();
    return msg;
  }

  async function update(email, client) {
    if (!email) return null;
    const encoded = encodeURIComponent(email);
    const msg = await HttpUtil.post(`/panel/api/clients/update/${encoded}`, client, JSON_HEADERS);
    if (msg?.success) await refresh();
    return msg;
  }

  async function remove(email, keepTraffic = false) {
    if (!email) return null;
    const encoded = encodeURIComponent(email);
    const url = keepTraffic
      ? `/panel/api/clients/del/${encoded}?keepTraffic=1`
      : `/panel/api/clients/del/${encoded}`;
    const msg = await HttpUtil.post(url);
    if (msg?.success) await refresh();
    return msg;
  }

  async function removeMany(emails, keepTraffic = false) {
    if (!Array.isArray(emails) || emails.length === 0) return [];
    const suffix = keepTraffic ? '?keepTraffic=1' : '';
    const silentOpts = { silent: true };
    const results = await Promise.all(emails.map((email) => {
      const url = `/panel/api/clients/del/${encodeURIComponent(email)}${suffix}`;
      return HttpUtil.post(url, undefined, silentOpts);
    }));
    await refresh();
    return results;
  }

  async function attach(email, inboundIds) {
    if (!email) return null;
    const encoded = encodeURIComponent(email);
    const msg = await HttpUtil.post(`/panel/api/clients/${encoded}/attach`, { inboundIds }, JSON_HEADERS);
    if (msg?.success) await refresh();
    return msg;
  }

  async function detach(email, inboundIds) {
    if (!email) return null;
    const encoded = encodeURIComponent(email);
    const msg = await HttpUtil.post(`/panel/api/clients/${encoded}/detach`, { inboundIds }, JSON_HEADERS);
    if (msg?.success) await refresh();
    return msg;
  }

  async function resetTraffic(client) {
    if (!client?.email) return null;
    const url = `/panel/api/clients/resetTraffic/${encodeURIComponent(client.email)}`;
    const msg = await HttpUtil.post(url);
    if (msg?.success) await refresh();
    return msg;
  }

  async function resetAllTraffics() {
    const msg = await HttpUtil.post('/panel/api/clients/resetAllTraffics');
    if (msg?.success) await refresh();
    return msg;
  }

  async function delDepleted() {
    const msg = await HttpUtil.post('/panel/api/clients/delDepleted');
    if (msg?.success) await refresh();
    return msg;
  }

  async function setEnable(client, enable) {
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
  }

  function applyTrafficEvent(payload) {
    if (!payload || typeof payload !== 'object') return;
    if (Array.isArray(payload.onlineClients)) {
      onlines.value = payload.onlineClients;
    }
  }

  function applyClientStatsEvent(payload) {
    if (!payload || typeof payload !== 'object') return;
    if (!Array.isArray(payload.clients) || payload.clients.length === 0) return;
    const byEmail = new Map();
    for (const row of payload.clients) {
      if (row && row.email) byEmail.set(row.email, row);
    }
    let touched = false;
    const next = clients.value || [];
    for (let i = 0; i < next.length; i++) {
      const row = next[i];
      const upd = byEmail.get(row?.email);
      if (!upd) continue;
      const merged = { ...(row.traffic || {}) };
      if (typeof upd.up === 'number') merged.up = upd.up;
      if (typeof upd.down === 'number') merged.down = upd.down;
      if (typeof upd.total === 'number') merged.total = upd.total;
      if (typeof upd.expiryTime === 'number') merged.expiryTime = upd.expiryTime;
      if (typeof upd.enable === 'boolean') merged.enable = upd.enable;
      if (typeof upd.lastOnline === 'number') merged.lastOnline = upd.lastOnline;
      next[i] = { ...row, traffic: merged };
      touched = true;
    }
    if (touched) clients.value = [...next];
  }

  let invalidateTimer = null;
  function applyInvalidate(payload) {
    if (!payload || typeof payload !== 'object') return;
    if (payload.type !== 'inbounds' && payload.type !== 'clients') return;
    if (invalidateTimer) clearTimeout(invalidateTimer);
    invalidateTimer = setTimeout(() => {
      invalidateTimer = null;
      refresh();
    }, 200);
  }

  onMounted(async () => {
    await Promise.all([refresh(), fetchSubSettings()]);
  });

  return {
    clients,
    inbounds,
    onlines,
    loading,
    fetched,
    subSettings,
    ipLimitEnable,
    tgBotEnable,
    expireDiff,
    trafficDiff,
    refresh,
    create,
    update,
    remove,
    removeMany,
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
