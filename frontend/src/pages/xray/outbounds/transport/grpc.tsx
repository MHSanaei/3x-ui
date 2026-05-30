import { useTranslation } from 'react-i18next';
import { Form, Input, Switch } from 'antd';

export default function GrpcForm() {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item
        label={t('pages.inbounds.form.serviceName')}
        name={['streamSettings', 'grpcSettings', 'serviceName']}
      >
        <Input />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.form.authority')}
        name={['streamSettings', 'grpcSettings', 'authority']}
      >
        <Input />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.form.multiMode')}
        name={['streamSettings', 'grpcSettings', 'multiMode']}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
    </>
  );
}
