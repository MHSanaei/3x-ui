import { useTranslation } from 'react-i18next';
import { Input, Switch } from 'antd';

import { FormField } from '@/components/form/rhf';

export default function GrpcForm() {
  const { t } = useTranslation();
  return (
    <>
      <FormField
        name={['streamSettings', 'grpcSettings', 'serviceName']}
        label={t('pages.inbounds.form.serviceName')}
      >
        <Input />
      </FormField>
      <FormField
        name={['streamSettings', 'grpcSettings', 'authority']}
        label={t('pages.inbounds.form.authority')}
      >
        <Input />
      </FormField>
      <FormField
        name={['streamSettings', 'grpcSettings', 'multiMode']}
        label={t('pages.inbounds.form.multiMode')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
    </>
  );
}
