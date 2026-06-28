import { useCallback, useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { HttpUtil, Msg } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { AllSetting } from '@/models/setting';
import { AllSettingSchema, type AllSettingInput } from '@/schemas/setting';
import { keys } from '@/api/queryKeys';

type SettingSavePayload = Partial<AllSetting> & Record<string, unknown>;

async function fetchAllSetting(): Promise<AllSettingInput | null> {
  const msg = await HttpUtil.post('/panel/api/setting/all', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch settings');
  const validated = parseMsg(msg, AllSettingSchema, 'setting/all');
  return validated.obj;
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
    mutationFn: async (next: SettingSavePayload): Promise<Msg<unknown>> => {
      const payload = { ...next };
      const body = AllSettingSchema.partial().safeParse(payload);
      if (!body.success) {
        console.warn('[zod] setting/update body failed validation', body.error.issues);
      }
      return HttpUtil.post('/panel/api/setting/update', body.success ? { ...payload, ...body.data } : payload);
    },
    onSuccess: (msg) => {
      if (msg?.success) queryClient.invalidateQueries({ queryKey: keys.settings.all() });
    },
  });

  const saveAll = useCallback(() => saveMut.mutateAsync({ ...draft }), [saveMut, draft]);
  const savePayload = useCallback((payload: SettingSavePayload) => saveMut.mutateAsync(payload), [saveMut]);
  const saveDisabled = useMemo(() => server.equals(draft), [server, draft]);

  return {
    allSetting: draft,
    updateSetting,
    fetched: query.data !== undefined,
    spinning: extraSpinning || saveMut.isPending,
    setSpinning: setExtraSpinning,
    saveDisabled,
    saveAll,
    savePayload,
  };
}
