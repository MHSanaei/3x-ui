import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, Select, Space, Switch, type FormInstance } from 'antd';
import { DeleteOutlined, MinusOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';

import { Wireguard } from '@/utils';
import { InputAddon } from '@/components/ui';
import { WireguardDomainStrategy } from '@/schemas/primitives';
import type { OutboundFormValues } from '@/schemas/forms/outbound-form';

export default function WireguardFields({ form }: { form: FormInstance<OutboundFormValues> }) {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item label={t('pages.inbounds.address')} name={['settings', 'address']}>
        <Input placeholder="comma-separated, e.g. 10.0.0.1,fd00::1" />
      </Form.Item>
      <Form.Item label={t('pages.inbounds.privatekey')}>
        <Space.Compact block>
          <Form.Item name={['settings', 'secretKey']} noStyle>
            <Input style={{ width: 'calc(100% - 32px)' }} />
          </Form.Item>
          <Button
            icon={<ReloadOutlined />}
            onClick={() => {
              const pair = Wireguard.generateKeypair();
              form.setFieldValue(['settings', 'secretKey'], pair.privateKey);
              form.setFieldValue(['settings', 'pubKey'], pair.publicKey);
            }}
          />
        </Space.Compact>
      </Form.Item>
      <Form.Item label={t('pages.inbounds.publicKey')} name={['settings', 'pubKey']}>
        <Input disabled />
      </Form.Item>
      <Form.Item label={t('pages.xray.wireguard.domainStrategy')} name={['settings', 'domainStrategy']}>
        <Select
          options={[
            { value: '', label: `(${t('none')})` },
            ...WireguardDomainStrategy.map((s) => ({ value: s, label: s })),
          ]}
        />
      </Form.Item>
      <Form.Item label="MTU" name={['settings', 'mtu']}>
        <InputNumber min={0} />
      </Form.Item>
      <Form.Item label={t('pages.xray.outboundForm.workers')} name={['settings', 'workers']}>
        <InputNumber min={0} />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.info.noKernelTun')}
        name={['settings', 'noKernelTun']}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      <Form.Item label={t('pages.xray.outboundForm.reserved')} name={['settings', 'reserved']}>
        <Input placeholder="comma-separated bytes, e.g. 1,2,3" />
      </Form.Item>
      <Form.List name={['settings', 'peers']}>
        {(fields, { add, remove }) => (
          <>
            <Form.Item label={t('pages.inbounds.form.peers')}>
              <Button
                size="small"
                type="primary"
                icon={<PlusOutlined />}
                onClick={() =>
                  add({
                    publicKey: '',
                    psk: '',
                    allowedIPs: ['0.0.0.0/0', '::/0'],
                    endpoint: '',
                    keepAlive: 0,
                  })
                }
              />
            </Form.Item>
            {fields.map((field, index) => (
              <div key={field.key}>
                <Form.Item wrapperCol={{ md: { span: 14, offset: 8 } }}>
                  <div className="item-heading">
                    <span>{t('pages.inbounds.info.peerNumber', { n: index + 1 })}</span>
                    {fields.length > 1 && (
                      <DeleteOutlined
                        className="danger-icon"
                        onClick={() => remove(field.name)}
                      />
                    )}
                  </div>
                </Form.Item>
                <Form.Item label={t('pages.xray.wireguard.endpoint')} name={[field.name, 'endpoint']}>
                  <Input />
                </Form.Item>
                <Form.Item
                  label={t('pages.inbounds.publicKey')}
                  name={[field.name, 'publicKey']}
                >
                  <Input />
                </Form.Item>
                <Form.Item label="PSK" name={[field.name, 'psk']}>
                  <Input />
                </Form.Item>
                <Form.Item label={t('pages.xray.wireguard.allowedIPs')}>
                  <Form.List name={[field.name, 'allowedIPs']}>
                    {(ipFields, { add: addIp, remove: removeIp }) => (
                      <>
                        {ipFields.map((ipField, ipIdx) => (
                          <Space.Compact
                            key={ipField.key}
                            block
                            style={{ marginBottom: 4 }}
                          >
                            <Form.Item noStyle name={ipField.name}>
                              <Input />
                            </Form.Item>
                            {ipFields.length > 1 && (
                              <InputAddon onClick={() => removeIp(ipIdx)}>
                                <MinusOutlined />
                              </InputAddon>
                            )}
                          </Space.Compact>
                        ))}
                        <Button
                          size="small"
                          icon={<PlusOutlined />}
                          onClick={() => addIp('')}
                        />
                      </>
                    )}
                  </Form.List>
                </Form.Item>
                <Form.Item label={t('pages.inbounds.info.keepAlive')} name={[field.name, 'keepAlive']}>
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
