import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, Select, Space, Switch } from 'antd';
import { DeleteOutlined, MinusOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { useFieldArray, useFormContext } from 'react-hook-form';

import { Wireguard } from '@/utils';
import { activateOnKey } from '@/utils/a11y';
import { InputAddon } from '@/components/ui';
import { FormField } from '@/components/form/rhf';
import { WireguardDomainStrategy } from '@/schemas/primitives';

function AllowedIPsList({ peerIndex }: { peerIndex: number }) {
  const { t } = useTranslation();
  const { control } = useFormContext();
  const { fields, append, remove } = useFieldArray({
    control,
    name: `settings.peers.${peerIndex}.allowedIPs`,
  });
  return (
    <>
      {fields.map((field, ipIdx) => (
        <Space.Compact key={field.id} block style={{ marginBottom: 4 }}>
          <FormField noStyle name={['settings', 'peers', peerIndex, 'allowedIPs', ipIdx]}>
            <Input aria-label={t('pages.xray.wireguard.allowedIPs')} />
          </FormField>
          {fields.length > 1 && (
            <InputAddon ariaLabel={t('remove')} onClick={() => remove(ipIdx)}>
              <MinusOutlined />
            </InputAddon>
          )}
        </Space.Compact>
      ))}
      <Button
        size="small"
        icon={<PlusOutlined />}
        aria-label={t('add')}
        onClick={() => append('')}
      />
    </>
  );
}

export default function WireguardFields() {
  const { t } = useTranslation();
  const { control, setValue } = useFormContext();
  const {
    fields: peerFields,
    append: appendPeer,
    remove: removePeer,
  } = useFieldArray({ control, name: 'settings.peers' });
  return (
    <>
      <FormField label={t('pages.inbounds.address')} name={['settings', 'address']}>
        <Input placeholder="comma-separated, e.g. 10.0.0.1,fd00::1" />
      </FormField>
      <Form.Item label={t('pages.inbounds.privatekey')}>
        <Space.Compact block>
          <FormField name={['settings', 'secretKey']} noStyle>
            <Input aria-label={t('pages.inbounds.privatekey')} style={{ width: 'calc(100% - 32px)' }} />
          </FormField>
          <Button
            icon={<ReloadOutlined />}
            aria-label={t('regenerate')}
            onClick={() => {
              const pair = Wireguard.generateKeypair();
              setValue('settings.secretKey', pair.privateKey);
              setValue('settings.pubKey', pair.publicKey);
            }}
          />
        </Space.Compact>
      </Form.Item>
      <FormField label={t('pages.inbounds.publicKey')} name={['settings', 'pubKey']}>
        <Input disabled />
      </FormField>
      <FormField label={t('pages.xray.wireguard.domainStrategy')} name={['settings', 'domainStrategy']}>
        <Select
          options={[
            { value: '', label: `(${t('none')})` },
            ...WireguardDomainStrategy.map((s) => ({ value: s, label: s })),
          ]}
        />
      </FormField>
      <FormField label="MTU" name={['settings', 'mtu']}>
        <InputNumber min={0} />
      </FormField>
      <FormField
        label={t('pages.inbounds.info.noKernelTun')}
        name={['settings', 'noKernelTun']}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <FormField label={t('pages.xray.outboundForm.reserved')} name={['settings', 'reserved']}>
        <Input placeholder="comma-separated bytes, e.g. 1,2,3" />
      </FormField>
      <Form.Item label={t('pages.inbounds.form.peers')}>
        <Button
          size="small"
          type="primary"
          icon={<PlusOutlined />}
          aria-label={t('add')}
          onClick={() =>
            appendPeer({
              publicKey: '',
              psk: '',
              allowedIPs: ['0.0.0.0/0', '::/0'],
              endpoint: '',
              keepAlive: 0,
            })
          }
        />
      </Form.Item>
      {peerFields.map((field, index) => (
        <div key={field.id}>
          <Form.Item wrapperCol={{ md: { span: 14, offset: 8 } }}>
            <div className="item-heading">
              <span>{t('pages.inbounds.info.peerNumber', { n: index + 1 })}</span>
              {peerFields.length > 1 && (
                <DeleteOutlined
                  className="danger-icon"
                  role="button"
                  tabIndex={0}
                  aria-label={t('remove')}
                  onClick={() => removePeer(index)}
                  onKeyDown={activateOnKey(() => removePeer(index))}
                />
              )}
            </div>
          </Form.Item>
          <FormField label={t('pages.xray.wireguard.endpoint')} name={['settings', 'peers', index, 'endpoint']}>
            <Input />
          </FormField>
          <FormField label={t('pages.inbounds.publicKey')} name={['settings', 'peers', index, 'publicKey']}>
            <Input />
          </FormField>
          <FormField label="PSK" name={['settings', 'peers', index, 'psk']}>
            <Input />
          </FormField>
          <Form.Item label={t('pages.xray.wireguard.allowedIPs')}>
            <AllowedIPsList peerIndex={index} />
          </Form.Item>
          <FormField label={t('pages.inbounds.info.keepAlive')} name={['settings', 'peers', index, 'keepAlive']}>
            <InputNumber min={0} />
          </FormField>
        </div>
      ))}
    </>
  );
}
