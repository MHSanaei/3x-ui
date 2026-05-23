import { useCallback, useEffect, useRef, useState } from 'react';

import { HttpUtil } from '@/utils';
import { Status } from '@/models/status';

const POLL_INTERVAL_MS = 2000;

export function useStatus() {
  const [status, setStatus] = useState<Status>(() => new Status());
  const [fetched, setFetched] = useState(false);
  const fetchedRef = useRef(false);

  const refresh = useCallback(async () => {
    try {
      const msg = await HttpUtil.get('/panel/api/server/status');
      if (msg?.success) {
        setStatus(new Status(msg.obj));
        if (!fetchedRef.current) {
          fetchedRef.current = true;
          setFetched(true);
        }
      }
    } catch (e) {
      console.error('Failed to get status:', e);
    }
  }, []);

  useEffect(() => {
    refresh();
    const timer = window.setInterval(refresh, POLL_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [refresh]);

  return { status, fetched, refresh };
}
