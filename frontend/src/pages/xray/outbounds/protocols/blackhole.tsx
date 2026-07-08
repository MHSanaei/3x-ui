import { useTranslation } from 'react-i18next';
import { Select } from 'antd';

import { FormField } from '@/components/form/rhf';

export default function BlackholeFields() {
  const { t } = useTranslation();
  return (
    <FormField label={t('pages.xray.outboundForm.responseType')} name={['settings', 'type']}>
      <Select
        options={[
          { value: '', label: '(empty)' },
          { value: 'none', label: 'none' },
          { value: 'http', label: 'http' },
        ]}
      />
    </FormField>
  );
}
