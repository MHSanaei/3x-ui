
import { onMounted, onUnmounted, ref, watch } from 'vue';
import { HttpUtil, PromiseUtil } from '@/utils';

const DIRTY_POLL_MS = 1000;

let syncing = false;

export function useXraySetting() {
  const fetched = ref(false);
  const spinning = ref(false);
  const saveDisabled = ref(true);
  const fetchError = ref('');
  const xraySetting = ref('');
  const oldXraySetting = ref('');
  const templateSettings = ref(null);
  const outboundTestUrl = ref('https://www.google.com/generate_204');
  const oldOutboundTestUrl = ref('');
  const inboundTags = ref([]);
  const clientReverseTags = ref([]);
  const restartResult = ref('');
  const outboundsTraffic = ref([]);
  const outboundTestStates = ref({});

  async function fetchAll() {
    fetchError.value = '';
    const msg = await HttpUtil.post('/panel/xray/');
    if (!msg?.success) {
      fetchError.value = msg?.msg || 'Failed to load xray config';
      fetched.value = true;
      return;
    }
    let obj;
    try {
      obj = JSON.parse(msg.obj);
    } catch (e) {
      fetchError.value = `Malformed xray config response: ${e?.message || e}`;
      fetched.value = true;
      return;
    }
    const pretty = JSON.stringify(obj.xraySetting, null, 2);
    syncing = true;
    xraySetting.value = pretty;
    oldXraySetting.value = pretty;
    templateSettings.value = obj.xraySetting;
    syncing = false;
    inboundTags.value = obj.inboundTags || [];
    clientReverseTags.value = obj.clientReverseTags || [];
    outboundTestUrl.value = obj.outboundTestUrl || 'https://www.google.com/generate_204';
    oldOutboundTestUrl.value = outboundTestUrl.value;
    fetched.value = true;
    saveDisabled.value = true;
  }


  watch(
    templateSettings,
    (next) => {
      if (syncing || !next) return;
      syncing = true;
      try {
        xraySetting.value = JSON.stringify(next, null, 2);
      } finally {
        syncing = false;
      }
    },
    { deep: true },
  );

  watch(xraySetting, (next) => {
    if (syncing) return;
    try {
      const parsed = JSON.parse(next);
      syncing = true;
      try {
        templateSettings.value = parsed;
      } finally {
        syncing = false;
      }
    } catch (_e) { /* ignore — wait for user to finish */ }
  });

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

  async function fetchOutboundsTraffic() {
    const msg = await HttpUtil.get('/panel/xray/getOutboundsTraffic');
    if (msg?.success) outboundsTraffic.value = msg.obj || [];
  }

  async function resetOutboundsTraffic(tag) {
    const msg = await HttpUtil.post('/panel/xray/resetOutboundsTraffic', { tag });
    if (msg?.success) await fetchOutboundsTraffic();
  }

  function applyOutboundsEvent(payload) {
    if (Array.isArray(payload)) outboundsTraffic.value = payload;
  }

  async function testOutbound(index, outbound, mode = 'tcp') {
    if (!outbound) return null;
    if (!outboundTestStates.value[index]) outboundTestStates.value[index] = {};
    outboundTestStates.value[index] = { testing: true, result: null, mode };
    try {
      const msg = await HttpUtil.post('/panel/xray/testOutbound', {
        outbound: JSON.stringify(outbound),
        allOutbounds: JSON.stringify(templateSettings.value?.outbounds || []),
        mode,
      });
      if (msg?.success) {
        outboundTestStates.value[index] = { testing: false, result: msg.obj };
        return msg.obj;
      }
      outboundTestStates.value[index] = {
        testing: false,
        result: { success: false, error: msg?.msg || 'Unknown error', mode },
      };
    } catch (e) {
      outboundTestStates.value[index] = {
        testing: false,
        result: { success: false, error: String(e), mode },
      };
    }
    return null;
  }

  const testingAll = ref(false);
  async function testAllOutbounds(mode = 'tcp') {
    const list = templateSettings.value?.outbounds || [];
    if (list.length === 0 || testingAll.value) return;
    testingAll.value = true;
    try {
      const concurrency = mode === 'tcp' ? 8 : 1;
      const queue = list
        .map((ob, i) => ({ index: i, outbound: ob }))
        .filter(({ outbound }) => {
          const tag = outbound?.tag;
          const proto = outbound?.protocol;
          if (proto === 'blackhole' || proto === 'loopback' || tag === 'blocked') return false;
          if (mode === 'tcp' && (proto === 'freedom' || proto === 'dns')) return false;
          return true;
        });
      async function worker() {
        while (queue.length > 0) {
          const item = queue.shift();
          if (!item) break;
          await testOutbound(item.index, item.outbound, mode);
        }
      }
      const workers = Array.from({ length: Math.min(concurrency, queue.length) }, () => worker());
      await Promise.all(workers);
    } finally {
      testingAll.value = false;
    }
  }

  async function resetToDefault() {
    spinning.value = true;
    try {
      const msg = await HttpUtil.get('/panel/setting/getDefaultJsonConfig');
      if (msg?.success) {

        templateSettings.value = JSON.parse(JSON.stringify(msg.obj));
      }
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
    fetchOutboundsTraffic();
    startDirtyPoll();
  });
  onUnmounted(stopDirtyPoll);

  return {
    fetched,
    spinning,
    saveDisabled,
    fetchError,
    xraySetting,
    templateSettings,
    outboundTestUrl,
    inboundTags,
    clientReverseTags,
    restartResult,
    outboundsTraffic,
    outboundTestStates,
    testingAll,
    fetchAll,
    fetchOutboundsTraffic,
    resetOutboundsTraffic,
    applyOutboundsEvent,
    testOutbound,
    testAllOutbounds,
    saveAll,
    resetToDefault,
    restartXray,
  };
}
