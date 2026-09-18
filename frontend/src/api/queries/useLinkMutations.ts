import { useMutation, useQueryClient } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';
import {
  toAssignRequest,
  toLinkSaveRequest,
  type LinkAssignValues,
  type LinkSaveValues,
} from '@/schemas/api/link';

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } } as const;

export function useLinkMutations() {
  const queryClient = useQueryClient();
  // A library row is shared, so an edit here changes what every client that
  // inherits it receives — the client views are stale for the same reason.
  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: keys.links.root() });
    queryClient.invalidateQueries({ queryKey: keys.clients.root() });
  };

  const saveMut = useMutation({
    mutationFn: (payload: LinkSaveValues) =>
      HttpUtil.post('/panel/api/links/add', toLinkSaveRequest(payload), JSON_HEADERS),
    onSuccess: (msg) => {
      if (msg?.success) invalidate();
    },
  });

  const removeMut = useMutation({
    mutationFn: (id: number) => HttpUtil.post(`/panel/api/links/del/${id}`),
    onSuccess: (msg) => {
      if (msg?.success) invalidate();
    },
  });

  const setEnableMut = useMutation({
    mutationFn: ({ id, enable }: { id: number; enable: boolean }) =>
      HttpUtil.post(`/panel/api/links/enable/${id}`, { enable }, JSON_HEADERS),
    onSuccess: (msg) => {
      if (msg?.success) invalidate();
    },
  });

  const reorderMut = useMutation({
    mutationFn: (ids: number[]) => HttpUtil.post('/panel/api/links/reorder', { ids }, JSON_HEADERS),
    onSuccess: (msg) => {
      if (msg?.success) invalidate();
    },
  });

  const assignMut = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: LinkAssignValues }) =>
      HttpUtil.post(`/panel/api/links/assign/${id}`, toAssignRequest(payload), JSON_HEADERS),
    onSuccess: (msg) => {
      if (msg?.success) invalidate();
    },
  });

  const unassignMut = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: LinkAssignValues }) =>
      HttpUtil.post(`/panel/api/links/unassign/${id}`, toAssignRequest(payload), JSON_HEADERS),
    onSuccess: (msg) => {
      if (msg?.success) invalidate();
    },
  });

  const refreshMut = useMutation({
    mutationFn: (id: number) => HttpUtil.post(`/panel/api/links/refresh/${id}`),
    onSuccess: (msg) => {
      if (msg?.success) invalidate();
    },
  });

  return {
    save: (payload: LinkSaveValues) => saveMut.mutateAsync(payload),
    remove: (id: number) => removeMut.mutateAsync(id),
    setEnable: (id: number, enable: boolean) => setEnableMut.mutateAsync({ id, enable }),
    reorder: (ids: number[]) => reorderMut.mutateAsync(ids),
    assign: (id: number, payload: LinkAssignValues) => assignMut.mutateAsync({ id, payload }),
    unassign: (id: number, payload: LinkAssignValues) => unassignMut.mutateAsync({ id, payload }),
    refresh: (id: number) => refreshMut.mutateAsync(id),
  };
}
