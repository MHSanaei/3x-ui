import { onBeforeUnmount, onMounted, ref, shallowRef } from 'vue';

import { HttpUtil } from '@/utils';
import { Status } from '@/models/status.js';

const POLL_INTERVAL_MS = 2000;

// Polls /panel/api/server/status and exposes a reactive Status object
// + a `fetched` flag so consumers can show a spinner before the first
// successful fetch.
//
// WebSocket integration is intentionally deferred to a later sub-phase.
// Polling at 2s is the same fallback the legacy panel falls back to
// when its websocket link drops, so we're shipping the proven path
// first and adding the websocket on top later.
export function useStatus() {
  const status = shallowRef(new Status());
  const fetched = ref(false);
  let timer = null;

  async function refresh() {
    try {
      const msg = await HttpUtil.get('/panel/api/server/status');
      if (msg?.success) {
        status.value = new Status(msg.obj);
        if (!fetched.value) fetched.value = true;
      }
    } catch (e) {
      console.error('Failed to get status:', e);
    }
  }

  onMounted(() => {
    refresh();
    timer = window.setInterval(refresh, POLL_INTERVAL_MS);
  });

  onBeforeUnmount(() => {
    if (timer != null) window.clearInterval(timer);
  });

  return { status, fetched, refresh };
}
