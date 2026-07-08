import { useTranslation } from 'react-i18next';
import { Input } from 'antd';

import { FormField, rhfZodValidate } from '@/components/form/rhf';
import { TrojanOutboundFormSettingsSchema } from '@/schemas/forms/outbound-form';

export default function TrojanFields() {
  const { t } = useTranslation();
  return (
    <FormField
      label={t('password')}
      name={['settings', 'password']}
      rules={{ validate: rhfZodValidate(TrojanOutboundFormSettingsSchema.shape.password) }}
    >
      <Input />
    </FormField>
  );
}
