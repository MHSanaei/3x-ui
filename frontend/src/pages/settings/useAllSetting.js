// Centralizes the AllSetting fetch/save lifecycle the legacy panel
// scattered across data() + methods + a busy-loop dirty checker.
//
// The dirty flag is recomputed once per second (matching the legacy
// `while (true) sleep(1000)` poll) — we don't deep-watch because the
// settings tree has many nested fields and a poll is cheap enough.

import { onMounted, onUnmounted, reactive, ref } from 'vue';
import { HttpUtil } from '@/utils';
import { AllSetting } from '@/models/setting.js';

const DIRTY_POLL_MS = 1000;

export function useAllSetting() {
  const fetched = ref(false);
  const spinning = ref(false);
  const saveDisabled = ref(true);

  // Two reactive snapshots: the last server-side state and the one the
  // user is editing. `equals` compares enumerable props field-by-field.
  const oldAllSetting = reactive(new AllSetting());
  const allSetting = reactive(new AllSetting());

  function applyServerState(obj) {
    const fresh = new AllSetting(obj);
    Object.assign(oldAllSetting, fresh);
    Object.assign(allSetting, fresh);
    saveDisabled.value = true;
  }

  async function fetchAll() {
    const msg = await HttpUtil.post('/panel/setting/all');
    if (msg?.success) {
      fetched.value = true;
      applyServerState(msg.obj);
    }
  }

  async function saveAll() {
    spinning.value = true;
    try {
      const msg = await HttpUtil.post('/panel/setting/update', allSetting);
      if (msg?.success) await fetchAll();
    } finally {
      spinning.value = false;
    }
  }

  let timer = null;
  function startDirtyPoll() {
    if (timer != null) return;
    timer = setInterval(() => {
      // ObjectUtil.equals walks own enumerable props; reactive proxies
      // expose them transparently so this works without cloning.
      saveDisabled.value = oldAllSetting.equals(allSetting);
    }, DIRTY_POLL_MS);
  }
  function stopDirtyPoll() {
    if (timer != null) {
      clearInterval(timer);
      timer = null;
    }
  }

  onMounted(() => {
    fetchAll();
    startDirtyPoll();
  });
  onUnmounted(stopDirtyPoll);

  return {
    fetched,
    spinning,
    saveDisabled,
    oldAllSetting,
    allSetting,
    fetchAll,
    saveAll,
  };
}
