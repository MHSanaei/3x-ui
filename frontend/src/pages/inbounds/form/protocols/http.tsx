import { useTranslation } from 'react-i18next';
import { Form, Switch } from 'antd';

import AccountsList from './accounts-list';

export default function HttpFields() {
  const { t } = useTranslation();
  return (
    <>
      <AccountsList />
      <Form.Item
        name={['settings', 'allowTransparent']}
        label={t('pages.inbounds.form.allowTransparent')}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
    </>
  );
}
