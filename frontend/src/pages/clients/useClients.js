import { onMounted, onUnmounted, ref, shallowRef } from 'vue';
import { HttpUtil } from '@/utils';

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };
const ONLINES_POLL_MS = 10000;

export function useClients() {
  const clients = shallowRef([]);
  const inbounds = shallowRef([]);
  const onlines = ref([]);
  const loading = ref(false);
  const fetched = ref(false);
  let onlinesTimer = null;

  async function refresh() {
    loading.value = true;
    try {
      const [clientsMsg, inboundsMsg] = await Promise.all([
        HttpUtil.get('/panel/api/clients/list'),
        HttpUtil.get('/panel/api/inbounds/list'),
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

  async function refreshOnlines() {
    const msg = await HttpUtil.post('/panel/api/inbounds/onlines');
    if (msg?.success) {
      onlines.value = Array.isArray(msg.obj) ? msg.obj : [];
    }
  }

  async function create(payload) {
    const msg = await HttpUtil.post('/panel/api/clients/add', payload, JSON_HEADERS);
    if (msg?.success) await refresh();
    return msg;
  }

  async function update(id, client) {
    const msg = await HttpUtil.post(`/panel/api/clients/update/${id}`, client, JSON_HEADERS);
    if (msg?.success) await refresh();
    return msg;
  }

  async function remove(id, keepTraffic = false) {
    const url = keepTraffic
      ? `/panel/api/clients/del/${id}?keepTraffic=1`
      : `/panel/api/clients/del/${id}`;
    const msg = await HttpUtil.post(url);
    if (msg?.success) await refresh();
    return msg;
  }

  async function attach(id, inboundIds) {
    const msg = await HttpUtil.post(`/panel/api/clients/${id}/attach`, { inboundIds }, JSON_HEADERS);
    if (msg?.success) await refresh();
    return msg;
  }

  async function detach(id, inboundIds) {
    const msg = await HttpUtil.post(`/panel/api/clients/${id}/detach`, { inboundIds }, JSON_HEADERS);
    if (msg?.success) await refresh();
    return msg;
  }

  async function resetTraffic(client) {
    const ibIds = Array.isArray(client?.inboundIds) ? client.inboundIds : [];
    if (!client?.email || ibIds.length === 0) return null;
    const url = `/panel/api/inbounds/${ibIds[0]}/resetClientTraffic/${encodeURIComponent(client.email)}`;
    const msg = await HttpUtil.post(url);
    if (msg?.success) await refresh();
    return msg;
  }

  onMounted(async () => {
    await refresh();
    refreshOnlines();
    onlinesTimer = setInterval(refreshOnlines, ONLINES_POLL_MS);
  });

  onUnmounted(() => {
    if (onlinesTimer) clearInterval(onlinesTimer);
  });

  return {
    clients,
    inbounds,
    onlines,
    loading,
    fetched,
    refresh,
    refreshOnlines,
    create,
    update,
    remove,
    attach,
    detach,
    resetTraffic,
  };
}
