import { useMutation, useQueryClient } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';
import type { SubBalancerFormValues } from '@/schemas/subBalancer';

// Deliberately urlencoded (no JSON headers): the Go side binds inboundIds from
// repeated form keys, which is exactly how HttpUtil encodes arrays. Weights go
// as one JSON string — gin cannot bind bracket-keyed maps from form bodies.
function toWirePayload(values: SubBalancerFormValues): Record<string, unknown> {
  const { memberWeights, ...rest } = values;
  if (values.strategy === 'leastLoad' && memberWeights && Object.keys(memberWeights).length > 0) {
    return { ...rest, memberWeights: JSON.stringify(memberWeights) };
  }
  return rest;
}

export function useSubBalancerMutations() {
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: keys.subBalancers.root() });

  const createMut = useMutation({
    mutationFn: (payload: SubBalancerFormValues) =>
      HttpUtil.post('/panel/api/sub-balancers', toWirePayload(payload)),
    onSuccess: (msg) => {
      if (msg?.success) invalidate();
    },
  });

  const updateMut = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: SubBalancerFormValues }) =>
      HttpUtil.post(`/panel/api/sub-balancers/${id}`, toWirePayload(payload)),
    onSuccess: (msg) => {
      if (msg?.success) invalidate();
    },
  });

  const removeMut = useMutation({
    mutationFn: (id: number) => HttpUtil.post(`/panel/api/sub-balancers/${id}/del`),
    onSuccess: (msg) => {
      if (msg?.success) invalidate();
    },
  });

  return {
    create: (payload: SubBalancerFormValues) => createMut.mutateAsync(payload),
    update: (id: number, payload: SubBalancerFormValues) => updateMut.mutateAsync({ id, payload }),
    remove: (id: number) => removeMut.mutateAsync(id),
  };
}
