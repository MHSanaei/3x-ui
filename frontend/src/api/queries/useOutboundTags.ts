import { useQuery } from '@tanstack/react-query';

import { keys } from '@/api/queryKeys';
import { fetchXrayConfig } from '@/hooks/useXraySetting';

// Available outbound (and balancer-eligible) tags the user can route an mtproto
// inbound's Telegram traffic to. Shares the cached xray config query so opening
// the inbound form costs no extra request when the Xray page was already
// visited; `select` derives just the tag list without disturbing other readers.
export function useOutboundTags(opts?: { excludeBlackhole?: boolean }) {
  const excludeBlackhole = opts?.excludeBlackhole ?? false;
  return useQuery({
    queryKey: keys.xray.config(),
    queryFn: fetchXrayConfig,
    staleTime: Infinity,
    select: (data): string[] => {
      const tags = new Set<string>();
      for (const o of data?.xraySetting?.outbounds ?? []) {
        const ob = o as { tag?: string; protocol?: string } | null;
        if (!ob?.tag) continue;
        if (excludeBlackhole && ob.protocol === 'blackhole') continue;
        tags.add(ob.tag);
      }
      for (const t of data?.subscriptionOutboundTags ?? []) {
        if (t) tags.add(t);
      }
      // Balancers are valid routing targets too — injectMtprotoEgress emits a
      // balancerTag rule when the chosen tag names a balancer.
      const balancers = (data?.xraySetting?.routing as { balancers?: Array<{ tag?: string }> } | undefined)?.balancers;
      for (const b of balancers ?? []) {
        if (b?.tag) tags.add(b.tag);
      }
      return [...tags];
    },
  });
}
