import { useMutation, useQueryClient } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';
import type { ChainProxyFormValues, ChainSelectorFormValues } from '@/schemas/chainProxy';

// Deliberately urlencoded: Go binds proxyIds from repeated form keys, which the
// gin binder cannot do from a JSON array in a form body.
export function useChainProxyMutations() {
  const queryClient = useQueryClient();
  const invalidateProxies = () =>
    queryClient.invalidateQueries({ queryKey: keys.chainProxies.root() });
  const invalidateSelectors = () =>
    queryClient.invalidateQueries({ queryKey: keys.chainSelectors.root() });

  const createProxy = useMutation({
    mutationFn: (payload: ChainProxyFormValues) =>
      HttpUtil.post('/panel/api/chain-proxies', payload),
    onSuccess: (msg) => {
      if (msg?.success) invalidateProxies();
    },
  });

  const updateProxy = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: ChainProxyFormValues }) =>
      HttpUtil.post(`/panel/api/chain-proxies/${id}`, payload),
    onSuccess: (msg) => {
      if (msg?.success) invalidateProxies();
    },
  });

  const removeProxy = useMutation({
    mutationFn: (id: number) => HttpUtil.post(`/panel/api/chain-proxies/${id}/del`),
    onSuccess: (msg) => {
      if (msg?.success) {
        invalidateProxies();
        // A deleted hop is stripped from selectors and inbound assignments.
        invalidateSelectors();
        queryClient.invalidateQueries({ queryKey: keys.inbounds.root() });
      }
    },
  });

  const createSelector = useMutation({
    mutationFn: (payload: ChainSelectorFormValues) =>
      HttpUtil.post('/panel/api/chain-selectors', payload),
    onSuccess: (msg) => {
      if (msg?.success) invalidateSelectors();
    },
  });

  const updateSelector = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: ChainSelectorFormValues }) =>
      HttpUtil.post(`/panel/api/chain-selectors/${id}`, payload),
    onSuccess: (msg) => {
      if (msg?.success) invalidateSelectors();
    },
  });

  const removeSelector = useMutation({
    mutationFn: (id: number) => HttpUtil.post(`/panel/api/chain-selectors/${id}/del`),
    onSuccess: (msg) => {
      if (msg?.success) {
        invalidateSelectors();
        queryClient.invalidateQueries({ queryKey: keys.inbounds.root() });
      }
    },
  });

  return {
    createProxy: (payload: ChainProxyFormValues) => createProxy.mutateAsync(payload),
    updateProxy: (id: number, payload: ChainProxyFormValues) =>
      updateProxy.mutateAsync({ id, payload }),
    removeProxy: (id: number) => removeProxy.mutateAsync(id),
    createSelector: (payload: ChainSelectorFormValues) => createSelector.mutateAsync(payload),
    updateSelector: (id: number, payload: ChainSelectorFormValues) =>
      updateSelector.mutateAsync({ id, payload }),
    removeSelector: (id: number) => removeSelector.mutateAsync(id),
  };
}
