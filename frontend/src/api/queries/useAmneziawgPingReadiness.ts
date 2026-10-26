import { useQuery } from '@tanstack/react-query';
import { AWGPingReadinessSchema } from '@/schemas/primitives/awg-ping';
import { HttpUtil } from '@/utils';

export function useAmneziawgPingReadiness(enabled: boolean) {
  return useQuery({
    queryKey: ['inbounds', 'amneziawgPingReadiness'],
    enabled,
    staleTime: 30_000,
    queryFn: async () => {
      const msg = await HttpUtil.get('/panel/api/inbounds/amneziawgPingReadiness', undefined, {
        silent: true,
      });
      if (!msg?.success) throw new Error(msg?.msg || 'Ping readiness unavailable');
      return AWGPingReadinessSchema.parse(msg.obj);
    },
  });
}
