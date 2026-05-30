import { useTranslation } from 'react-i18next';
import { Form, Input } from 'antd';

import {
  VlessOutboundFormSettingsSchema,
  VmessOutboundFormSettingsSchema,
} from '@/schemas/forms/outbound-form';
import { antdRule } from '@/utils/zodForm';

export default function VlessFields() {
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
        label={t('encryption')}
        name={['settings', 'encryption']}
        rules={[antdRule(VlessOutboundFormSettingsSchema.shape.encryption, t)]}
      >
        <Input />
      </Form.Item>
      <Form.Item label={t('pages.clients.reverseTag')} name={['settings', 'reverseTag']}>
        <Input placeholder={t('pages.xray.outboundForm.optional')} />
      </Form.Item>
    </>
  );
}
