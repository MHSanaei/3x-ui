import { useTranslation } from 'react-i18next';
import { Alert, Button, Form, Input, Space } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';

import { generateMtprotoSecret, mtprotoSecretForDomain } from '@/lib/xray/inbound-defaults';

export default function MtprotoFields() {
  const { t } = useTranslation();
  const form = Form.useFormInstance();
  return (
    <>
      <Form.Item name={['settings', 'fakeTlsDomain']} label={t('pages.inbounds.form.fakeTlsDomain')}>
        <Input
          placeholder="www.cloudflare.com"
          onChange={(e) => {
            const current = (form.getFieldValue(['settings', 'secret']) as string) ?? '';
            form.setFieldValue(['settings', 'secret'], mtprotoSecretForDomain(current, e.target.value));
          }}
        />
      </Form.Item>
      <Form.Item label={t('pages.inbounds.form.mtprotoSecret')}>
        <Space.Compact block>
          <Form.Item name={['settings', 'secret']} noStyle>
            <Input readOnly style={{ width: 'calc(100% - 32px)' }} />
          </Form.Item>
          <Button
            icon={<ReloadOutlined />}
            onClick={() => {
              const domain = form.getFieldValue(['settings', 'fakeTlsDomain']);
              form.setFieldValue(['settings', 'secret'], generateMtprotoSecret(domain as string));
            }}
          />
        </Space.Compact>
      </Form.Item>
      <Form.Item wrapperCol={{ span: 24 }}>
        <Alert type="info" showIcon message={t('pages.inbounds.form.mtprotoHint')} />
      </Form.Item>
    </>
  );
}
