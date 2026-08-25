import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, Space, Switch } from 'antd';
import { MinusOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { useFieldArray, useFormContext } from 'react-hook-form';

import { Wireguard } from '@/utils';
import { activateOnKey } from '@/utils/a11y';
import { InputAddon } from '@/components/ui';
import { FormField } from '@/components/form/rhf';

// Fields for the panel's amneziawg OUTBOUND pseudo-protocol. Reuses the
// inbound AmneziaWG i18n keys: every label names the identical protocol
// parameter (jc/s1/h1/...), and both ends of a tunnel must carry the same
// values, so one label set serves both forms.
export default function AmneziawgFields() {
  const { t } = useTranslation();
  const { control, setValue } = useFormContext();
  const {
    fields: peerFields,
    append: appendPeer,
    remove: removePeer,
  } = useFieldArray({ control, name: 'settings.peers' });

  return (
    <>
      <FormField label={t('pages.xray.amneziawg.mtu')} name={['settings', 'mtu']}>
        <InputNumber min={0} style={{ width: '100%' }} />
      </FormField>
      <FormField
        label={t('pages.xray.amneziawg.listenPort')}
        name={['settings', 'listenPort']}
        extra={t('pages.xray.amneziawg.listenPortHint')}
      >
        <InputNumber min={0} max={65535} style={{ width: '100%' }} />
      </FormField>
      <Form.Item label={t('pages.inbounds.privatekey')}>
        <FormField name={['settings', 'secretKey']} noStyle>
          <Input
            aria-label={t('pages.inbounds.privatekey')}
            style={{ width: 'calc(100% - 32px)' }}
          />
        </FormField>
        <Button
          icon={<ReloadOutlined />}
          aria-label={t('regenerate')}
          onClick={() => {
            const pair = Wireguard.generateKeypair();
            setValue('settings.secretKey', pair.privateKey);
          }}
        />
      </Form.Item>
      <Form.Item label={t('pages.inbounds.address')} required>
        <FormField name={['settings', 'address', 0]} noStyle>
          <Input placeholder="10.8.0.2/32" aria-label={t('pages.inbounds.address')} />
        </FormField>
      </Form.Item>
      <FormField
        name={['settings', 'headerProtectionKey']}
        label={t('pages.xray.amneziawg.headerProtectionKey')}
        extra={t('pages.xray.amneziawg.headerProtectionKeyHint')}
      >
        <Input />
      </FormField>

      <Form.Item label={t('pages.xray.amneziawg.obfuscation')}>
        <span className="text-xs text-gray-400">
          {t('pages.xray.amneziawg.outboundObfuscationHint')}
        </span>
      </Form.Item>
      <ObfNumber name="jc" label={t('pages.xray.amneziawg.jc')} min={0} />
      <ObfNumber name="jmin" label={t('pages.xray.amneziawg.jmin')} min={0} />
      <ObfNumber name="jmax" label={t('pages.xray.amneziawg.jmax')} min={0} />
      <ObfNumber name="s1" label={t('pages.xray.amneziawg.s1')} min={0} />
      <ObfNumber name="s2" label={t('pages.xray.amneziawg.s2')} min={0} />
      <ObfNumber name="s3" label={t('pages.xray.amneziawg.s3')} min={0} max={64} />
      <ObfNumber name="s4" label={t('pages.xray.amneziawg.s4')} min={0} max={32} />
      <ObfText name="h1" label={t('pages.xray.amneziawg.h1')} placeholder="100-800" />
      <ObfText name="h2" label={t('pages.xray.amneziawg.h2')} placeholder="900-1600" />
      <ObfText name="h3" label={t('pages.xray.amneziawg.h3')} placeholder="1700-2400" />
      <ObfText name="h4" label={t('pages.xray.amneziawg.h4')} placeholder="2500-3200" />
      <ObfText name="i1" label={t('pages.xray.amneziawg.i1')} placeholder="<r 64>" />
      <ObfText
        name="contentPaddingAddition"
        label={t('pages.xray.amneziawg.contentPaddingAddition')}
        placeholder="8-64"
      />

      <Form.Item label={t('pages.inbounds.form.peers')}>
        <Button
          size="small"
          type="primary"
          icon={<PlusOutlined />}
          aria-label={t('add')}
          onClick={() =>
            appendPeer({
              publicKey: '',
              presharedKey: '',
              allowedIPs: ['0.0.0.0/0', '::/0'],
              endpoint: '',
              keepAlive: 25,
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
                <MinusOutlined
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
          <FormField
            label={t('pages.xray.wireguard.endpoint')}
            name={['settings', 'peers', index, 'endpoint']}
          >
            <Input placeholder="203.0.113.7:51820" />
          </FormField>
          <FormField
            label={t('pages.inbounds.publicKey')}
            name={['settings', 'peers', index, 'publicKey']}
          >
            <Input />
          </FormField>
          <FormField label="PSK" name={['settings', 'peers', index, 'presharedKey']}>
            <Input />
          </FormField>
          <PeerAllowedIPs peerIndex={index} />
          <FormField
            label={t('pages.inbounds.info.keepAlive')}
            name={['settings', 'peers', index, 'keepAlive']}
          >
            <InputNumber min={0} />
          </FormField>
        </div>
      ))}

      <FormField
        name={['settings', 'randomTrailers']}
        label={t('pages.xray.amneziawg.randomTrailers')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <FormField
        name={['settings', 'disableCookies']}
        label={t('pages.xray.amneziawg.disableCookies')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
    </>
  );
}

function PeerAllowedIPs({ peerIndex }: { peerIndex: number }) {
  const { t } = useTranslation();
  const { control } = useFormContext();
  const { fields, append, remove } = useFieldArray({
    control,
    name: `settings.peers.${peerIndex}.allowedIPs`,
  });
  return (
    <Form.Item label={t('pages.xray.wireguard.allowedIPs')}>
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
    </Form.Item>
  );
}

function ObfNumber({
  name,
  label,
  min,
  max,
}: {
  name: string;
  label: string;
  min?: number;
  max?: number;
}) {
  return (
    <FormField label={label} name={['settings', name] as never}>
      <InputNumber min={min} max={max} style={{ width: '100%' }} />
    </FormField>
  );
}

function ObfText({
  name,
  label,
  placeholder,
}: {
  name: string;
  label: string;
  placeholder?: string;
}) {
  return (
    <FormField label={label} name={['settings', name] as never}>
      <Input placeholder={placeholder} />
    </FormField>
  );
}
