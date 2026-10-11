import { useTranslation } from 'react-i18next';
import { Input, InputNumber, Select } from 'antd';

import { FormField } from '@/components/form/rhf';
import { MASQUE_DEFAULT_PATH } from '@/schemas/protocols/stream/masque';

export function MasqueSettingsFields() {
  const { t } = useTranslation();
  return (
    <>
      <FormField
        label={t('pages.inbounds.form.masque.addressPool')}
        tooltip={t('pages.inbounds.form.masque.addressPoolDesc')}
        name={['settings', 'address']}
        required
      >
        <Select mode="tags" tokenSeparators={[',', ' ']} open={false} />
      </FormField>
      <FormField label={t('pages.inbounds.info.mtu')} name={['settings', 'mtu']}>
        <InputNumber min={1280} max={65535} placeholder="1280" style={{ width: '100%' }} />
      </FormField>
    </>
  );
}

// The server matches requests on `path` alone; host, credentials and headers are
// the client's, delivered through the JSON subscription.
export default function MasqueStreamFields() {
  const { t } = useTranslation();
  return (
    <FormField label={t('path')} name={['streamSettings', 'masqueSettings', 'path']}>
      <Input placeholder={MASQUE_DEFAULT_PATH} />
    </FormField>
  );
}
