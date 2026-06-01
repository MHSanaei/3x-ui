import { useTranslation } from 'react-i18next';
import { Form, Input, Switch } from 'antd';

import { HeaderMapEditor } from '@/components/form';

export default function HttpUpgradeForm() {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item
        name={['streamSettings', 'httpupgradeSettings', 'acceptProxyProtocol']}
        label={t('pages.inbounds.form.proxyProtocol')}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'httpupgradeSettings', 'host']}
        label={t('host')}
      >
        <Input />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'httpupgradeSettings', 'path']}
        label={t('path')}
      >
        <Input />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.form.headers')}
        name={['streamSettings', 'httpupgradeSettings', 'headers']}
      >
        <HeaderMapEditor mode="v1" />
      </Form.Item>
    </>
  );
}
