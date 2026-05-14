// Lightweight composable that fetches the node list once on mount and
// exposes id→name + id→online lookups. Used by the Inbounds page so it
// can render a Node selector and a Node column without pulling the
// full pages/nodes/useNodes.js (which polls and owns CRUD state).

import { onMounted, ref, computed } from 'vue';
import { HttpUtil } from '@/utils';

export function useNodeList() {
  const nodes = ref([]);
  const fetched = ref(false);

  async function refresh() {
    const msg = await HttpUtil.get('/panel/api/nodes/list');
    if (msg?.success) {
      nodes.value = Array.isArray(msg.obj) ? msg.obj : [];
    }
    fetched.value = true;
  }

  // Indexed by id for O(1) UI lookups (Node column on N-row tables).
  const byId = computed(() => {
    const m = new Map();
    for (const n of nodes.value) m.set(n.id, n);
    return m;
  });

  function nameFor(id) {
    if (id == null) return null;
    return byId.value.get(id)?.name || null;
  }

  function isOnline(id) {
    if (id == null) return true;
    const n = byId.value.get(id);
    return n != null && n.enable && n.status === 'online';
  }

  const hasActive = computed(() => nodes.value.some((n) => n.enable));

  onMounted(refresh);

  return { nodes, fetched, refresh, byId, nameFor, isOnline, hasActive };
}
