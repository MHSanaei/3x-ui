import { useTranslation } from 'react-i18next';
import { Form, Input } from 'antd';

import { HeaderMapEditor } from '@/components/form';

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
      <Form.Item label={t('pages.inbounds.form.headers')} name={['settings', 'headers']}>
        <HeaderMapEditor mode="v1" />
      </Form.Item>
    </>
  );
}
