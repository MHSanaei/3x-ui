import { useTranslation } from 'react-i18next';
import { Switch } from 'antd';

import { FormField } from '@/components/form/rhf';
import AccountsList from './accounts-list';

export default function HttpFields() {
  const { t } = useTranslation();
  return (
    <>
      <AccountsList />
      <FormField
        name={['settings', 'allowTransparent']}
        label={t('pages.inbounds.form.allowTransparent')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
    </>
  );
}
