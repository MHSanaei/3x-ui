import { useTranslation } from 'react-i18next';
import { InputNumber } from 'antd';

import { FormField } from '@/components/form/rhf';

export default function KcpForm() {
  const { t } = useTranslation();
  return (
    <>
      <FormField label="MTU" name={['streamSettings', 'kcpSettings', 'mtu']}>
        <InputNumber min={0} />
      </FormField>
      <FormField label={t('pages.inbounds.form.ttiMs')} name={['streamSettings', 'kcpSettings', 'tti']}>
        <InputNumber min={0} />
      </FormField>
      <FormField
        label={t('pages.inbounds.form.uplinkMbps')}
        name={['streamSettings', 'kcpSettings', 'uplinkCapacity']}
      >
        <InputNumber min={0} />
      </FormField>
      <FormField
        label={t('pages.inbounds.form.downlinkMbps')}
        name={['streamSettings', 'kcpSettings', 'downlinkCapacity']}
      >
        <InputNumber min={0} />
      </FormField>
      <FormField
        label={t('pages.inbounds.form.cwndMultiplier')}
        name={['streamSettings', 'kcpSettings', 'cwndMultiplier']}
      >
        <InputNumber min={1} />
      </FormField>
      <FormField
        label={t('pages.inbounds.form.maxSendingWindow')}
        name={['streamSettings', 'kcpSettings', 'maxSendingWindow']}
      >
        <InputNumber min={0} />
      </FormField>
    </>
  );
}
