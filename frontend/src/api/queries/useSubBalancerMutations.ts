import { useMutation, useQueryClient } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';
import type { SubBalancerFormValues } from '@/schemas/subBalancer';

// Deliberately urlencoded (no JSON headers): the Go side binds inboundIds from
// repeated form keys, which is exactly how HttpUtil encodes arrays.
export function useSubBalancerMutations() {
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: keys.subBalancers.root() });

  const createMut = useMutation({
    mutationFn: (payload: SubBalancerFormValues) => HttpUtil.post('/panel/api/sub-balancers', payload),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const updateMut = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: SubBalancerFormValues }) =>
      HttpUtil.post(`/panel/api/sub-balancers/${id}`, payload),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const removeMut = useMutation({
    mutationFn: (id: number) => HttpUtil.post(`/panel/api/sub-balancers/${id}/del`),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  return {
    create: (payload: SubBalancerFormValues) => createMut.mutateAsync(payload),
    update: (id: number, payload: SubBalancerFormValues) => updateMut.mutateAsync({ id, payload }),
    remove: (id: number) => removeMut.mutateAsync(id),
  };
}
