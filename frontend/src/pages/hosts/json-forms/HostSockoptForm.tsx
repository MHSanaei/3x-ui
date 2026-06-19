import { SockoptForm } from '@/pages/xray/outbounds/transport';
import { useOutboundTagGroups } from '@/api/queries/useOutboundTags';

import OutboundSubtreeJsonForm from './OutboundSubtreeJsonForm';
import { serializeOverride } from './helpers';

// Sockopt override editor — reuses the outbound SockoptForm (which carries its
// own enable Switch and writes streamSettings.sockopt). Serialized to the host's
// sockoptParams JSON string.
//
// A host is the client/dialer side, so the inbound-only sockopt keys are dropped
// from the output. Verified against xray-core transport/internet/sockopt_*.go:
// only V6Only and the handler-level acceptProxyProtocol / trustedXForwardedFor
// are inbound-only — tproxy (IP_TRANSPARENT) and keepalive/interface ARE applied
// on the outbound/dialer socket, so they stay. The outbound form no longer shows
// the inbound-only keys, but its default object still seeds them, so strip here.
const INBOUND_ONLY_SOCKOPT = ['acceptProxyProtocol', 'V6Only', 'trustedXForwardedFor'];

function serializeClientSockopt(sockopt: unknown): string {
  if (!sockopt || typeof sockopt !== 'object') return serializeOverride(sockopt);
  const copy = { ...(sockopt as Record<string, unknown>) };
  for (const key of INBOUND_ONLY_SOCKOPT) delete copy[key];
  return serializeOverride(copy);
}

export default function HostSockoptForm({ value, onChange }: { value?: string; onChange?: (next: string) => void }) {
  // Populate the dialerProxy dropdown with the panel's outbound tags (a host can
  // chain through one of the subscription's outbounds by tag). dialerProxy chains
  // through a single outbound, so balancers (routing targets) are excluded — only
  // the outbound group is used; blackhole is dropped too (chaining to it just
  // drops the traffic).
  const { data: tagGroups } = useOutboundTagGroups({ excludeBlackhole: true });
  const outboundTags = tagGroups?.outbounds ?? [];
  return (
    <OutboundSubtreeJsonForm
      value={value}
      onChange={onChange}
      path={['streamSettings', 'sockopt']}
      serialize={serializeClientSockopt}
      render={(form) => <SockoptForm form={form} outboundTags={outboundTags} />}
    />
  );
}
