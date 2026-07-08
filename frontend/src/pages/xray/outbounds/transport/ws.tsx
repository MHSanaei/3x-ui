import { useTranslation } from 'react-i18next';
import { Input, InputNumber } from 'antd';

import { HeaderMapEditor } from '@/components/form';
import { FormField } from '@/components/form/rhf';

export default function WsForm() {
  const { t } = useTranslation();
  return (
    <>
      <FormField label={t('host')} name={['streamSettings', 'wsSettings', 'host']}>
        <Input />
      </FormField>
      <FormField label={t('path')} name={['streamSettings', 'wsSettings', 'path']}>
        <Input />
      </FormField>
      <FormField
        label={t('pages.inbounds.form.heartbeatPeriod')}
        name={['streamSettings', 'wsSettings', 'heartbeatPeriod']}
      >
        <InputNumber min={0} />
      </FormField>
      <FormField
        label={t('pages.inbounds.form.headers')}
        name={['streamSettings', 'wsSettings', 'headers']}
      >
        <HeaderMapEditor mode="v1" />
      </FormField>
    </>
  );
}
