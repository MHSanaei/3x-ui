import { useTranslation } from 'react-i18next';
import { Input, Switch } from 'antd';

import { HeaderMapEditor } from '@/components/form';
import { FormField } from '@/components/form/rhf';

export default function HttpUpgradeForm() {
  const { t } = useTranslation();
  return (
    <>
      <FormField
        name={['streamSettings', 'httpupgradeSettings', 'acceptProxyProtocol']}
        label={t('pages.inbounds.form.proxyProtocol')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <FormField
        name={['streamSettings', 'httpupgradeSettings', 'host']}
        label={t('host')}
      >
        <Input />
      </FormField>
      <FormField
        name={['streamSettings', 'httpupgradeSettings', 'path']}
        label={t('path')}
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
