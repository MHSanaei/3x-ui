import { useEffect, useState } from 'react';
import { HttpUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { DefaultsPayloadSchema } from '@/schemas/defaults';

type Calendar = 'gregorian' | 'jalalian';

let cachedValue: Calendar = 'gregorian';
let fetched = false;
let pending: Promise<void> | null = null;
const listeners = new Set<(value: Calendar) => void>();

function notify(value: Calendar) {
  listeners.forEach((fn) => fn(value));
}

async function loadOnce(): Promise<void> {
  if (fetched) return;
  if (pending) {
    await pending;
    return;
  }
  pending = (async () => {
    try {
      const msg = await HttpUtil.post('/panel/api/setting/defaultSettings');
      if (msg?.success) {
        const validated = parseMsg(msg, DefaultsPayloadSchema, 'setting/defaultSettings');
        cachedValue = validated.obj?.datepicker || 'gregorian';
        notify(cachedValue);
      }
    } finally {
      fetched = true;
      pending = null;
    }
  })();
  await pending;
}

export function setDatepicker(value: Calendar) {
  fetched = true;
  cachedValue = value || 'gregorian';
  notify(cachedValue);
}

export function useDatepicker() {
  const [datepicker, setLocal] = useState<Calendar>(cachedValue);

  useEffect(() => {
    listeners.add(setLocal);
    loadOnce();
    return () => {
      listeners.delete(setLocal);
    };
  }, []);

  return { datepicker };
}
