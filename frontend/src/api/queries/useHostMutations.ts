import { useMutation, useQueryClient } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';
import type { BulkAddHostValues } from '@/schemas/api/host';

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };

export function useHostMutations() {
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: keys.hosts.root() });

  const bulkCreateMut = useMutation({
    mutationFn: (payload: BulkAddHostValues) => HttpUtil.post('/panel/api/hosts/bulk/add', payload, JSON_HEADERS),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const updateMut = useMutation({
    mutationFn: ({ groupId, payload }: { groupId: string; payload: BulkAddHostValues }) =>
      HttpUtil.post(`/panel/api/hosts/update/${groupId}`, payload, JSON_HEADERS),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const removeMut = useMutation({
    mutationFn: (groupId: string) => HttpUtil.post(`/panel/api/hosts/del/${groupId}`),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const setEnableMut = useMutation({
    mutationFn: ({ groupId, enable }: { groupId: string; enable: boolean }) =>
      HttpUtil.post(`/panel/api/hosts/setEnable/${groupId}`, { enable }),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const reorderMut = useMutation({
    mutationFn: (groupIds: string[]) => HttpUtil.post('/panel/api/hosts/reorder', { ids: groupIds }, JSON_HEADERS),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const bulkEnableMut = useMutation({
    mutationFn: ({ groupIds, enable }: { groupIds: string[]; enable: boolean }) =>
      HttpUtil.post('/panel/api/hosts/bulk/setEnable', { ids: groupIds, enable }, JSON_HEADERS),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const bulkDelMut = useMutation({
    mutationFn: (groupIds: string[]) => HttpUtil.post('/panel/api/hosts/bulk/del', { ids: groupIds }, JSON_HEADERS),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  return {
    bulkCreate: (payload: BulkAddHostValues) => bulkCreateMut.mutateAsync(payload),
    update: (groupId: string, payload: BulkAddHostValues) => updateMut.mutateAsync({ groupId, payload }),
    remove: (groupId: string) => removeMut.mutateAsync(groupId),
    setEnable: (groupId: string, enable: boolean) => setEnableMut.mutateAsync({ groupId, enable }),
    reorder: (groupIds: string[]) => reorderMut.mutateAsync(groupIds),
    bulkSetEnable: (groupIds: string[], enable: boolean) => bulkEnableMut.mutateAsync({ groupIds, enable }),
    bulkDel: (groupIds: string[]) => bulkDelMut.mutateAsync(groupIds),
  };
}
