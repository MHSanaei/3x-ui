import { useTranslation } from 'react-i18next';
import { Form, Input, Switch } from 'antd';

export default function GrpcForm() {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item
        name={['streamSettings', 'grpcSettings', 'serviceName']}
        label={t('pages.inbounds.form.serviceName')}
      >
        <Input />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'grpcSettings', 'authority']}
        label={t('pages.inbounds.form.authority')}
      >
        <Input />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'grpcSettings', 'multiMode']}
        label={t('pages.inbounds.form.multiMode')}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
    </>
  );
}
