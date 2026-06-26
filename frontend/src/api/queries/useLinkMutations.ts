import { useMutation, useQueryClient } from '@tanstack/react-query';

import { keys } from '@/api/queryKeys';
import { LinkAssignResultSchema, type LinkAssignResult, type ManagedLinkFormValues } from '@/schemas/api/link';
import { HttpUtil, type Msg } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } } as const;

export function useLinkMutations() {
  const queryClient = useQueryClient();
  const invalidate = () => Promise.all([
    queryClient.invalidateQueries({ queryKey: keys.links.root() }),
    queryClient.invalidateQueries({ queryKey: keys.clients.root() }),
  ]);

  const createMut = useMutation({
    mutationFn: (payload: ManagedLinkFormValues) => HttpUtil.post('/panel/api/links/add', payload, JSON_HEADERS),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const updateMut = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: ManagedLinkFormValues }) =>
      HttpUtil.post(`/panel/api/links/update/${id}`, payload, JSON_HEADERS),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const removeMut = useMutation({
    mutationFn: (id: number) => HttpUtil.post(`/panel/api/links/del/${id}`),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const setEnableMut = useMutation({
    mutationFn: ({ id, enable }: { id: number; enable: boolean }) =>
      HttpUtil.post(`/panel/api/links/setEnable/${id}`, { enable }, JSON_HEADERS),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const reorderMut = useMutation({
    mutationFn: (ids: number[]) => HttpUtil.post('/panel/api/links/reorder', { ids }, JSON_HEADERS),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const assignMut = useMutation({
    mutationFn: async (payload: { linkIds: number[]; emails: string[] }): Promise<Msg<LinkAssignResult>> => {
      const raw = await HttpUtil.post('/panel/api/links/assign', payload, JSON_HEADERS);
      return parseMsg(raw, LinkAssignResultSchema, 'links/assign');
    },
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const bulkEnableMut = useMutation({
    mutationFn: ({ ids, enable }: { ids: number[]; enable: boolean }) =>
      HttpUtil.post('/panel/api/links/bulk/setEnable', { ids, enable }, JSON_HEADERS),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const bulkDelMut = useMutation({
    mutationFn: (ids: number[]) => HttpUtil.post('/panel/api/links/bulk/del', { ids }, JSON_HEADERS),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  return {
    create: (payload: ManagedLinkFormValues) => createMut.mutateAsync(payload),
    update: (id: number, payload: ManagedLinkFormValues) => updateMut.mutateAsync({ id, payload }),
    remove: (id: number) => removeMut.mutateAsync(id),
    setEnable: (id: number, enable: boolean) => setEnableMut.mutateAsync({ id, enable }),
    reorder: (ids: number[]) => reorderMut.mutateAsync(ids),
    assign: (linkIds: number[], emails: string[]) => assignMut.mutateAsync({ linkIds, emails }),
    bulkSetEnable: (ids: number[], enable: boolean) => bulkEnableMut.mutateAsync({ ids, enable }),
    bulkDel: (ids: number[]) => bulkDelMut.mutateAsync(ids),
  };
}
