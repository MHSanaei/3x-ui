import { useTranslation } from 'react-i18next';
import { Button, Divider, Form, Input, InputNumber, Space, Switch } from 'antd';
import { MinusOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';

import { Wireguard } from '@/utils';

interface WireguardFieldsProps {
  wgPubKey: string;
  regenInboundWg: () => void;
  regenWgPeerKeypair: (name: number) => void;
}

export default function WireguardFields({ wgPubKey, regenInboundWg, regenWgPeerKeypair }: WireguardFieldsProps) {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item label={t('pages.xray.wireguard.secretKey')}>
        <Space.Compact block>
          <Form.Item name={['settings', 'secretKey']} noStyle>
            <Input style={{ width: 'calc(100% - 32px)' }} />
          </Form.Item>
          <Button icon={<ReloadOutlined />} onClick={regenInboundWg} />
        </Space.Compact>
      </Form.Item>
      <Form.Item label={t('pages.xray.wireguard.publicKey')}>
        <Input value={wgPubKey} disabled />
      </Form.Item>
      <Form.Item name={['settings', 'mtu']} label="MTU">
        <InputNumber />
      </Form.Item>
      <Form.Item
        name={['settings', 'noKernelTun']}
        label={t('pages.inbounds.info.noKernelTun')}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      <Form.List name={['settings', 'peers']}>
        {(fields, { add, remove }) => (
          <>
            <Form.Item label={t('pages.inbounds.form.peers')}>
              <Button
                size="small"
                onClick={() => {
                  const kp = Wireguard.generateKeypair();
                  add({
                    privateKey: kp.privateKey,
                    publicKey: kp.publicKey,
                    allowedIPs: ['10.0.0.2/32'],
                    keepAlive: 0,
                  });
                }}
              >
                <PlusOutlined /> {t('pages.inbounds.form.addPeer')}
              </Button>
            </Form.Item>
            {fields.map((field, idx) => (
              <div key={field.key} className="wg-peer">
                <Divider titlePlacement="center">
                  <Space>
                    <span>{t('pages.inbounds.info.peerNumber', { n: idx + 1 })}</span>
                    {fields.length > 1 && (
                      <Button
                        size="small"
                        danger
                        icon={<MinusOutlined />}
                        onClick={() => remove(field.name)}
                      />
                    )}
                  </Space>
                </Divider>
                <Form.Item label={t('pages.xray.wireguard.secretKey')}>
                  <Space.Compact block>
                    <Form.Item name={[field.name, 'privateKey']} noStyle>
                      <Input style={{ width: 'calc(100% - 32px)' }} />
                    </Form.Item>
                    <Button
                      icon={<ReloadOutlined />}
                      onClick={() => regenWgPeerKeypair(field.name)}
                    />
                  </Space.Compact>
                </Form.Item>
                <Form.Item name={[field.name, 'publicKey']} label={t('pages.xray.wireguard.publicKey')}>
                  <Input />
                </Form.Item>
                <Form.Item name={[field.name, 'preSharedKey']} label="PSK">
                  <Input />
                </Form.Item>
                <Form.List name={[field.name, 'allowedIPs']}>
                  {(ipFields, { add: addIp, remove: removeIp }) => (
                    <Form.Item label={t('pages.xray.wireguard.allowedIPs')}>
                      <Button size="small" onClick={() => addIp('')}>
                        <PlusOutlined />
                      </Button>
                      {ipFields.map((ipField) => (
                        <Space.Compact key={ipField.key} block className="mt-4">
                          <Form.Item name={ipField.name} noStyle>
                            <Input />
                          </Form.Item>
                          {ipFields.length > 1 && (
                            <Button size="small" onClick={() => removeIp(ipField.name)}>
                              <MinusOutlined />
                            </Button>
                          )}
                        </Space.Compact>
                      ))}
                    </Form.Item>
                  )}
                </Form.List>
                <Form.Item name={[field.name, 'keepAlive']} label={t('pages.inbounds.form.keepAlive')}>
                  <InputNumber min={0} />
                </Form.Item>
              </div>
            ))}
          </>
        )}
      </Form.List>
    </>
  );
}
