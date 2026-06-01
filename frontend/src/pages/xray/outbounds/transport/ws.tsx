import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber } from 'antd';

import { HeaderMapEditor } from '@/components/form';

export default function WsForm() {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item label={t('host')} name={['streamSettings', 'wsSettings', 'host']}>
        <Input />
      </Form.Item>
      <Form.Item label={t('path')} name={['streamSettings', 'wsSettings', 'path']}>
        <Input />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.form.heartbeatPeriod')}
        name={['streamSettings', 'wsSettings', 'heartbeatPeriod']}
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
