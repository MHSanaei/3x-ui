import { useTranslation } from 'react-i18next';
import { Input, Switch } from 'antd';

import { FormField } from '@/components/form/rhf';

export default function GrpcForm() {
  const { t } = useTranslation();
  return (
    <>
      <FormField
        label={t('pages.inbounds.form.serviceName')}
        name={['streamSettings', 'grpcSettings', 'serviceName']}
      >
        <Input />
      </FormField>
      <FormField
        label={t('pages.inbounds.form.authority')}
        name={['streamSettings', 'grpcSettings', 'authority']}
      >
        <Input />
      </FormField>
      <FormField
        label={t('pages.inbounds.form.multiMode')}
        name={['streamSettings', 'grpcSettings', 'multiMode']}
        valueProp="checked"
      >
        <Switch />
      </FormField>
    </>
  );
}
