import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber, Switch } from 'antd';

import { HeaderMapEditor } from '@/components/form';

export default function WsForm() {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item
        name={['streamSettings', 'wsSettings', 'acceptProxyProtocol']}
        label={t('pages.inbounds.form.proxyProtocol')}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      <Form.Item name={['streamSettings', 'wsSettings', 'host']} label={t('host')}>
        <Input />
      </Form.Item>
      <Form.Item name={['streamSettings', 'wsSettings', 'path']} label={t('path')}>
        <Input />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'wsSettings', 'heartbeatPeriod']}
        label={t('pages.inbounds.form.heartbeatPeriod')}
      >
        <InputNumber min={0} />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.form.headers')}
        name={['streamSettings', 'wsSettings', 'headers']}
      >
        <HeaderMapEditor mode="v1" />
      </Form.Item>
    </>
  );
}
