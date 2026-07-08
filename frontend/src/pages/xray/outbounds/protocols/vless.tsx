import { useTranslation } from 'react-i18next';
import { Input } from 'antd';

import { FormField, rhfZodValidate } from '@/components/form/rhf';
import {
  VlessOutboundFormSettingsSchema,
  VmessOutboundFormSettingsSchema,
} from '@/schemas/forms/outbound-form';

export default function VlessFields() {
  const { t } = useTranslation();
  return (
    <>
      <FormField
        label="ID"
        name={['settings', 'id']}
        rules={{ validate: rhfZodValidate(VmessOutboundFormSettingsSchema.shape.id) }}
      >
        <Input placeholder="UUID" />
      </FormField>
      <FormField
        label={t('encryption')}
        name={['settings', 'encryption']}
        rules={{ validate: rhfZodValidate(VlessOutboundFormSettingsSchema.shape.encryption) }}
      >
        <Input />
      </FormField>
      <FormField label={t('pages.clients.reverseTag')} name={['settings', 'reverseTag']}>
        <Input placeholder={t('pages.xray.outboundForm.optional')} />
      </FormField>
    </>
  );
}
