import { useTranslation } from 'react-i18next';
import { Form, Input } from 'antd';

export default function HttpFields() {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item label={t('username')} name={['settings', 'user']}>
        <Input />
      </Form.Item>
      <Form.Item label={t('password')} name={['settings', 'pass']}>
        <Input />
      </Form.Item>
    </>
  );
}
