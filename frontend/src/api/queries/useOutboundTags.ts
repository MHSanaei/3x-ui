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

export interface OutboundTagGroups {
  outbounds: string[];
  balancers: string[];
}

// Same data as useOutboundTags, but keeps outbound and balancer tags apart so a
// picker can render them in labeled groups (like the panel-outbound selector)
// instead of one flat list.
export function useOutboundTagGroups(opts?: { excludeBlackhole?: boolean }) {
  const excludeBlackhole = opts?.excludeBlackhole ?? false;
  return useQuery({
    queryKey: keys.xray.config(),
    queryFn: fetchXrayConfig,
    staleTime: Infinity,
    select: (data): OutboundTagGroups => {
      const outbounds = new Set<string>();
      for (const o of data?.xraySetting?.outbounds ?? []) {
        const ob = o as { tag?: string; protocol?: string } | null;
        if (!ob?.tag) continue;
        if (excludeBlackhole && ob.protocol === 'blackhole') continue;
        outbounds.add(ob.tag);
      }
      for (const t of data?.subscriptionOutboundTags ?? []) {
        if (t) outbounds.add(t);
      }
      const balancers: string[] = [];
      const bal = (data?.xraySetting?.routing as { balancers?: Array<{ tag?: string }> } | undefined)?.balancers;
      for (const b of bal ?? []) {
        if (b?.tag && !outbounds.has(b.tag)) balancers.push(b.tag);
      }
      return { outbounds: [...outbounds], balancers };
    },
  });
}
