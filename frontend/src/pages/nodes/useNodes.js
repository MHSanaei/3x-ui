// Loads the node list and runs CRUD/probe actions against the
// /panel/api/nodes/* endpoints. Live updates arrive over WebSocket
// (pushed by NodeHeartbeatJob every 10s) so we don't poll.

import { computed, onMounted, ref, shallowRef } from 'vue';
import { HttpUtil } from '@/utils';

export function useNodes() {
  const nodes = shallowRef([]);
  const loading = ref(false);
  const fetched = ref(false);

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

  // Replaces the local list with the snapshot pushed by the heartbeat job.
  // shallowRef means a fresh assignment is enough to retrigger reactivity;
  // we always assign a new array so Vue notices.
  function applyNodesEvent(payload) {
    if (Array.isArray(payload)) {
      nodes.value = payload;
      if (!fetched.value) fetched.value = true;
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

  // Aggregate cards on the dashboard. Computed off the live list so a
  // refresh (or a WS push) picks up new totals automatically.
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

  // Initial fetch — WebSocket takes over after the first heartbeat tick
  // (~10s) but the page should populate immediately on mount.
  onMounted(refresh);

  return {
    nodes,
    loading,
    fetched,
    totals,
    refresh,
    applyNodesEvent,
    create,
    update,
    remove,
    setEnable,
    testConnection,
    probe,
  };
}
