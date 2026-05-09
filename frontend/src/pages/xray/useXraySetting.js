// Drives the xray page's fetch / dirty / save lifecycle. The Go side
// returns the live xraySetting (the full JSON config), the inboundTags
// list, and a few sidecar values (clientReverseTags, outboundTestUrl)
// the structured tabs need. We keep the JSON as a string here — pretty-
// printed for the textarea; tabs that want a parsed view can JSON.parse
// it themselves.

import { onMounted, onUnmounted, ref, watch } from 'vue';
import { HttpUtil, PromiseUtil } from '@/utils';

const DIRTY_POLL_MS = 1000;

// Hoists the parsed `templateSettings` alongside the JSON string so
// structured tabs (Basics/Routing/Outbounds/etc.) can mutate fields
// directly while the Advanced (JSON) tab edits the same data as text.
// We keep both in sync with two cooperating watches:
//   • mutating templateSettings re-stringifies into xraySetting;
//   • editing the JSON text re-parses into templateSettings (only on
//     valid JSON — invalid edits leave templateSettings untouched
//     so the structured tabs don't blow up while the user types).
let syncing = false;

export function useXraySetting() {
  const fetched = ref(false);
  const spinning = ref(false);
  const saveDisabled = ref(true);
  // Holds a user-facing message when fetchAll fails; lets the page
  // render an error UI instead of an endless spinner.
  const fetchError = ref('');

  const xraySetting = ref('');
  const oldXraySetting = ref('');

  // Parsed mirror — null until first successful fetch / parse.
  const templateSettings = ref(null);

  const outboundTestUrl = ref('https://www.google.com/generate_204');
  const oldOutboundTestUrl = ref('');

  const inboundTags = ref([]);
  const clientReverseTags = ref([]);
  const restartResult = ref('');

  // Outbounds tab data — traffic stats + per-row test state. Test
  // states are keyed by outbound index (sparse object), each entry
  // is `{ testing, result }` where result is the wire response from
  // /panel/xray/testOutbound or null while the test is in flight.
  const outboundsTraffic = ref([]);
  const outboundTestStates = ref({});

  async function fetchAll() {
    fetchError.value = '';
    const msg = await HttpUtil.post('/panel/xray/');
    if (!msg?.success) {
      fetchError.value = msg?.msg || 'Failed to load xray config';
      // Mark as fetched so the spinner clears and the error UI renders.
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

  // Structured tabs mutate templateSettings deeply. Re-stringify on
  // change so the Advanced JSON view + the dirty-poll see the edits.
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

  // Advanced JSON edits — only refresh templateSettings when the text
  // parses, so structured tabs stay readable mid-edit.
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

  // Merges a WebSocket `outbounds` event into outboundsTraffic in place.
  // The xray traffic job pushes the full snapshot every ~10s so the user
  // doesn't have to click the (now-removed) refresh button.
  function applyOutboundsEvent(payload) {
    if (Array.isArray(payload)) outboundsTraffic.value = payload;
  }

  async function testOutbound(index, outbound) {
    if (!outbound) return null;
    if (!outboundTestStates.value[index]) outboundTestStates.value[index] = {};
    outboundTestStates.value[index] = { testing: true, result: null };
    try {
      const msg = await HttpUtil.post('/panel/xray/testOutbound', {
        outbound: JSON.stringify(outbound),
        allOutbounds: JSON.stringify(templateSettings.value?.outbounds || []),
      });
      if (msg?.success) {
        outboundTestStates.value[index] = { testing: false, result: msg.obj };
        return msg.obj;
      }
      outboundTestStates.value[index] = {
        testing: false,
        result: { success: false, error: msg?.msg || 'Unknown error' },
      };
    } catch (e) {
      outboundTestStates.value[index] = {
        testing: false,
        result: { success: false, error: String(e) },
      };
    }
    return null;
  }

  async function resetToDefault() {
    spinning.value = true;
    try {
      const msg = await HttpUtil.get('/panel/setting/getDefaultJsonConfig');
      if (msg?.success) {
        // Mutate templateSettings — the watch above re-stringifies into
        // xraySetting so the Advanced JSON tab and dirty-poll see it.
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
    fetchAll,
    fetchOutboundsTraffic,
    resetOutboundsTraffic,
    applyOutboundsEvent,
    testOutbound,
    saveAll,
    resetToDefault,
    restartXray,
  };
}
