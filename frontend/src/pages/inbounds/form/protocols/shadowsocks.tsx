import { useId } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Form, Input, Select, Space, Switch } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useFormContext } from 'react-hook-form';

import { RandomUtil } from '@/utils';
import { FormField } from '@/components/form/rhf';
import { SSInboundMethodSchema } from '@/schemas/protocols/shared/shadowsocks';

interface ShadowsocksFieldsProps {
  isSSWith2022: boolean;
}

export default function ShadowsocksFields({ isSSWith2022 }: ShadowsocksFieldsProps) {
  const { t } = useTranslation();
  const uid = useId();
  const { getValues, setValue } = useFormContext();
  return (
    <>
      <FormField
        name={['settings', 'method']}
        label={t('pages.inbounds.form.encryptionMethod')}
        onAfterChange={(v) => {
          setValue('settings.password', RandomUtil.randomShadowsocksPassword(v as string));
        }}
      >
        <Select options={SSInboundMethodSchema.options.map((m) => ({ value: m, label: m }))} />
      </FormField>
      {isSSWith2022 && (
        <Form.Item htmlFor={`${uid}-password`} label={t('password')}>
          <Space.Compact block>
            <FormField name={['settings', 'password']} noStyle>
              <Input id={`${uid}-password`} style={{ width: 'calc(100% - 32px)' }} />
            </FormField>
            <Button
              aria-label={t('regenerate')}
              icon={<ReloadOutlined aria-hidden />}
              onClick={() => {
                const method = getValues('settings.method');
                setValue(
                  'settings.password',
                  RandomUtil.randomShadowsocksPassword(method as string),
                );
              }}
            />
          </Space.Compact>
        </Form.Item>
      )}
      <FormField name={['settings', 'network']} label={t('pages.inbounds.network')}>
        <Select
          style={{ width: 120 }}
          options={[
            { value: 'tcp,udp', label: 'TCP, UDP' },
            { value: 'tcp', label: 'TCP' },
            { value: 'udp', label: 'UDP' },
          ]}
        />
      </FormField>
      <FormField name={['settings', 'ivCheck']} label="ivCheck" valueProp="checked">
        <Switch />
      </FormField>
    </>
  );
}
