import { useTranslation } from 'react-i18next';
import { InputNumber } from 'antd';

import { FormField } from '@/components/form/rhf';

export default function KcpForm() {
  const { t } = useTranslation();
  return (
    <>
      <FormField name={['streamSettings', 'kcpSettings', 'mtu']} label="MTU">
        <InputNumber min={576} max={1460} />
      </FormField>
      <FormField name={['streamSettings', 'kcpSettings', 'tti']} label={t('pages.inbounds.form.ttiMs')}>
        <InputNumber min={10} max={100} />
      </FormField>
      <FormField name={['streamSettings', 'kcpSettings', 'uplinkCapacity']} label={t('pages.inbounds.form.uplinkMbps')}>
        <InputNumber min={0} />
      </FormField>
      <FormField name={['streamSettings', 'kcpSettings', 'downlinkCapacity']} label={t('pages.inbounds.form.downlinkMbps')}>
        <InputNumber min={0} />
      </FormField>
      <FormField
        name={['streamSettings', 'kcpSettings', 'cwndMultiplier']}
        label={t('pages.inbounds.form.cwndMultiplier')}
      >
        <InputNumber min={1} />
      </FormField>
      <FormField
        name={['streamSettings', 'kcpSettings', 'maxSendingWindow']}
        label={t('pages.inbounds.form.maxSendingWindow')}
      >
        <InputNumber min={0} />
      </FormField>
    </>
  );
}
