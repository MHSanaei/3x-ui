import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber, Select, Switch } from 'antd';

import {
  ShadowsocksOutboundFormSettingsSchema,
  TrojanOutboundFormSettingsSchema,
  VlessOutboundFormSettingsSchema,
  VmessOutboundFormSettingsSchema,
} from '@/schemas/forms/outbound-form';
import { SSMethodSchema } from '@/schemas/protocols/shared/shadowsocks';
import { antdRule } from '@/utils/zodForm';

import { SECURITY_OPTIONS, SS_METHOD_OPTIONS } from './outbound-form-constants';

export function OutboundCoreProtocolFields({ protocol }: { protocol: string }) {
  const { t } = useTranslation();
  return (
    <>
      {(protocol === 'vmess' || protocol === 'vless') && (
        <Form.Item
          label="ID"
          name={['settings', 'id']}
          rules={[antdRule(VmessOutboundFormSettingsSchema.shape.id, t)]}
        >
          <Input placeholder="UUID" />
        </Form.Item>
      )}
      {protocol === 'vmess' && (
        <Form.Item
          label={t('security')}
          name={['settings', 'security']}
          rules={[antdRule(VmessOutboundFormSettingsSchema.shape.security, t)]}
        >
          <Select options={SECURITY_OPTIONS} />
        </Form.Item>
      )}
      {protocol === 'vless' && (
        <>
          <Form.Item
            label={t('encryption')}
            name={['settings', 'encryption']}
            rules={[antdRule(VlessOutboundFormSettingsSchema.shape.encryption, t)]}
          >
            <Input />
          </Form.Item>
          <Form.Item label={t('pages.clients.reverseTag')} name={['settings', 'reverseTag']}>
            <Input placeholder={t('pages.xray.outboundForm.optional')} />
          </Form.Item>
        </>
      )}

      {(protocol === 'trojan' || protocol === 'shadowsocks') && (
        <Form.Item
          label={t('password')}
          name={['settings', 'password']}
          rules={[
            antdRule(
              protocol === 'trojan'
                ? TrojanOutboundFormSettingsSchema.shape.password
                : ShadowsocksOutboundFormSettingsSchema.shape.password,
              t,
            ),
          ]}
        >
          <Input />
        </Form.Item>
      )}

      {protocol === 'shadowsocks' && (
        <>
          <Form.Item
            label={t('encryption')}
            name={['settings', 'method']}
            rules={[antdRule(SSMethodSchema, t)]}
          >
            <Select options={SS_METHOD_OPTIONS} />
          </Form.Item>
          <Form.Item
            label={t('pages.xray.outboundForm.udpOverTcp')}
            name={['settings', 'uot']}
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item label={t('pages.xray.outboundForm.uotVersion')} name={['settings', 'UoTVersion']}>
            <InputNumber min={1} max={2} />
          </Form.Item>
        </>
      )}

      {(protocol === 'socks' || protocol === 'http') && (
        <>
          <Form.Item label={t('username')} name={['settings', 'user']}>
            <Input />
          </Form.Item>
          <Form.Item label={t('password')} name={['settings', 'pass']}>
            <Input />
          </Form.Item>
        </>
      )}
    </>
  );
}
