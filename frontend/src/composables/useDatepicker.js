// Module-scoped reactive ref for the panel's "Calendar Type" setting.
// Loaded from /panel/setting/defaultSettings on first use, so any
// component (modals, inbound forms, future pages) can read the same
// value without prop-drilling and without re-fetching.
//
// useInbounds (which already reads defaultSettings for its own state)
// calls setDatepicker() after its fetch so we don't issue a second
// HTTP round-trip on the inbounds page.

import { readonly, ref } from 'vue';
import { HttpUtil } from '@/utils';

const datepicker = ref('gregorian');
let fetched = false;
let pending = null;

async function loadOnce() {
  if (fetched) return;
  if (pending) {
    await pending;
    return;
  }
  pending = (async () => {
    try {
      const msg = await HttpUtil.post('/panel/setting/defaultSettings');
      if (msg?.success) {
        datepicker.value = msg.obj?.datepicker || 'gregorian';
      }
    } finally {
      fetched = true;
      pending = null;
    }
  })();
  await pending;
}

export function setDatepicker(value) {
  fetched = true;
  datepicker.value = value || 'gregorian';
}

export function useDatepicker() {
  loadOnce();
  return { datepicker: readonly(datepicker) };
}
