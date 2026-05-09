// Loads the node list and runs CRUD/probe actions against the
// /panel/api/nodes/* endpoints. Polls every 5s while the page is
// visible so heartbeat status stays fresh without a WebSocket.

import { computed, onBeforeUnmount, onMounted, ref, shallowRef } from 'vue';
import { HttpUtil } from '@/utils';

const POLL_INTERVAL_MS = 5000;

export function useNodes() {
  const nodes = shallowRef([]);
  const loading = ref(false);
  const fetched = ref(false);

  let pollTimer = null;
  let pageVisible = true;

  async function refresh() {
    loading.value = true;
    try {
      const msg = await HttpUtil.get('/panel/api/nodes/list');
      if (msg?.success) {
        nodes.value = Array.isArray(msg.obj) ? msg.obj : [];
      }
      fetched.value = true;
    } finally {
      loading.value = false;
    }
  }

  async function create(payload) {
    const msg = await HttpUtil.post('/panel/api/nodes/add', payload);
    if (msg?.success) await refresh();
    return msg;
  }

  async function update(id, payload) {
    const msg = await HttpUtil.post(`/panel/api/nodes/update/${id}`, payload);
    if (msg?.success) await refresh();
    return msg;
  }

  async function remove(id) {
    const msg = await HttpUtil.post(`/panel/api/nodes/del/${id}`);
    if (msg?.success) await refresh();
    return msg;
  }

  async function setEnable(id, enable) {
    const msg = await HttpUtil.post(`/panel/api/nodes/setEnable/${id}`, { enable });
    if (msg?.success) await refresh();
    return msg;
  }

  // testConnection probes a transient (unsaved) node config so the form
  // can validate before save. Returns the ProbeResultUI shape from Go.
  async function testConnection(payload) {
    const msg = await HttpUtil.post('/panel/api/nodes/test', payload);
    return msg;
  }

  // probe forces an immediate heartbeat against an already-saved node.
  async function probe(id) {
    const msg = await HttpUtil.post(`/panel/api/nodes/probe/${id}`);
    if (msg?.success) await refresh();
    return msg;
  }

  // Aggregate cards on the dashboard. Computed off the live list so
  // a refresh picks up new totals automatically.
  const totals = computed(() => {
    const list = nodes.value;
    let online = 0;
    let offline = 0;
    let latencySum = 0;
    let latencyCount = 0;
    for (const n of list) {
      if (!n.enable) continue;
      if (n.status === 'online') {
        online += 1;
        if (n.latencyMs > 0) {
          latencySum += n.latencyMs;
          latencyCount += 1;
        }
      } else if (n.status === 'offline') {
        offline += 1;
      }
    }
    return {
      total: list.length,
      online,
      offline,
      avgLatency: latencyCount > 0 ? Math.round(latencySum / latencyCount) : 0,
    };
  });

  function startPolling() {
    if (pollTimer) return;
    pollTimer = setInterval(() => {
      if (pageVisible) refresh();
    }, POLL_INTERVAL_MS);
  }

  function stopPolling() {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  }

  function onVisibilityChange() {
    pageVisible = !document.hidden;
    if (pageVisible) refresh();
  }

  onMounted(() => {
    refresh();
    startPolling();
    document.addEventListener('visibilitychange', onVisibilityChange);
  });

  onBeforeUnmount(() => {
    stopPolling();
    document.removeEventListener('visibilitychange', onVisibilityChange);
  });

  return {
    nodes,
    loading,
    fetched,
    totals,
    refresh,
    create,
    update,
    remove,
    setEnable,
    testConnection,
    probe,
  };
}
