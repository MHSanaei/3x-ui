import { useTranslation } from 'react-i18next';
import { Input, InputNumber, Switch } from 'antd';

import { HeaderMapEditor } from '@/components/form';
import { FormField } from '@/components/form/rhf';

export default function WsForm() {
  const { t } = useTranslation();
  return (
    <>
      <FormField
        name={['streamSettings', 'wsSettings', 'acceptProxyProtocol']}
        label={t('pages.inbounds.form.proxyProtocol')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <FormField name={['streamSettings', 'wsSettings', 'host']} label={t('host')}>
        <Input />
      </FormField>
      <FormField name={['streamSettings', 'wsSettings', 'path']} label={t('path')}>
        <Input />
      </FormField>
      <FormField
        name={['streamSettings', 'wsSettings', 'heartbeatPeriod']}
        label={t('pages.inbounds.form.heartbeatPeriod')}
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
