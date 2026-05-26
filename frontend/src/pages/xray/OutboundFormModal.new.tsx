import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber, Modal, Select, Space, Switch, Tabs, message } from 'antd';

import JsonEditor from '@/components/JsonEditor';
import {
  formValuesToWirePayload,
  rawOutboundToFormValues,
} from '@/lib/xray/outbound-form-adapter';
import {
  OutboundFormBaseSchema,
  ShadowsocksOutboundFormSettingsSchema,
  TrojanOutboundFormSettingsSchema,
  VlessOutboundFormSettingsSchema,
  VmessOutboundFormSettingsSchema,
  type OutboundFormValues,
} from '@/schemas/forms/outbound-form';
import {
  OutboundProtocols as Protocols,
  TLS_FLOW_CONTROL,
  USERS_SECURITY,
} from '@/schemas/primitives';
import { SSMethodSchema } from '@/schemas/protocols/inbound/shadowsocks';
import { antdRule } from '@/utils/zodForm';
import './OutboundFormModal.css';

// Pattern A rewrite of OutboundFormModal. Built as a sibling `.new.tsx`
// file so the build stays green section-by-section. The atomic swap at
// the end of the rewrite replaces the legacy file in one commit
// (per Core Decision 7 in the migration spec).

interface OutboundFormModalProps {
  open: boolean;
  outbound: Record<string, unknown> | null;
  existingTags: string[];
  onClose: () => void;
  onConfirm: (outbound: Record<string, unknown>) => void;
}

const PROTOCOL_OPTIONS = Object.values(Protocols).map((p) => ({ value: p, label: p }));
const SECURITY_OPTIONS = Object.values(USERS_SECURITY).map((v) => ({ value: v, label: v }));
const FLOW_OPTIONS = Object.values(TLS_FLOW_CONTROL).map((v) => ({ value: v, label: v }));
const SS_METHOD_OPTIONS = SSMethodSchema.options.map((v) => ({ value: v, label: v }));

// Protocols whose form schema carries a flat connect target — these all
// get the shared "server" sub-block (address + port) at the top of the
// protocol section. Wireguard has an address but no port. DNS/freedom/
// blackhole/loopback have no connect target.
const SERVER_PROTOCOLS = new Set<string>([
  'vmess', 'vless', 'trojan', 'shadowsocks', 'socks', 'http', 'hysteria',
]);

function buildAddModeValues(): OutboundFormValues {
  return rawOutboundToFormValues({});
}

