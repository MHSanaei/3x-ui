import { useTranslation } from 'react-i18next';

import { XhttpForm } from '@/pages/xray/outbounds/transport';

import OutboundSubtreeJsonForm from './OutboundSubtreeJsonForm';

// XHTTP extra-params override editor — reuses the outbound XhttpForm under an
// enable Switch (XhttpForm has no toggle of its own). The non-empty fields are
// merged into the inbound stream's xhttpSettings; serialized to the host's
// xhttpExtraParams JSON string.
export default function HostXhttpForm({ value, onChange }: { value?: string; onChange?: (next: string) => void }) {
  const { t } = useTranslation();
  return (
    <OutboundSubtreeJsonForm
      value={value}
      onChange={onChange}
      path={['streamSettings', 'xhttpSettings']}
      enableSwitch
      enableLabel={t('pages.hosts.fields.xhttpExtraParams')}
      render={(form) => (
        <XhttpForm
          form={form}
          onXmuxToggle={(checked) =>
            form.setFieldValue(
              ['streamSettings', 'xhttpSettings', 'xmux'],
              checked ? (form.getFieldValue(['streamSettings', 'xhttpSettings', 'xmux']) ?? {}) : undefined,
            )
          }
        />
      )}
    />
  );
}
