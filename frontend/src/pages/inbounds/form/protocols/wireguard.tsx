import { useId } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, Space, Switch } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';

import { useFormContext, useWatch } from 'react-hook-form';
import { VkTurnCallLinkSchema, VkTurnSettingsSchema } from '@/schemas/protocols/inbound/wireguard';
import { FormField, rhfZodValidate } from '@/components/form/rhf';

interface WireguardFieldsProps {
  wgPubKey: string;
  regenInboundWg: () => void;
}

export default function WireguardFields({ wgPubKey, regenInboundWg }: WireguardFieldsProps) {
  const { t } = useTranslation();
  const uid = useId();
  const { getValues, setValue } = useFormContext();
  const vkEnabled = useWatch({ name: 'settings.vkTurn.enabled' });
  return (
    <>
      <Form.Item htmlFor={`${uid}-secretKey`} label={t('pages.xray.wireguard.secretKey')}>
        <Space.Compact block>
          <FormField name={['settings', 'secretKey']} noStyle>
            <Input id={`${uid}-secretKey`} style={{ width: 'calc(100% - 32px)' }} />
          </FormField>
          <Button
            aria-label={t('regenerate')}
            icon={<ReloadOutlined aria-hidden />}
            onClick={regenInboundWg}
          />
        </Space.Compact>
      </Form.Item>
      <Form.Item htmlFor={`${uid}-publicKey`} label={t('pages.xray.wireguard.publicKey')}>
        <Input id={`${uid}-publicKey`} value={wgPubKey} disabled />
      </Form.Item>
      <FormField name={['settings', 'subnetIp']} label={t('pages.xray.wireguard.subnetIp')}>
        <Input placeholder="10.0.0.0" />
      </FormField>
      <FormField name={['settings', 'subnetCidr']} label={t('pages.xray.wireguard.subnetCidr')}>
        <InputNumber min={1} max={32} style={{ width: '100%' }} />
      </FormField>
      <FormField
        name={['settings', 'vkTurn', 'enabled']}
        label={t('pages.xray.wireguard.vkTurnEnabled')}
        tooltip={t('pages.xray.wireguard.vkTurnHint')}
        valueProp="checked"
        onAfterChange={(enabled) => {
          if (enabled && getValues('settings.vkTurn.port') === undefined) {
            setValue('settings.vkTurn.port', VkTurnSettingsSchema.shape.port.parse(undefined));
          }
        }}
      >
        <Switch />
      </FormField>
      {vkEnabled && (
        <>
          <FormField
            name={['settings', 'vkTurn', 'callLink']}
            label={t('pages.xray.wireguard.vkTurnCallLink')}
            tooltip={t('pages.xray.wireguard.vkTurnCallLinkHint')}
            rules={{ validate: rhfZodValidate(VkTurnCallLinkSchema) }}
          >
            <Input placeholder="https://vk.ru/call/join/..." />
          </FormField>
          <FormField
            name={['settings', 'vkTurn', 'port']}
            label={t('pages.xray.wireguard.vkTurnPort')}
            rules={{ validate: rhfZodValidate(VkTurnSettingsSchema.shape.port) }}
          >
            <InputNumber min={1} max={65535} placeholder="56000" style={{ width: '100%' }} />
          </FormField>
        </>
      )}
      <FormField name={['settings', 'mtu']} label="MTU">
        <InputNumber />
      </FormField>
      <FormField name={['settings', 'dns']} label={t('pages.inbounds.info.dns')}>
        <Input placeholder="1.1.1.1, 1.0.0.1" />
      </FormField>
      <FormField
        name={['settings', 'noKernelTun']}
        label={t('pages.inbounds.info.noKernelTun')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
    </>
  );
}
