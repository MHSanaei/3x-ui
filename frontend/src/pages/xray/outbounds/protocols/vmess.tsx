import { useTranslation } from 'react-i18next';
import { Form, Input, Select } from 'antd';

import { VmessOutboundFormSettingsSchema } from '@/schemas/forms/outbound-form';
import { antdRule } from '@/utils/zodForm';

import { SECURITY_OPTIONS } from '../outbound-form-constants';

export default function VmessFields() {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item
        label="ID"
        name={['settings', 'id']}
        rules={[antdRule(VmessOutboundFormSettingsSchema.shape.id, t)]}
      >
        <Input placeholder="UUID" />
      </Form.Item>
      <Form.Item
        label={t('security')}
        name={['settings', 'security']}
        rules={[antdRule(VmessOutboundFormSettingsSchema.shape.security, t)]}
      >
        <Select options={SECURITY_OPTIONS} />
      </Form.Item>
    </>
  );
}
