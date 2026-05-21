import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { HttpUtil } from '@/utils';
import { AllSetting } from '@/models/setting';

interface ApiMsg<T = unknown> {
  success?: boolean;
  obj?: T;
}

export function useAllSetting() {
  const [allSetting, setAllSetting] = useState<AllSetting>(() => new AllSetting());
  const [oldAllSetting, setOldAllSetting] = useState<AllSetting>(() => new AllSetting());
  const [fetched, setFetched] = useState(false);
  const [spinning, setSpinning] = useState(false);
  const fetchedRef = useRef(false);

  const applyServerState = useCallback((obj: unknown) => {
    setAllSetting(new AllSetting(obj));
    setOldAllSetting(new AllSetting(obj));
  }, []);

  const fetchAll = useCallback(async () => {
    const msg = await HttpUtil.post('/panel/setting/all') as ApiMsg;
    if (msg?.success) {
      applyServerState(msg.obj);
      fetchedRef.current = true;
      setFetched(true);
    }
  }, [applyServerState]);

  const saveAll = useCallback(async () => {
    setSpinning(true);
    try {
      const msg = await HttpUtil.post('/panel/setting/update', allSetting) as ApiMsg;
      if (msg?.success) await fetchAll();
    } finally {
      setSpinning(false);
    }
  }, [allSetting, fetchAll]);

  const updateSetting = useCallback((patch: Partial<AllSetting>) => {
    setAllSetting((prev) => {
      const next = new AllSetting(prev);
      Object.assign(next, patch);
      return next;
    });
  }, []);

  const saveDisabled = useMemo(
    () => allSetting.equals(oldAllSetting),
    [allSetting, oldAllSetting],
  );

  useEffect(() => {
     
    fetchAll();
  }, [fetchAll]);

  return {
    allSetting,
    updateSetting,
    fetched,
    spinning,
    setSpinning,
    saveDisabled,
    fetchAll,
    saveAll,
  };
}
