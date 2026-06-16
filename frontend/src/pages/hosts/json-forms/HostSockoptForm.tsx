import { SockoptForm } from '@/pages/xray/outbounds/transport';

import OutboundSubtreeJsonForm from './OutboundSubtreeJsonForm';

// Sockopt override editor — reuses the outbound SockoptForm (which carries its
// own enable Switch and writes streamSettings.sockopt). Serialized to the host's
// sockoptParams JSON string.
export default function HostSockoptForm({ value, onChange }: { value?: string; onChange?: (next: string) => void }) {
  return (
    <OutboundSubtreeJsonForm
      value={value}
      onChange={onChange}
      path={['streamSettings', 'sockopt']}
      render={(form) => <SockoptForm form={form} outboundTags={[]} />}
    />
  );
}
