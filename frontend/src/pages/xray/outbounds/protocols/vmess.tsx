import { useTranslation } from 'react-i18next';
import { Input, Select } from 'antd';

import { FormField, rhfZodValidate } from '@/components/form/rhf';
import { VmessOutboundFormSettingsSchema } from '@/schemas/forms/outbound-form';

import { SECURITY_OPTIONS } from '../outbound-form-constants';

export default function VmessFields() {
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
        label={t('security')}
        name={['settings', 'security']}
        rules={{ validate: rhfZodValidate(VmessOutboundFormSettingsSchema.shape.security) }}
      >
        <Select options={SECURITY_OPTIONS} />
      </FormField>
    </>
  );
}
