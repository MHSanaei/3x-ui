import { useMutation, useQueryClient } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';
import type { HostFormValues } from '@/schemas/api/host';

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };

export function useHostMutations() {
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: keys.hosts.root() });

  const createMut = useMutation({
    mutationFn: (payload: Partial<HostFormValues>) => HttpUtil.post('/panel/api/hosts/add', payload),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const updateMut = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: Partial<HostFormValues> }) =>
      HttpUtil.post(`/panel/api/hosts/update/${id}`, payload),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const removeMut = useMutation({
    mutationFn: (id: number) => HttpUtil.post(`/panel/api/hosts/del/${id}`),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const setEnableMut = useMutation({
    mutationFn: ({ id, enable }: { id: number; enable: boolean }) =>
      HttpUtil.post(`/panel/api/hosts/setEnable/${id}`, { enable }),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const reorderMut = useMutation({
    mutationFn: (ids: number[]) => HttpUtil.post('/panel/api/hosts/reorder', { ids }, JSON_HEADERS),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const bulkEnableMut = useMutation({
    mutationFn: ({ ids, enable }: { ids: number[]; enable: boolean }) =>
      HttpUtil.post('/panel/api/hosts/bulk/setEnable', { ids, enable }, JSON_HEADERS),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const bulkDelMut = useMutation({
    mutationFn: (ids: number[]) => HttpUtil.post('/panel/api/hosts/bulk/del', { ids }, JSON_HEADERS),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  return {
    create: (payload: Partial<HostFormValues>) => createMut.mutateAsync(payload),
    update: (id: number, payload: Partial<HostFormValues>) => updateMut.mutateAsync({ id, payload }),
    remove: (id: number) => removeMut.mutateAsync(id),
    setEnable: (id: number, enable: boolean) => setEnableMut.mutateAsync({ id, enable }),
    reorder: (ids: number[]) => reorderMut.mutateAsync(ids),
    bulkSetEnable: (ids: number[], enable: boolean) => bulkEnableMut.mutateAsync({ ids, enable }),
    bulkDel: (ids: number[]) => bulkDelMut.mutateAsync(ids),
  };
}
