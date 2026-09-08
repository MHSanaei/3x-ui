import { useTranslation } from 'react-i18next';
import { Input, Select } from 'antd';
import { useFormContext, useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';

export default function BlackholeFields() {
  const { t } = useTranslation();
  const { control } = useFormContext();
  const type = useWatch({ control, name: 'settings.type' }) as string | undefined;
  return (
    <>
      <FormField label={t('pages.xray.outboundForm.responseType')} name={['settings', 'type']}>
        <Select
          options={[
            { value: '', label: '(empty)' },
            { value: 'none', label: 'none' },
            { value: 'http', label: 'http' },
            { value: 'custom', label: 'custom' },
          ]}
        />
      </FormField>
      {type === 'custom' && (
        <FormField
          label={t('pages.xray.outboundForm.customResponseData')}
          name={['settings', 'customResponseData']}
        >
          <Input.TextArea rows={3} placeholder="SFRUUC8xLjEgNDAzIEZvcmJpZGRlbg0KDQo=" />
        </FormField>
      )}
    </>
  );
}
