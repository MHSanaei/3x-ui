import { useTranslation } from 'react-i18next';
import { Button, Form, Input, Select, Space, Switch, type FormInstance } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';

import { RandomUtil } from '@/utils';
import { SSMethodSchema } from '@/schemas/protocols/shared/shadowsocks';
import type { InboundFormValues } from '@/schemas/forms/inbound-form';

interface ShadowsocksFieldsProps {
  form: FormInstance<InboundFormValues>;
  isSSWith2022: boolean;
}

export default function ShadowsocksFields({ form, isSSWith2022 }: ShadowsocksFieldsProps) {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item name={['settings', 'method']} label={t('pages.inbounds.form.encryptionMethod')}>
        <Select
          onChange={(v) => {
            form.setFieldValue(
              ['settings', 'password'],
              RandomUtil.randomShadowsocksPassword(v as string),
            );
          }}
          options={SSMethodSchema.options.map((m) => ({ value: m, label: m }))}
        />
      </Form.Item>
      {isSSWith2022 && (
        <Form.Item label={t('password')}>
          <Space.Compact block>
            <Form.Item name={['settings', 'password']} noStyle>
              <Input style={{ width: 'calc(100% - 32px)' }} />
            </Form.Item>
            <Button
              aria-label={t('regenerate')}
              icon={<ReloadOutlined />}
              onClick={() => {
                const method = form.getFieldValue(['settings', 'method']);
                form.setFieldValue(
                  ['settings', 'password'],
                  RandomUtil.randomShadowsocksPassword(method as string),
                );
              }}
            />
          </Space.Compact>
        </Form.Item>
      )}
      <Form.Item name={['settings', 'network']} label={t('pages.inbounds.network')}>
        <Select
          style={{ width: 120 }}
          options={[
            { value: 'tcp,udp', label: 'TCP, UDP' },
            { value: 'tcp', label: 'TCP' },
            { value: 'udp', label: 'UDP' },
          ]}
        />
      </Form.Item>
      <Form.Item
        name={['settings', 'ivCheck']}
        label="ivCheck"
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
    </>
  );
}
