import { SockoptForm } from '@/pages/xray/outbounds/transport';

import OutboundSubtreeJsonForm from './OutboundSubtreeJsonForm';
import { serializeOverride } from './helpers';

// Sockopt override editor — reuses the outbound SockoptForm (which carries its
// own enable Switch and writes streamSettings.sockopt). Serialized to the host's
// sockoptParams JSON string.
//
// A host is the client/dialer side, so the inbound-only sockopt keys are dropped
// from the output (xray ignores them on an outbound anyway). The outbound form
// no longer shows them, but its default object still seeds them, so strip on
// serialize to keep the host's override honest to the server/client split.
// Ref: https://xtls.github.io/config/transports/sockopt.html#sockoptobject
const INBOUND_ONLY_SOCKOPT = ['tproxy', 'acceptProxyProtocol', 'V6Only', 'trustedXForwardedFor'];

function serializeClientSockopt(sockopt: unknown): string {
  if (!sockopt || typeof sockopt !== 'object') return serializeOverride(sockopt);
  const copy = { ...(sockopt as Record<string, unknown>) };
  for (const key of INBOUND_ONLY_SOCKOPT) delete copy[key];
  return serializeOverride(copy);
}

export default function HostSockoptForm({ value, onChange }: { value?: string; onChange?: (next: string) => void }) {
  return (
    <OutboundSubtreeJsonForm
      value={value}
      onChange={onChange}
      path={['streamSettings', 'sockopt']}
      serialize={serializeClientSockopt}
      render={(form) => <SockoptForm form={form} outboundTags={[]} />}
    />
  );
}
