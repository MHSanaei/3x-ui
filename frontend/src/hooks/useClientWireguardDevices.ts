import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { keys } from '@/api/queryKeys';
import { httpRequest } from '@/api/http-init';
import { HttpUtil } from '@/utils';
import { WireguardDevicesSchema } from '@/schemas/client';

export function useClientWireguardDevices(email: string | undefined, active: boolean) {
  const queryClient = useQueryClient();
  const queryKey = ['clients', 'wireguardDevices', email];
  const path = '/panel/api/clients/wireguardDevices';
  const emailQuery = `?email=${encodeURIComponent(email ?? '')}`;
  const query = useQuery({
    queryKey,
    enabled: active && !!email,
    queryFn: async () => {
      const msg = await HttpUtil.get(path + emailQuery);
      if (!msg.success) throw new Error(msg.msg);
      return WireguardDevicesSchema.parse(msg.obj);
    },
  });
  const mutation = useMutation({
    mutationFn: async ({
      method,
      suffix,
      body,
    }: {
      method: string;
      suffix: string;
      body?: unknown;
    }) => {
      const response = await httpRequest(method, path + suffix + emailQuery, body, {
        headers: { 'Content-Type': 'application/json' },
      });
      const msg = HttpUtil._respToMsg(response);
      HttpUtil._handleMsg(msg);
      if (!msg.success) throw new Error(msg.msg);
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.clients.root() }),
  });
  return {
    data: query.data,
    loading: query.isLoading,
    error: query.error,
    pending: mutation.isPending,
    mutationError: mutation.error,
    create: (name: string) => mutation.mutate({ method: 'POST', suffix: '', body: { name } }),
    update: (id: string, body: { name?: string; enable?: boolean }) =>
      mutation.mutate({
        method: 'PATCH',
        suffix: `/${encodeURIComponent(id)}`,
        body,
      }),
    remove: (id: string) =>
      mutation.mutate({ method: 'DELETE', suffix: `/${encodeURIComponent(id)}` }),
  };
}
