import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber, Select, Switch } from 'antd';

import { ShadowsocksOutboundFormSettingsSchema } from '@/schemas/forms/outbound-form';
import { SSMethodSchema } from '@/schemas/protocols/shared/shadowsocks';
import { antdRule } from '@/utils/zodForm';

import { SS_METHOD_OPTIONS } from '../outbound-form-constants';

export default function ShadowsocksFields() {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item
        label={t('password')}
        name={['settings', 'password']}
        rules={[antdRule(ShadowsocksOutboundFormSettingsSchema.shape.password, t)]}
      >
        <Input />
      </Form.Item>
      <Form.Item
        label={t('encryption')}
        name={['settings', 'method']}
        rules={[antdRule(SSMethodSchema, t)]}
      >
        <Select options={SS_METHOD_OPTIONS} />
      </Form.Item>
      <Form.Item
        label={t('pages.xray.outboundForm.udpOverTcp')}
        name={['settings', 'uot']}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      <Form.Item label={t('pages.xray.outboundForm.uotVersion')} name={['settings', 'UoTVersion']}>
        <InputNumber min={1} max={2} />
      </Form.Item>
    </>
  );
}
