import { useQuery } from '@tanstack/react-query';
import { ReadinessSchema, type Readiness } from '@/generated/zod';
import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';

export type KernelReadiness = Readiness;

async function fetchKernelReadiness(): Promise<KernelReadiness> {
  const msg = await HttpUtil.get('/panel/api/inbounds/amneziawgKernelReadiness', undefined, {
    silent: true,
  });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch kernel readiness');
  return ReadinessSchema.parse(msg.obj);
}

// Probing the module is cheap but not free, and the answer only changes when an
// admin installs something, so a short stale window is plenty.
export function useAmneziawgKernelReadiness(enabled: boolean) {
  return useQuery({
    queryKey: keys.inbounds.amneziawgKernelReadiness(),
    queryFn: fetchKernelReadiness,
    enabled,
    staleTime: 30_000,
  });
}
