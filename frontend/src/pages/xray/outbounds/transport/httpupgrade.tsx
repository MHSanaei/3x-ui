import { useTranslation } from 'react-i18next';
import { Input } from 'antd';

import { HeaderMapEditor } from '@/components/form';
import { FormField } from '@/components/form/rhf';

export default function HttpUpgradeForm() {
  const { t } = useTranslation();
  return (
    <>
      <FormField
        label={t('host')}
        name={['streamSettings', 'httpupgradeSettings', 'host']}
      >
        <Input />
      </FormField>
      <FormField
        label={t('path')}
        name={['streamSettings', 'httpupgradeSettings', 'path']}
      >
        <Input />
      </FormField>
      <FormField
        label={t('pages.inbounds.form.headers')}
        name={['streamSettings', 'httpupgradeSettings', 'headers']}
      >
        <HeaderMapEditor mode="v1" />
      </FormField>
    </>
  );
}
