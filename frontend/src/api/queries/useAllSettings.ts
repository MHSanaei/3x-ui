import { useCallback, useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { HttpUtil, Msg } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { AllSetting } from '@/models/setting';
import { AllSettingSchema, type AllSettingInput } from '@/schemas/setting';
import { keys } from '@/api/queryKeys';
import { useMe } from '@/hooks/useMe';

async function fetchAllSetting(): Promise<AllSettingInput | null> {
  const msg = await HttpUtil.post('/panel/setting/all', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch settings');
  const validated = parseMsg(msg, AllSettingSchema, 'setting/all');
  return validated.obj;
}

export function useAllSettings() {
  const queryClient = useQueryClient();
  const { me } = useMe();
  const [draft, setDraft] = useState<AllSetting>(() => new AllSetting());
  const [extraSpinning, setExtraSpinning] = useState(false);

  // The full settings payload is an admin-only endpoint. Non-admins (who never
  // see the settings UI) skip the fetch entirely and fall back to defaults so
  // the shared sidebar that consumes this hook doesn't 403.
  const query = useQuery({
    queryKey: keys.settings.all(),
    queryFn: fetchAllSetting,
    staleTime: Infinity,
    enabled: me === undefined || me.isAdmin,
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
    mutationFn: async (next: AllSetting): Promise<Msg<unknown>> => {
      const body = AllSettingSchema.partial().safeParse(next);
      if (!body.success) {
        console.warn('[zod] setting/update body failed validation', body.error.issues);
      }
      return HttpUtil.post('/panel/setting/update', body.success ? body.data : next);
    },
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
