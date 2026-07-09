import { useTranslation } from 'react-i18next';
import { Controller, useFormContext } from 'react-hook-form';

import { SniffingField } from '@/lib/xray/forms/fields';

export default function SniffingTab() {
  const { t } = useTranslation();
  const { control } = useFormContext();
  return (
    <Controller
      control={control}
      name="sniffing"
      render={({ field }) => (
        <SniffingField
          value={field.value}
          onChange={field.onChange}
          enableLabel={t('enable')}
        />
      )}
    />
  );
}