export default function OutboundFormModalNew({
  open,
  outbound: outboundProp,
  existingTags,
  onClose,
  onConfirm,
}: OutboundFormModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [form] = Form.useForm<OutboundFormValues>();
  const [activeKey, setActiveKey] = useState('1');
  const [jsonText, setJsonText] = useState('');
  const [jsonDirty, setJsonDirty] = useState(false);

  const isEdit = outboundProp != null;
  const title = isEdit
    ? `${t('edit')} ${t('pages.xray.Outbounds')}`
    : `+ ${t('pages.xray.Outbounds')}`;
  const okText = isEdit ? t('pages.clients.submitEdit') : t('create');

  useEffect(() => {
    if (!open) return;
    const initial = outboundProp
      ? rawOutboundToFormValues(outboundProp)
      : buildAddModeValues();
    form.resetFields();
    form.setFieldsValue(initial);
    setActiveKey('1');
    setJsonText(JSON.stringify(formValuesToWirePayload(initial), null, 2));
    setJsonDirty(false);
  }, [open, outboundProp, form]);

  const tag = Form.useWatch('tag', form) ?? '';
  const protocol = (Form.useWatch('protocol', form) ?? 'vless') as string;

  // Switching protocol resets the settings sub-object to fresh defaults
  // so leftover fields from the previous protocol do not bleed through.
  // The adapter's rawOutboundToFormValues seeds whatever the new protocol
  // expects (vless flat shape, vmess flat shape, wireguard with secretKey
  // placeholder, etc.).
  function onValuesChange(changed: Partial<OutboundFormValues>) {
    if ('protocol' in changed && changed.protocol) {
      const next = rawOutboundToFormValues({ protocol: changed.protocol });
      form.setFieldValue('settings', next.settings);
    }
  }

  const duplicateTag = useMemo(() => {
    const myTag = tag.trim();
    if (!myTag) return false;
    if (isEdit && (outboundProp?.tag as string | undefined) === myTag) return false;
    return (existingTags || []).includes(myTag);
  }, [tag, existingTags, isEdit, outboundProp]);

  // Bridge form ↔ JSON tab: when leaving the JSON tab back to Basic, push
  // any edits into form state. When entering JSON tab, snapshot current
  // form values so the user sees the live shape.
  function applyJsonToForm(): boolean {
    if (!jsonDirty) return true;
    const raw = jsonText.trim();
    if (!raw) return true;
    let parsed: Record<string, unknown>;
    try {
      parsed = JSON.parse(raw) as Record<string, unknown>;
    } catch (e) {
      messageApi.error(`JSON: ${(e as Error).message}`);
      return false;
    }
    const next = rawOutboundToFormValues(parsed);
    form.resetFields();
    form.setFieldsValue(next);
    setJsonDirty(false);
    return true;
  }

  function onTabChange(key: string) {
    if (document.activeElement instanceof HTMLElement) {
      document.activeElement.blur();
    }
    if (key === '2') {
      const values = form.getFieldsValue(true) as OutboundFormValues;
      setJsonText(JSON.stringify(formValuesToWirePayload(values), null, 2));
      setJsonDirty(false);
      setActiveKey(key);
      return;
    }
    if (key === '1' && activeKey === '2') {
      if (!applyJsonToForm()) return;
    }
    setActiveKey(key);
  }

  async function onOk() {
    if (activeKey === '2' && !applyJsonToForm()) return;
    let values: OutboundFormValues;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    if (duplicateTag) {
      messageApi.error('Tag already used by another outbound');
      return;
    }
    onConfirm(formValuesToWirePayload(values));
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={title}
        okText={okText}
        cancelText={t('close')}
        mask={{ closable: false }}
        width={780}
        onOk={onOk}
        onCancel={onClose}
        destroyOnHidden
      >
        <Form
          form={form}
          colon={false}
          labelCol={{ md: { span: 8 } }}
          wrapperCol={{ md: { span: 14 } }}
          onValuesChange={onValuesChange}
        >
          <Tabs
            activeKey={activeKey}
            onChange={onTabChange}
            items={[
              {
                key: '1',
                label: t('pages.xray.basicTemplate'),
                children: (
                  <>
                    <Form.Item
                      label={t('protocol')}
                      name="protocol"
                      rules={[antdRule(OutboundFormBaseSchema.shape.tag, t)]}
                    >
                      <Select options={PROTOCOL_OPTIONS} />
                    </Form.Item>

                    <Form.Item
                      label="Tag"
                      name="tag"
                      validateStatus={duplicateTag ? 'warning' : undefined}
                      help={duplicateTag ? 'Tag already used by another outbound' : undefined}
                      rules={[
                        { required: true, message: 'Tag is required' },
                      ]}
                    >
                      <Input placeholder="unique-tag" />
                    </Form.Item>

                    <Form.Item label="Send through" name="sendThrough">
                      <Input placeholder="local IP" />
                    </Form.Item>

                    {/* Shared connect target (address + port) for protocols
                        whose form schema carries them flat at settings root.
                        Hidden for freedom/blackhole/dns/loopback/wireguard. */}
                    {SERVER_PROTOCOLS.has(protocol) && (
                      <>
                        <Form.Item
                          label={t('pages.inbounds.address')}
                          name={['settings', 'address']}
                          rules={[{ required: true, message: 'Address is required' }]}
                        >
                          <Input />
                        </Form.Item>
                        <Form.Item
                          label={t('pages.inbounds.port')}
                          name={['settings', 'port']}
                          rules={[{ required: true, message: 'Port is required' }]}
                        >
                          <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                        </Form.Item>
                      </>
                    )}

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
                        <Form.Item label="Flow" name={['settings', 'flow']}>
                          <Select
                            allowClear
                            placeholder={t('none')}
                            options={FLOW_OPTIONS}
                          />
                        </Form.Item>
                        <Form.Item label="Reverse tag" name={['settings', 'reverseTag']}>
                          <Input placeholder="optional" />
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
                          label="UDP over TCP"
                          name={['settings', 'uot']}
                          valuePropName="checked"
                        >
                          <Switch />
                        </Form.Item>
                        <Form.Item label="UoT version" name={['settings', 'UoTVersion']}>
                          <InputNumber min={1} max={2} />
                        </Form.Item>
                      </>
                    )}
                  </>
                ),
              },
              {
                key: '2',
                label: 'JSON',
                children: (
                  <Space orientation="vertical" size={10} style={{ width: '100%', marginTop: 10 }}>
                    <JsonEditor
                      value={jsonText}
                      onChange={(next) => {
                        setJsonText(next);
                        setJsonDirty(true);
                      }}
                      minHeight="360px"
                      maxHeight="600px"
                    />
                  </Space>
                ),
              },
            ]}
          />
        </Form>
      </Modal>
    </>
  );
}
