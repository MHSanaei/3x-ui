import { useTranslation } from 'react-i18next';
import { Form, Input } from 'antd';

import { TrojanOutboundFormSettingsSchema } from '@/schemas/forms/outbound-form';
import { antdRule } from '@/utils/zodForm';

export default function TrojanFields() {
  const { t } = useTranslation();
  return (
    <Form.Item
      label={t('password')}
      name={['settings', 'password']}
      rules={[antdRule(TrojanOutboundFormSettingsSchema.shape.password, t)]}
    >
      <Input />
    </Form.Item>
  );
}
