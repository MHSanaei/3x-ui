import { useTranslation } from 'react-i18next';
import { Input, InputNumber, Select, Switch } from 'antd';

import { HeaderMapEditor } from '@/components/form';
import { FormField } from '@/components/form/rhf';

export default function TunnelFields() {
  const { t } = useTranslation();
  return (
    <>
      <FormField name={['settings', 'rewriteAddress']} label={t('pages.inbounds.form.rewriteAddress')}>
        <Input />
      </FormField>
      <FormField name={['settings', 'rewritePort']} label={t('pages.inbounds.form.rewritePort')}>
        <InputNumber min={0} max={65535} />
      </FormField>
      <FormField name={['settings', 'allowedNetwork']} label={t('pages.inbounds.form.allowedNetwork')}>
        <Select
          options={[
            { value: 'tcp,udp', label: 'TCP, UDP' },
            { value: 'tcp', label: 'TCP' },
            { value: 'udp', label: 'UDP' },
          ]}
        />
      </FormField>
      <FormField label={t('pages.inbounds.portMap')} name={['settings', 'portMap']}>
        <HeaderMapEditor mode="v1" />
      </FormField>
      <FormField
        name={['settings', 'followRedirect']}
        label={t('pages.inbounds.form.followRedirect')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
    </>
  );
}
