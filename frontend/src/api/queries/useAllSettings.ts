import { useCallback, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { HttpUtil, Msg } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { AllSetting } from '@/models/setting';
import { AllSettingSchema, type AllSettingInput } from '@/schemas/setting';
import { keys } from '@/api/queryKeys';
import { useServerDraft } from '@/hooks/useServerDraft';

type SettingSavePayload = Partial<AllSetting> & Record<string, unknown>;
type SettingSaveResult = {
  msg: Msg<unknown>;
  saved?: AllSetting;
};

async function fetchAllSetting(): Promise<AllSettingInput | null> {
  const msg = await HttpUtil.post('/panel/api/setting/all', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch settings');
  const validated = parseMsg(msg, AllSettingSchema, 'setting/all');
  return validated.obj;
}

export function useAllSettings() {
  const queryClient = useQueryClient();
  const [extraSpinning, setExtraSpinning] = useState(false);

  const query = useQuery({
    queryKey: keys.settings.all(),
    queryFn: fetchAllSetting,
    staleTime: Infinity,
  });

  const server = useMemo(() => new AllSetting(query.data), [query.data]);
  const { draft, setDraft, isDirty, markSaved } = useServerDraft(
    query.data === undefined ? undefined : server,
    (setting) => new AllSetting(setting),
    (left, right) => left.equals(right),
  );
  const allSetting = draft ?? server;

  const updateSetting = useCallback((patch: Partial<AllSetting>) => {
    setDraft((prev) => {
      const next = new AllSetting(prev ?? server);
      Object.assign(next, patch);
      return next;
    });
  }, [server, setDraft]);

  const saveMut = useMutation({
    mutationFn: async ({ payload, saved }: { payload: SettingSavePayload; saved?: AllSetting }): Promise<SettingSaveResult> => {
      const next = { ...payload };
      const body = AllSettingSchema.partial().safeParse(next);
      if (!body.success) {
        console.warn('[zod] setting/update body failed validation', body.error.issues);
      }
      const msg = await HttpUtil.post('/panel/api/setting/update', body.success ? { ...next, ...body.data } : next);
      return { msg, saved };
    },
    onSuccess: ({ msg, saved }) => {
      if (!msg?.success) return;
      if (saved) markSaved(saved);
      queryClient.invalidateQueries({ queryKey: keys.settings.all() });
    },
  });

  const saveAll = useCallback(async () => {
    const saved = new AllSetting(allSetting);
    return (await saveMut.mutateAsync({ payload: { ...saved }, saved })).msg;
  }, [allSetting, saveMut]);
  const savePayload = useCallback(
    async (payload: SettingSavePayload) => {
      const saved = new AllSetting(allSetting);
      Object.assign(saved, payload);
      return (await saveMut.mutateAsync({ payload, saved })).msg;
    },
    [allSetting, saveMut],
  );
  const saveDisabled = !isDirty;

  return {
    allSetting,
    updateSetting,
    fetched: query.data !== undefined,
    spinning: extraSpinning || saveMut.isPending,
    setSpinning: setExtraSpinning,
    saveDisabled,
    saveAll,
    savePayload,
  };
}
