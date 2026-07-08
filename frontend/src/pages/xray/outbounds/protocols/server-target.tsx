import { useTranslation } from 'react-i18next';
import { Input, InputNumber } from 'antd';

import { FormField } from '@/components/form/rhf';

export default function ServerTarget() {
  const { t } = useTranslation();
  return (
    <>
      <FormField
        label={t('pages.inbounds.address')}
        name={['settings', 'address']}
        required
        rules={{ required: 'pages.xray.outboundForm.addressRequired' }}
      >
        <Input />
      </FormField>
      <FormField
        label={t('pages.inbounds.port')}
        name={['settings', 'port']}
        required
        rules={{ required: 'pages.xray.outboundForm.portRequired' }}
      >
        <InputNumber min={1} max={65535} style={{ width: '100%' }} />
      </FormField>
    </>
  );
}
