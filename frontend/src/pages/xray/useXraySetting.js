// Drives the xray page's fetch / dirty / save lifecycle. The Go side
// returns the live xraySetting (the full JSON config), the inboundTags
// list, and a few sidecar values (clientReverseTags, outboundTestUrl)
// the structured tabs need. We keep the JSON as a string here — pretty-
// printed for the textarea; tabs that want a parsed view can JSON.parse
// it themselves.

import { onMounted, onUnmounted, ref } from 'vue';
import { HttpUtil, PromiseUtil } from '@/utils';

const DIRTY_POLL_MS = 1000;

export function useXraySetting() {
  const fetched = ref(false);
  const spinning = ref(false);
  const saveDisabled = ref(true);

  const xraySetting = ref('');
  const oldXraySetting = ref('');

  const outboundTestUrl = ref('https://www.google.com/generate_204');
  const oldOutboundTestUrl = ref('');

  const inboundTags = ref([]);
  const clientReverseTags = ref([]);
  const restartResult = ref('');

  async function fetchAll() {
    const msg = await HttpUtil.post('/panel/xray/');
    if (!msg?.success) return;
    const obj = JSON.parse(msg.obj);
    const pretty = JSON.stringify(obj.xraySetting, null, 2);
    xraySetting.value = pretty;
    oldXraySetting.value = pretty;
    inboundTags.value = obj.inboundTags || [];
    clientReverseTags.value = obj.clientReverseTags || [];
    outboundTestUrl.value = obj.outboundTestUrl || 'https://www.google.com/generate_204';
    oldOutboundTestUrl.value = outboundTestUrl.value;
    fetched.value = true;
    saveDisabled.value = true;
  }

  async function saveAll() {
    spinning.value = true;
    try {
      const msg = await HttpUtil.post('/panel/xray/update', {
        xraySetting: xraySetting.value,
        outboundTestUrl: outboundTestUrl.value || 'https://www.google.com/generate_204',
      });
      if (msg?.success) await fetchAll();
    } finally {
      spinning.value = false;
    }
  }

  async function restartXray() {
    spinning.value = true;
    try {
      const msg = await HttpUtil.post('/panel/api/server/restartXrayService');
      if (msg?.success) {
        // Match legacy: short pause, then poll for the result blob so
        // the popover surfaces any startup error from the new process.
        await PromiseUtil.sleep(500);
        const r = await HttpUtil.get('/panel/xray/getXrayResult');
        if (r?.success) restartResult.value = r.obj || '';
      }
    } finally {
      spinning.value = false;
    }
  }

  // Same 1s busy-loop pattern the settings page uses — keep it cheap
  // and consistent. Real work (the JSON diff) is just a string compare.
  let timer = null;
  function startDirtyPoll() {
    if (timer != null) return;
    timer = setInterval(() => {
      saveDisabled.value =
        oldXraySetting.value === xraySetting.value
        && oldOutboundTestUrl.value === outboundTestUrl.value;
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
    xraySetting,
    outboundTestUrl,
    inboundTags,
    clientReverseTags,
    restartResult,
    fetchAll,
    saveAll,
    restartXray,
  };
}
