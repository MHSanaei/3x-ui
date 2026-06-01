import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber, Select, Switch } from 'antd';

import { HeaderMapEditor } from '@/components/form';

export default function TunnelFields() {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item name={['settings', 'rewriteAddress']} label={t('pages.inbounds.form.rewriteAddress')}>
        <Input />
      </Form.Item>
      <Form.Item name={['settings', 'rewritePort']} label={t('pages.inbounds.form.rewritePort')}>
        <InputNumber min={0} max={65535} />
      </Form.Item>
      <Form.Item name={['settings', 'allowedNetwork']} label={t('pages.inbounds.form.allowedNetwork')}>
        <Select
          options={[
            { value: 'tcp,udp', label: 'TCP, UDP' },
            { value: 'tcp', label: 'TCP' },
            { value: 'udp', label: 'UDP' },
          ]}
        />
      </Form.Item>
      <Form.Item label={t('pages.inbounds.portMap')} name={['settings', 'portMap']}>
        <HeaderMapEditor mode="v1" />
      </Form.Item>
      <Form.Item
        name={['settings', 'followRedirect']}
        label={t('pages.inbounds.form.followRedirect')}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
    </>
  );
}
