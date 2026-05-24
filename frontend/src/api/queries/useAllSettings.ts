import { useCallback, useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { AllSetting } from '@/models/setting';
import { keys } from '@/api/queryKeys';

interface ApiMsg<T = unknown> {
  success?: boolean;
  obj?: T;
  msg?: string;
}

async function fetchAllSetting(): Promise<unknown> {
  const msg = await HttpUtil.post('/panel/setting/all', undefined, { silent: true }) as ApiMsg;
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch settings');
  return msg.obj;
}

export function useAllSettings() {
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState<AllSetting>(() => new AllSetting());
  const [extraSpinning, setExtraSpinning] = useState(false);

  const query = useQuery({
    queryKey: keys.settings.all(),
    queryFn: fetchAllSetting,
    staleTime: Infinity,
  });

  const server = useMemo(() => new AllSetting(query.data), [query.data]);

  useEffect(() => {
    if (query.data !== undefined) {
      setDraft(new AllSetting(query.data));
    }
  }, [query.data]);

  const updateSetting = useCallback((patch: Partial<AllSetting>) => {
    setDraft((prev) => {
      const next = new AllSetting(prev);
      Object.assign(next, patch);
      return next;
    });
  }, []);

  const saveMut = useMutation({
    mutationFn: async (next: AllSetting) =>
      HttpUtil.post('/panel/setting/update', next) as Promise<ApiMsg>,
    onSuccess: (msg) => {
      if (msg?.success) queryClient.invalidateQueries({ queryKey: keys.settings.all() });
    },
  });

  const saveAll = useCallback(() => saveMut.mutateAsync(draft), [saveMut, draft]);
  const saveDisabled = useMemo(() => server.equals(draft), [server, draft]);

  return {
    allSetting: draft,
    updateSetting,
    fetched: query.data !== undefined,
    spinning: extraSpinning || saveMut.isPending,
    setSpinning: setExtraSpinning,
    saveDisabled,
    saveAll,
  };
}
