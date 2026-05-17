import { onMounted, ref, shallowRef } from 'vue';
import { HttpUtil } from '@/utils';

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };

export function useClients() {
  const clients = shallowRef([]);
  const inbounds = shallowRef([]);
  const loading = ref(false);
  const fetched = ref(false);

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

  onMounted(refresh);

  return {
    clients,
    inbounds,
    loading,
    fetched,
    refresh,
    create,
    update,
    remove,
    attach,
    detach,
  };
}
