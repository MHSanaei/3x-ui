import { useTranslation } from 'react-i18next';
import { Input, InputNumber, Select, Switch } from 'antd';

import { FormField, rhfZodValidate } from '@/components/form/rhf';
import { ShadowsocksOutboundFormSettingsSchema } from '@/schemas/forms/outbound-form';
import { SSMethodSchema } from '@/schemas/protocols/shared/shadowsocks';

import { SS_METHOD_OPTIONS } from '../outbound-form-constants';

export default function ShadowsocksFields() {
  const { t } = useTranslation();
  return (
    <>
      <FormField
        label={t('password')}
        name={['settings', 'password']}
        rules={{ validate: rhfZodValidate(ShadowsocksOutboundFormSettingsSchema.shape.password) }}
      >
        <Input />
      </FormField>
      <FormField
        label={t('encryption')}
        name={['settings', 'method']}
        rules={{ validate: rhfZodValidate(SSMethodSchema) }}
      >
        <Select options={SS_METHOD_OPTIONS} />
      </FormField>
      <FormField
        label={t('pages.xray.outboundForm.udpOverTcp')}
        name={['settings', 'uot']}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <FormField label={t('pages.xray.outboundForm.uotVersion')} name={['settings', 'UoTVersion']}>
        <InputNumber min={1} max={2} />
      </FormField>
    </>
  );
}
