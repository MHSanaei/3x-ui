import { useTranslation } from 'react-i18next';
import { Input } from 'antd';

import { HeaderMapEditor } from '@/components/form';
import { FormField } from '@/components/form/rhf';

export default function HttpFields() {
  const { t } = useTranslation();
  return (
    <>
      <FormField label={t('username')} name={['settings', 'user']}>
        <Input />
      </FormField>
      <FormField label={t('password')} name={['settings', 'pass']}>
        <Input />
      </FormField>
      <FormField label={t('pages.inbounds.form.headers')} name={['settings', 'headers']}>
        <HeaderMapEditor mode="v1" />
      </FormField>
    </>
  );
}
