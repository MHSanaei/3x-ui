import { MuxForm } from '@/pages/xray/outbounds/transport';

import OutboundSubtreeJsonForm from './OutboundSubtreeJsonForm';
import { serializeOverride } from './helpers';

/*
 * Mux override editor — reuses the outbound MuxForm (same fields as the sub-JSON
 * settings editor). Stored in the host's muxParams JSON string. Defaults match
 * the sub-JSON editor; the host stores '' (= inherit the inbound/global mux)
 * when the toggle is off, an explicit mux object when on.
 */
const DEFAULT_MUX = { enabled: false, concurrency: 8, xudpConcurrency: 16, xudpProxyUDP443: 'reject' };

export default function HostMuxForm({ value, onChange }: { value?: string; onChange?: (next: string) => void }) {
  return (
    <OutboundSubtreeJsonForm
      value={value}
      onChange={onChange}
      path={['mux']}
      defaultSubtree={DEFAULT_MUX}
      serialize={(mux) => ((mux as { enabled?: boolean } | undefined)?.enabled ? serializeOverride(mux) : '')}
      /* protocol/network are fixed only to satisfy MuxForm's isMuxAllowed gate;
         a host's mux override is protocol-agnostic and should always be editable. */
      render={() => <MuxForm protocol="vmess" network="tcp" />}
    />
  );
}
