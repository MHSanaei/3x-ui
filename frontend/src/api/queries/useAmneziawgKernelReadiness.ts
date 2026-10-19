import { useQuery } from '@tanstack/react-query';
import { z } from 'zod';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';

// Mirrors amneziawgkern.Readiness. Blockers carry a machine code so the panel
// translates them itself rather than rendering a server-side English string.
export const KernelReadinessSchema = z.object({
  ready: z.boolean(),
  blockers: z
    .array(z.object({ code: z.string(), detail: z.string().optional() }))
    .nullish()
    .transform((v) => v ?? []),
  installCommand: z.string().optional().default(''),
});

export type KernelReadiness = z.infer<typeof KernelReadinessSchema>;

async function fetchKernelReadiness(): Promise<KernelReadiness> {
  const msg = await HttpUtil.get('/panel/api/inbounds/amneziawgKernelReadiness', undefined, {
    silent: true,
  });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch kernel readiness');
  return KernelReadinessSchema.parse(msg.obj);
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
