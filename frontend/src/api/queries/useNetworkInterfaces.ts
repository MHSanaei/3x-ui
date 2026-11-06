import { useQuery } from '@tanstack/react-query';

import { NetworkInterfacesResponseSchema } from '@/generated/zod';
import { keys } from '@/api/queryKeys';
import { HttpUtil } from '@/utils';

export function useNetworkInterfaces(enabled: boolean) {
  return useQuery({
    enabled,
    queryKey: keys.server.networkInterfaces(),
    queryFn: async () => {
      const msg = await HttpUtil.get('/panel/api/server/networkInterfaces', undefined, {
        silent: true,
      });
      if (!msg?.success) throw new Error(msg?.msg || 'Failed to load network interfaces');
      return NetworkInterfacesResponseSchema.parse(msg.obj);
    },
    staleTime: 10_000,
    refetchInterval: 15_000,
  });
}
