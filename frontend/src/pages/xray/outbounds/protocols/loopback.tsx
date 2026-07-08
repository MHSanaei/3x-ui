import { useTranslation } from 'react-i18next';
import { Input } from 'antd';
import { Controller, useFormContext } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { SniffingField } from '@/lib/xray/forms/fields';

export default function LoopbackFields() {
  const { t } = useTranslation();
  const { control } = useFormContext();

  return (
    <>
      <FormField label={t('pages.xray.outboundForm.inboundTag')} name={['settings', 'inboundTag']}>
        <Input placeholder={t('pages.xray.outboundForm.inboundTagPlaceholder')} />
      </FormField>

      <Controller
        control={control}
        name="settings.sniffing"
        render={({ field }) => (
          <SniffingField
            value={field.value}
            onChange={field.onChange}
            enableLabel={t('pages.inbounds.sniffingTab')}
          />
        )}
      />
    </>
  );
}
