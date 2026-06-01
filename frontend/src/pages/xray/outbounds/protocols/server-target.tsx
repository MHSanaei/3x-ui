import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber } from 'antd';

export default function ServerTarget() {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item
        label={t('pages.inbounds.address')}
        name={['settings', 'address']}
        rules={[{ required: true, message: t('pages.xray.outboundForm.addressRequired') }]}
      >
        <Input />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.port')}
        name={['settings', 'port']}
        rules={[{ required: true, message: t('pages.xray.outboundForm.portRequired') }]}
      >
        <InputNumber min={1} max={65535} style={{ width: '100%' }} />
      </Form.Item>
    </>
  );
}
