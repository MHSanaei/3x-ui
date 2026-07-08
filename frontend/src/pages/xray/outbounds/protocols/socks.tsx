import { useTranslation } from 'react-i18next';
import { Input } from 'antd';

import { FormField } from '@/components/form/rhf';

export default function SocksFields() {
  const { t } = useTranslation();
  return (
    <>
      <FormField label={t('username')} name={['settings', 'user']}>
        <Input />
      </FormField>
      <FormField label={t('password')} name={['settings', 'pass']}>
        <Input />
      </FormField>
    </>
  );
}
