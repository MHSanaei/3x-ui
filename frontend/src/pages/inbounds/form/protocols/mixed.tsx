import { useTranslation } from 'react-i18next';
import { Form, Input, Select, Switch } from 'antd';

import AccountsList from './accounts-list';

export default function MixedFields({ mixedUdpOn }: { mixedUdpOn: boolean }) {
  const { t } = useTranslation();
  return (
    <>
      <AccountsList />
      <Form.Item name={['settings', 'auth']} label={t('pages.inbounds.info.auth')}>
        <Select
          options={[
            { value: 'noauth', label: 'noauth' },
            { value: 'password', label: 'password' },
          ]}
        />
      </Form.Item>
      <Form.Item
        name={['settings', 'udp']}
        label="UDP"
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      {mixedUdpOn && (
        <Form.Item name={['settings', 'ip']} label="UDP IP">
          <Input />
        </Form.Item>
      )}
    </>
  );
}
