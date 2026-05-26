import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Form,
  Input,
  InputNumber,
  Modal,
  Radio,
  Select,
  Space,
  Switch,
  Tabs,
  message,
} from 'antd';
import { DeleteOutlined, MinusOutlined, PlusOutlined, SyncOutlined } from '@ant-design/icons';

import InputAddon from '@/components/InputAddon';
import JsonEditor from '@/components/JsonEditor';
import { Wireguard } from '@/utils';
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
  ALPN_OPTION,
  Address_Port_Strategy,
  DNSRuleActions,
  MODE_OPTION,
  OutboundDomainStrategies,
  OutboundProtocols as Protocols,
  SNIFFING_OPTION,
  TLS_FLOW_CONTROL,
  USERS_SECURITY,
  UTLS_FINGERPRINT,
  WireguardDomainStrategy,
} from '@/schemas/primitives';
import {
  canEnableReality,
  canEnableStream,
  canEnableTls,
  canEnableTlsFlow,
} from '@/lib/xray/protocol-capabilities';
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
const MODE_OPTIONS = Object.values(MODE_OPTION).map((v) => ({ value: v, label: v }));
const UTLS_OPTIONS = Object.values(UTLS_FINGERPRINT).map((v) => ({ value: v, label: v }));
const ALPN_OPTIONS = Object.values(ALPN_OPTION).map((v) => ({ value: v, label: v }));
const ADDRESS_PORT_STRATEGY_OPTIONS = Object.values(Address_Port_Strategy).map((v) => ({
  value: v,
  label: v,
}));

// canEnableMux mirrors the adapter's helper but lives here so the modal
// can show/hide the Mux section without going through the adapter.
const MUX_PROTOCOLS = new Set<string>(['vmess', 'vless', 'trojan', 'shadowsocks', 'http', 'socks']);
function isMuxAllowed(protocol: string, flow: string, network: string): boolean {
  if (!MUX_PROTOCOLS.has(protocol)) return false;
  if (protocol === 'vless' && flow) return false;
  if (network === 'xhttp') return false;
  return true;
}

const NETWORK_OPTIONS: { value: string; label: string }[] = [
  { value: 'tcp', label: 'TCP (RAW)' },
  { value: 'kcp', label: 'mKCP' },
  { value: 'ws', label: 'WebSocket' },
  { value: 'grpc', label: 'gRPC' },
  { value: 'httpupgrade', label: 'HTTPUpgrade' },
  { value: 'xhttp', label: 'XHTTP' },
];

// Per-network bootstrap. Mirrors the legacy class constructors so the
// initial state for each transport matches what xray-core expects.
function newStreamSlice(network: string): Record<string, unknown> {
  switch (network) {
    case 'tcp':
      return { network: 'tcp', tcpSettings: { header: { type: 'none' } } };
    case 'kcp':
      return {
        network: 'kcp',
        kcpSettings: {
          mtu: 1350, tti: 20, uplinkCapacity: 5, downlinkCapacity: 20,
          cwndMultiplier: 1, maxSendingWindow: 2097152,
        },
      };
    case 'ws':
      return {
        network: 'ws',
        wsSettings: { path: '/', host: '', headers: {}, heartbeatPeriod: 0 },
      };
    case 'grpc':
      return {
        network: 'grpc',
        grpcSettings: { serviceName: '', authority: '', multiMode: false },
      };
    case 'httpupgrade':
      return {
        network: 'httpupgrade',
        httpupgradeSettings: { path: '/', host: '', headers: {} },
      };
    case 'xhttp':
      return {
        network: 'xhttp',
        xhttpSettings: {
          path: '/', host: '', mode: '', headers: [],
          xPaddingBytes: '100-1000', scMaxEachPostBytes: '1000000',
        },
      };
    default:
      return { network: 'tcp', tcpSettings: { header: { type: 'none' } } };
  }
}

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

export default function OutboundFormModal({
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
  const network = (Form.useWatch(['streamSettings', 'network'], form) ?? '') as string;
  const security = (Form.useWatch(['streamSettings', 'security'], form) ?? 'none') as string;

  const streamAllowed = canEnableStream({ protocol });
  const tlsAllowed = canEnableTls({ protocol, streamSettings: { network, security } });
  const realityAllowed = canEnableReality({ protocol, streamSettings: { network, security } });
  const tlsFlowAllowed = canEnableTlsFlow({ protocol, streamSettings: { network, security } });

  // Seed streamSettings when the user picks a protocol that supports
  // streams but the form does not yet have a stream slice (new outbound,
  // or wire payload arrived without streamSettings).
  useEffect(() => {
    if (!streamAllowed) return;
    if (network) return;
    form.setFieldValue('streamSettings', { ...newStreamSlice('tcp'), security: 'none' });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [streamAllowed, network]);

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

  // Security change cascade: swap the security sub-key so the DU branch
  // matches. Seed default field values when entering tls/reality so the
  // sub-forms render without `undefined` field references.
  function onSecurityChange(next: string) {
    const stream = form.getFieldValue('streamSettings') ?? {};
    const cleaned = { ...stream } as Record<string, unknown>;
    delete cleaned.tlsSettings;
    delete cleaned.realitySettings;
    if (next === 'tls') {
      cleaned.tlsSettings = {
        serverName: '',
        alpn: [],
        fingerprint: '',
        echConfigList: '',
        verifyPeerCertByName: '',
        pinnedPeerCertSha256: '',
      };
    } else if (next === 'reality') {
      cleaned.realitySettings = {
        publicKey: '',
        fingerprint: 'chrome',
        serverName: '',
        shortId: '',
        spiderX: '',
        mldsa65Verify: '',
      };
    }
    cleaned.security = next;
    form.setFieldValue('streamSettings', cleaned);
  }

  // Network change cascade: swap the per-network sub-key (tcpSettings,
  // wsSettings, etc.) so the DU branch matches. Preserve security if
  // the new network supports it, otherwise force back to 'none'.
  function onNetworkChange(next: string) {
    const currentSecurity = form.getFieldValue(['streamSettings', 'security']) ?? 'none';
    const stillAllowed = canEnableTls({ protocol, streamSettings: { network: next, security: currentSecurity } });
    const stillReality = canEnableReality({ protocol, streamSettings: { network: next, security: currentSecurity } });
    const newSecurity =
      currentSecurity === 'tls' && !stillAllowed
        ? 'none'
        : currentSecurity === 'reality' && !stillReality
          ? 'none'
          : currentSecurity;
    form.setFieldValue('streamSettings', { ...newStreamSlice(next), security: newSecurity });
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

                    {protocol === 'hysteria' && (
                      <Form.Item label="Version" name={['settings', 'version']}>
                        <InputNumber min={2} max={2} disabled />
                      </Form.Item>
                    )}

                    {protocol === 'loopback' && (
                      <Form.Item label="Inbound tag" name={['settings', 'inboundTag']}>
                        <Input placeholder="inbound tag used in routing rules" />
                      </Form.Item>
                    )}

                    {protocol === 'blackhole' && (
                      <Form.Item label="Response type" name={['settings', 'type']}>
                        <Select
                          options={[
                            { value: '', label: '(empty)' },
                            { value: 'none', label: 'none' },
                            { value: 'http', label: 'http' },
                          ]}
                        />
                      </Form.Item>
                    )}

                    {protocol === 'dns' && (
                      <>
                        <Form.Item label="Rewrite network" name={['settings', 'rewriteNetwork']}>
                          <Select
                            allowClear
                            placeholder="(unchanged)"
                            options={[
                              { value: 'udp', label: 'udp' },
                              { value: 'tcp', label: 'tcp' },
                            ]}
                          />
                        </Form.Item>
                        <Form.Item label="Rewrite address" name={['settings', 'rewriteAddress']}>
                          <Input placeholder="(unchanged) e.g. 1.1.1.1" />
                        </Form.Item>
                        <Form.Item label="Rewrite port" name={['settings', 'rewritePort']}>
                          <InputNumber min={0} max={65535} style={{ width: '100%' }} />
                        </Form.Item>
                        <Form.Item label="User level" name={['settings', 'userLevel']}>
                          <InputNumber min={0} style={{ width: '100%' }} />
                        </Form.Item>
                        <Form.List name={['settings', 'rules']}>
                          {(fields, { add, remove }) => (
                            <>
                              <Form.Item label="Rules">
                                <Button
                                  size="small"
                                  type="primary"
                                  icon={<PlusOutlined />}
                                  onClick={() => add({ action: 'direct', qtype: '', domain: '' })}
                                />
                              </Form.Item>
                              {fields.map((field, index) => (
                                <div key={field.key}>
                                  <Form.Item wrapperCol={{ md: { span: 14, offset: 8 } }}>
                                    <div className="item-heading">
                                      <span>Rule {index + 1}</span>
                                      <DeleteOutlined
                                        className="danger-icon"
                                        onClick={() => remove(field.name)}
                                      />
                                    </div>
                                  </Form.Item>
                                  <Form.Item label="Action" name={[field.name, 'action']}>
                                    <Select
                                      options={DNSRuleActions.map((a) => ({ value: a, label: a }))}
                                    />
                                  </Form.Item>
                                  <Form.Item label="QType" name={[field.name, 'qtype']}>
                                    <Input placeholder="1,3,23-24" />
                                  </Form.Item>
                                  <Form.Item label={t('domainName')} name={[field.name, 'domain']}>
                                    <Input placeholder="domain:example.com" />
                                  </Form.Item>
                                </div>
                              ))}
                            </>
                          )}
                        </Form.List>
                      </>
                    )}

                    {protocol === 'freedom' && (
                      <>
                        <Form.Item label="Strategy" name={['settings', 'domainStrategy']}>
                          <Select
                            options={[
                              { value: '', label: `(${t('none')})` },
                              ...OutboundDomainStrategies.map((s) => ({ value: s, label: s })),
                            ]}
                          />
                        </Form.Item>
                        <Form.Item label="Redirect" name={['settings', 'redirect']}>
                          <Input />
                        </Form.Item>

                        <Form.Item label="Fragment" shouldUpdate noStyle>
                          {() => {
                            const fragment = (form.getFieldValue(['settings', 'fragment']) ?? {}) as {
                              packets?: string;
                              length?: string;
                              interval?: string;
                              maxSplit?: string;
                            };
                            const enabled = !!(fragment.length || fragment.interval || fragment.maxSplit);
                            return (
                              <>
                                <Form.Item label="Fragment">
                                  <Switch
                                    checked={enabled}
                                    onChange={(checked) => {
                                      form.setFieldValue(
                                        ['settings', 'fragment'],
                                        checked
                                          ? {
                                              packets: 'tlshello',
                                              length: '100-200',
                                              interval: '10-20',
                                              maxSplit: '300-400',
                                            }
                                          : { packets: '', length: '', interval: '', maxSplit: '' },
                                      );
                                    }}
                                  />
                                </Form.Item>
                                {enabled && (
                                  <>
                                    <Form.Item
                                      label="Packets"
                                      name={['settings', 'fragment', 'packets']}
                                    >
                                      <Select
                                        options={[
                                          { value: '1-3', label: '1-3' },
                                          { value: 'tlshello', label: 'tlshello' },
                                        ]}
                                      />
                                    </Form.Item>
                                    <Form.Item label="Length" name={['settings', 'fragment', 'length']}>
                                      <Input />
                                    </Form.Item>
                                    <Form.Item
                                      label="Interval"
                                      name={['settings', 'fragment', 'interval']}
                                    >
                                      <Input />
                                    </Form.Item>
                                    <Form.Item
                                      label="Max Split"
                                      name={['settings', 'fragment', 'maxSplit']}
                                    >
                                      <Input />
                                    </Form.Item>
                                  </>
                                )}
                              </>
                            );
                          }}
                        </Form.Item>

                        <Form.List name={['settings', 'noises']}>
                          {(fields, { add, remove }) => (
                            <>
                              <Form.Item label="Noises">
                                <Switch
                                  checked={fields.length > 0}
                                  onChange={(checked) => {
                                    if (checked) {
                                      add({
                                        type: 'rand',
                                        packet: '10-20',
                                        delay: '10-16',
                                        applyTo: 'ip',
                                      });
                                    } else {
                                      // remove() with no arg is not supported;
                                      // walk fields in reverse and drop each.
                                      for (let i = fields.length - 1; i >= 0; i--) {
                                        remove(fields[i].name);
                                      }
                                    }
                                  }}
                                />
                                {fields.length > 0 && (
                                  <Button
                                    size="small"
                                    type="primary"
                                    className="ml-8"
                                    icon={<PlusOutlined />}
                                    onClick={() =>
                                      add({
                                        type: 'rand',
                                        packet: '10-20',
                                        delay: '10-16',
                                        applyTo: 'ip',
                                      })
                                    }
                                  />
                                )}
                              </Form.Item>
                              {fields.map((field, index) => (
                                <div key={field.key}>
                                  <Form.Item wrapperCol={{ md: { span: 14, offset: 8 } }}>
                                    <div className="item-heading">
                                      <span>Noise {index + 1}</span>
                                      {fields.length > 1 && (
                                        <DeleteOutlined
                                          className="danger-icon"
                                          onClick={() => remove(field.name)}
                                        />
                                      )}
                                    </div>
                                  </Form.Item>
                                  <Form.Item label="Type" name={[field.name, 'type']}>
                                    <Select
                                      options={['rand', 'base64', 'str', 'hex'].map((v) => ({
                                        value: v,
                                        label: v,
                                      }))}
                                    />
                                  </Form.Item>
                                  <Form.Item label="Packet" name={[field.name, 'packet']}>
                                    <Input />
                                  </Form.Item>
                                  <Form.Item label="Delay (ms)" name={[field.name, 'delay']}>
                                    <Input />
                                  </Form.Item>
                                  <Form.Item label="Apply to" name={[field.name, 'applyTo']}>
                                    <Select
                                      options={['ip', 'ipv4', 'ipv6'].map((v) => ({
                                        value: v,
                                        label: v,
                                      }))}
                                    />
                                  </Form.Item>
                                </div>
                              ))}
                            </>
                          )}
                        </Form.List>

                        <Form.List name={['settings', 'finalRules']}>
                          {(fields, { add, remove }) => (
                            <>
                              <Form.Item label="Final Rules">
                                <Button
                                  size="small"
                                  type="primary"
                                  icon={<PlusOutlined />}
                                  onClick={() =>
                                    add({
                                      action: 'allow',
                                      network: '',
                                      port: '',
                                      ip: [],
                                      blockDelay: '',
                                    })
                                  }
                                />
                                <span className="ml-8" style={{ opacity: 0.6 }}>
                                  Override Xray&apos;s default private-IP block
                                </span>
                              </Form.Item>
                              {fields.map((field, index) => (
                                <div key={field.key}>
                                  <Form.Item wrapperCol={{ md: { span: 14, offset: 8 } }}>
                                    <div className="item-heading">
                                      <span>Rule {index + 1}</span>
                                      <DeleteOutlined
                                        className="danger-icon"
                                        onClick={() => remove(field.name)}
                                      />
                                    </div>
                                  </Form.Item>
                                  <Form.Item label="Action" name={[field.name, 'action']}>
                                    <Select
                                      options={['allow', 'block'].map((v) => ({
                                        value: v,
                                        label: v,
                                      }))}
                                    />
                                  </Form.Item>
                                  <Form.Item label="Network" name={[field.name, 'network']}>
                                    <Select
                                      allowClear
                                      placeholder="(any)"
                                      options={['tcp', 'udp', 'tcp,udp'].map((v) => ({
                                        value: v,
                                        label: v,
                                      }))}
                                    />
                                  </Form.Item>
                                  <Form.Item label="Port" name={[field.name, 'port']}>
                                    <Input placeholder="e.g. 80,443 or 1000-2000" />
                                  </Form.Item>
                                  <Form.Item label="IP / CIDR / geoip" name={[field.name, 'ip']}>
                                    <Select
                                      mode="tags"
                                      tokenSeparators={[',', ' ']}
                                      placeholder="e.g. 10.0.0.0/8, geoip:private"
                                    />
                                  </Form.Item>
                                  <Form.Item shouldUpdate noStyle>
                                    {() => {
                                      const ruleAction = form.getFieldValue([
                                        'settings',
                                        'finalRules',
                                        field.name,
                                        'action',
                                      ]);
                                      if (ruleAction !== 'block') return null;
                                      return (
                                        <Form.Item
                                          label="Block delay (ms)"
                                          name={[field.name, 'blockDelay']}
                                        >
                                          <Input placeholder="optional: 5000-10000" />
                                        </Form.Item>
                                      );
                                    }}
                                  </Form.Item>
                                </div>
                              ))}
                            </>
                          )}
                        </Form.List>
                      </>
                    )}

                    {protocol === 'vless' && (
                      <Form.Item shouldUpdate noStyle>
                        {() => {
                          const reverseTag = form.getFieldValue(['settings', 'reverseTag']);
                          if (!reverseTag) return null;
                          const sniff = (form.getFieldValue(['settings', 'reverseSniffing']) ?? {}) as {
                            enabled?: boolean;
                          };
                          return (
                            <>
                              <Form.Item
                                label="Reverse Sniffing"
                                name={['settings', 'reverseSniffing', 'enabled']}
                                valuePropName="checked"
                              >
                                <Switch />
                              </Form.Item>
                              {sniff.enabled && (
                                <>
                                  <Form.Item
                                    wrapperCol={{ md: { span: 14, offset: 8 } }}
                                    name={['settings', 'reverseSniffing', 'destOverride']}
                                  >
                                    <Select
                                      mode="multiple"
                                      className="sniffing-options"
                                      options={Object.entries(SNIFFING_OPTION).map(([k, v]) => ({
                                        value: v,
                                        label: k,
                                      }))}
                                    />
                                  </Form.Item>
                                  <Form.Item
                                    label="Metadata Only"
                                    name={['settings', 'reverseSniffing', 'metadataOnly']}
                                    valuePropName="checked"
                                  >
                                    <Switch />
                                  </Form.Item>
                                  <Form.Item
                                    label="Route Only"
                                    name={['settings', 'reverseSniffing', 'routeOnly']}
                                    valuePropName="checked"
                                  >
                                    <Switch />
                                  </Form.Item>
                                  <Form.Item
                                    label="IPs Excluded"
                                    name={['settings', 'reverseSniffing', 'ipsExcluded']}
                                  >
                                    <Select
                                      mode="tags"
                                      tokenSeparators={[',']}
                                      placeholder="IP/CIDR/geoip:*"
                                    />
                                  </Form.Item>
                                  <Form.Item
                                    label="Domains Excluded"
                                    name={['settings', 'reverseSniffing', 'domainsExcluded']}
                                  >
                                    <Select
                                      mode="tags"
                                      tokenSeparators={[',']}
                                      placeholder="domain:*"
                                    />
                                  </Form.Item>
                                </>
                              )}
                            </>
                          );
                        }}
                      </Form.Item>
                    )}

                    {protocol === 'wireguard' && (
                      <>
                        <Form.Item label={t('pages.inbounds.address')} name={['settings', 'address']}>
                          <Input placeholder="comma-separated, e.g. 10.0.0.1,fd00::1" />
                        </Form.Item>
                        <Form.Item
                          label={
                            <>
                              {t('pages.inbounds.privatekey')}
                              <SyncOutlined
                                className="random-icon"
                                onClick={() => {
                                  const pair = Wireguard.generateKeypair();
                                  form.setFieldValue(['settings', 'secretKey'], pair.privateKey);
                                  form.setFieldValue(['settings', 'pubKey'], pair.publicKey);
                                }}
                              />
                            </>
                          }
                          name={['settings', 'secretKey']}
                        >
                          <Input />
                        </Form.Item>
                        <Form.Item label={t('pages.inbounds.publicKey')} name={['settings', 'pubKey']}>
                          <Input disabled />
                        </Form.Item>
                        <Form.Item label="Domain strategy" name={['settings', 'domainStrategy']}>
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
                        <Form.Item label="Workers" name={['settings', 'workers']}>
                          <InputNumber min={0} />
                        </Form.Item>
                        <Form.Item
                          label="No-kernel TUN"
                          name={['settings', 'noKernelTun']}
                          valuePropName="checked"
                        >
                          <Switch />
                        </Form.Item>
                        <Form.Item label="Reserved" name={['settings', 'reserved']}>
                          <Input placeholder="comma-separated bytes, e.g. 1,2,3" />
                        </Form.Item>
                        <Form.List name={['settings', 'peers']}>
                          {(fields, { add, remove }) => (
                            <>
                              <Form.Item label="Peers">
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
                                      <span>Peer {index + 1}</span>
                                      {fields.length > 1 && (
                                        <DeleteOutlined
                                          className="danger-icon"
                                          onClick={() => remove(field.name)}
                                        />
                                      )}
                                    </div>
                                  </Form.Item>
                                  <Form.Item label="Endpoint" name={[field.name, 'endpoint']}>
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
                                  <Form.Item label="Allowed IPs">
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
                                  <Form.Item label="Keep alive" name={[field.name, 'keepAlive']}>
                                    <InputNumber min={0} />
                                  </Form.Item>
                                </div>
                              ))}
                            </>
                          )}
                        </Form.List>
                      </>
                    )}

                    {streamAllowed && network && (
                      <>
                        <Form.Item
                          label={t('transmission')}
                          name={['streamSettings', 'network']}
                        >
                          <Select
                            value={network}
                            onChange={onNetworkChange}
                            options={NETWORK_OPTIONS}
                          />
                        </Form.Item>

                        {network === 'tcp' && (
                          <Form.Item shouldUpdate noStyle>
                            {() => {
                              const type =
                                form.getFieldValue([
                                  'streamSettings',
                                  'tcpSettings',
                                  'header',
                                  'type',
                                ]) ?? 'none';
                              return (
                                <>
                                  <Form.Item label={`HTTP ${t('camouflage')}`}>
                                    <Switch
                                      checked={type === 'http'}
                                      onChange={(checked) =>
                                        form.setFieldValue(
                                          ['streamSettings', 'tcpSettings', 'header'],
                                          checked
                                            ? { type: 'http', request: undefined, response: undefined }
                                            : { type: 'none' },
                                        )
                                      }
                                    />
                                  </Form.Item>
                                </>
                              );
                            }}
                          </Form.Item>
                        )}

                        {network === 'kcp' && (
                          <>
                            <Form.Item label="MTU" name={['streamSettings', 'kcpSettings', 'mtu']}>
                              <InputNumber min={0} />
                            </Form.Item>
                            <Form.Item label="TTI (ms)" name={['streamSettings', 'kcpSettings', 'tti']}>
                              <InputNumber min={0} />
                            </Form.Item>
                            <Form.Item
                              label="Uplink (MB/s)"
                              name={['streamSettings', 'kcpSettings', 'uplinkCapacity']}
                            >
                              <InputNumber min={0} />
                            </Form.Item>
                            <Form.Item
                              label="Downlink (MB/s)"
                              name={['streamSettings', 'kcpSettings', 'downlinkCapacity']}
                            >
                              <InputNumber min={0} />
                            </Form.Item>
                            <Form.Item
                              label="CWND multiplier"
                              name={['streamSettings', 'kcpSettings', 'cwndMultiplier']}
                            >
                              <InputNumber min={1} />
                            </Form.Item>
                            <Form.Item
                              label="Max sending window"
                              name={['streamSettings', 'kcpSettings', 'maxSendingWindow']}
                            >
                              <InputNumber min={0} />
                            </Form.Item>
                          </>
                        )}

                        {network === 'ws' && (
                          <>
                            <Form.Item label={t('host')} name={['streamSettings', 'wsSettings', 'host']}>
                              <Input />
                            </Form.Item>
                            <Form.Item label={t('path')} name={['streamSettings', 'wsSettings', 'path']}>
                              <Input />
                            </Form.Item>
                            <Form.Item
                              label="Heartbeat (s)"
                              name={['streamSettings', 'wsSettings', 'heartbeatPeriod']}
                            >
                              <InputNumber min={0} />
                            </Form.Item>
                          </>
                        )}

                        {network === 'grpc' && (
                          <>
                            <Form.Item
                              label="Service name"
                              name={['streamSettings', 'grpcSettings', 'serviceName']}
                            >
                              <Input />
                            </Form.Item>
                            <Form.Item
                              label="Authority"
                              name={['streamSettings', 'grpcSettings', 'authority']}
                            >
                              <Input />
                            </Form.Item>
                            <Form.Item
                              label="Multi mode"
                              name={['streamSettings', 'grpcSettings', 'multiMode']}
                              valuePropName="checked"
                            >
                              <Switch />
                            </Form.Item>
                          </>
                        )}

                        {network === 'httpupgrade' && (
                          <>
                            <Form.Item
                              label={t('host')}
                              name={['streamSettings', 'httpupgradeSettings', 'host']}
                            >
                              <Input />
                            </Form.Item>
                            <Form.Item
                              label={t('path')}
                              name={['streamSettings', 'httpupgradeSettings', 'path']}
                            >
                              <Input />
                            </Form.Item>
                          </>
                        )}

                        {network === 'xhttp' && (
                          <>
                            <Form.Item
                              label={t('host')}
                              name={['streamSettings', 'xhttpSettings', 'host']}
                            >
                              <Input />
                            </Form.Item>
                            <Form.Item
                              label={t('path')}
                              name={['streamSettings', 'xhttpSettings', 'path']}
                            >
                              <Input />
                            </Form.Item>
                            <Form.Item
                              label="Mode"
                              name={['streamSettings', 'xhttpSettings', 'mode']}
                            >
                              <Select options={MODE_OPTIONS} />
                            </Form.Item>
                            <Form.Item
                              label="Padding Bytes"
                              name={['streamSettings', 'xhttpSettings', 'xPaddingBytes']}
                            >
                              <Input />
                            </Form.Item>
                            <div style={{ marginTop: 4, opacity: 0.6, fontStyle: 'italic' }}>
                              XHTTP advanced fields (XMUX, sequence/session placement,
                              padding obfs) are still being migrated — edit them via
                              the JSON tab.
                            </div>
                          </>
                        )}
                      </>
                    )}

                    {tlsFlowAllowed && (
                      <Form.Item label="Flow" name={['settings', 'flow']}>
                        <Select
                          allowClear
                          placeholder={t('none')}
                          options={FLOW_OPTIONS}
                        />
                      </Form.Item>
                    )}

                    {streamAllowed && network && (
                      <Form.Item label={t('security')}>
                        <Radio.Group
                          value={security}
                          buttonStyle="solid"
                          onChange={(e) => onSecurityChange(e.target.value as string)}
                        >
                          <Radio.Button value="none">{t('none')}</Radio.Button>
                          {tlsAllowed && <Radio.Button value="tls">TLS</Radio.Button>}
                          {realityAllowed && <Radio.Button value="reality">Reality</Radio.Button>}
                        </Radio.Group>
                      </Form.Item>
                    )}

                    {security === 'tls' && tlsAllowed && (
                      <>
                        <Form.Item
                          label="SNI"
                          name={['streamSettings', 'tlsSettings', 'serverName']}
                        >
                          <Input placeholder="server name" />
                        </Form.Item>
                        <Form.Item
                          label="uTLS"
                          name={['streamSettings', 'tlsSettings', 'fingerprint']}
                        >
                          <Select
                            allowClear
                            placeholder={t('none')}
                            options={UTLS_OPTIONS}
                          />
                        </Form.Item>
                        <Form.Item
                          label="ALPN"
                          name={['streamSettings', 'tlsSettings', 'alpn']}
                        >
                          <Select mode="multiple" options={ALPN_OPTIONS} />
                        </Form.Item>
                        <Form.Item
                          label="ECH"
                          name={['streamSettings', 'tlsSettings', 'echConfigList']}
                        >
                          <Input />
                        </Form.Item>
                        <Form.Item
                          label="Verify peer name"
                          name={['streamSettings', 'tlsSettings', 'verifyPeerCertByName']}
                        >
                          <Input placeholder="cloudflare-dns.com" />
                        </Form.Item>
                        <Form.Item
                          label="Pinned SHA256"
                          name={['streamSettings', 'tlsSettings', 'pinnedPeerCertSha256']}
                        >
                          <Input placeholder="base64 SHA256" />
                        </Form.Item>
                      </>
                    )}

                    {security === 'reality' && realityAllowed && (
                      <>
                        <Form.Item
                          label="SNI"
                          name={['streamSettings', 'realitySettings', 'serverName']}
                        >
                          <Input />
                        </Form.Item>
                        <Form.Item
                          label="uTLS"
                          name={['streamSettings', 'realitySettings', 'fingerprint']}
                        >
                          <Select options={UTLS_OPTIONS} />
                        </Form.Item>
                        <Form.Item
                          label="Short ID"
                          name={['streamSettings', 'realitySettings', 'shortId']}
                        >
                          <Input />
                        </Form.Item>
                        <Form.Item
                          label="SpiderX"
                          name={['streamSettings', 'realitySettings', 'spiderX']}
                        >
                          <Input />
                        </Form.Item>
                        <Form.Item
                          label={t('pages.inbounds.publicKey')}
                          name={['streamSettings', 'realitySettings', 'publicKey']}
                        >
                          <Input.TextArea autoSize={{ minRows: 2 }} />
                        </Form.Item>
                        <Form.Item
                          label="mldsa65 verify"
                          name={['streamSettings', 'realitySettings', 'mldsa65Verify']}
                        >
                          <Input.TextArea autoSize={{ minRows: 2 }} />
                        </Form.Item>
                      </>
                    )}

                    {streamAllowed && network && (
                      <Form.Item shouldUpdate noStyle>
                        {() => {
                          const hasSockopt = !!form.getFieldValue([
                            'streamSettings',
                            'sockopt',
                          ]);
                          return (
                            <>
                              <Form.Item label="Sockopts">
                                <Switch
                                  checked={hasSockopt}
                                  onChange={(checked) => {
                                    form.setFieldValue(
                                      ['streamSettings', 'sockopt'],
                                      checked
                                        ? {
                                            acceptProxyProtocol: false,
                                            tcpFastOpen: false,
                                            mark: 0,
                                            tproxy: 'off',
                                            tcpMptcp: false,
                                            penetrate: false,
                                            domainStrategy: 'UseIP',
                                            tcpMaxSeg: 1440,
                                            dialerProxy: '',
                                            tcpKeepAliveInterval: 0,
                                            tcpKeepAliveIdle: 300,
                                            tcpUserTimeout: 10000,
                                            tcpcongestion: 'bbr',
                                            V6Only: false,
                                            tcpWindowClamp: 600,
                                            interfaceName: '',
                                            trustedXForwardedFor: [],
                                          }
                                        : undefined,
                                    );
                                  }}
                                />
                              </Form.Item>
                              {hasSockopt && (
                                <>
                                  <Form.Item
                                    label="Dialer proxy"
                                    name={['streamSettings', 'sockopt', 'dialerProxy']}
                                  >
                                    <Input />
                                  </Form.Item>
                                  <Form.Item
                                    label="Domain strategy"
                                    name={['streamSettings', 'sockopt', 'domainStrategy']}
                                  >
                                    <Select
                                      options={ADDRESS_PORT_STRATEGY_OPTIONS}
                                    />
                                  </Form.Item>
                                  <Form.Item
                                    label="Keep alive interval"
                                    name={['streamSettings', 'sockopt', 'tcpKeepAliveInterval']}
                                  >
                                    <InputNumber min={0} />
                                  </Form.Item>
                                  <Form.Item
                                    label="TCP Fast Open"
                                    name={['streamSettings', 'sockopt', 'tcpFastOpen']}
                                    valuePropName="checked"
                                  >
                                    <Switch />
                                  </Form.Item>
                                  <Form.Item
                                    label="Multipath TCP"
                                    name={['streamSettings', 'sockopt', 'tcpMptcp']}
                                    valuePropName="checked"
                                  >
                                    <Switch />
                                  </Form.Item>
                                  <Form.Item
                                    label="Penetrate"
                                    name={['streamSettings', 'sockopt', 'penetrate']}
                                    valuePropName="checked"
                                  >
                                    <Switch />
                                  </Form.Item>
                                  <Form.Item
                                    label="Mark (fwmark)"
                                    name={['streamSettings', 'sockopt', 'mark']}
                                  >
                                    <InputNumber min={0} />
                                  </Form.Item>
                                  <Form.Item
                                    label="Interface"
                                    name={['streamSettings', 'sockopt', 'interfaceName']}
                                  >
                                    <Input />
                                  </Form.Item>
                                </>
                              )}
                            </>
                          );
                        }}
                      </Form.Item>
                    )}

                    {(() => {
                      const flow = (form.getFieldValue(['settings', 'flow']) ?? '') as string;
                      if (!isMuxAllowed(protocol, flow, network)) return null;
                      return (
                        <Form.Item shouldUpdate noStyle>
                          {() => {
                            const muxEnabled = !!form.getFieldValue(['mux', 'enabled']);
                            return (
                              <>
                                <Form.Item
                                  label={t('pages.settings.mux')}
                                  name={['mux', 'enabled']}
                                  valuePropName="checked"
                                >
                                  <Switch />
                                </Form.Item>
                                {muxEnabled && (
                                  <>
                                    <Form.Item
                                      label="Concurrency"
                                      name={['mux', 'concurrency']}
                                    >
                                      <InputNumber min={-1} max={1024} />
                                    </Form.Item>
                                    <Form.Item
                                      label="xudp concurrency"
                                      name={['mux', 'xudpConcurrency']}
                                    >
                                      <InputNumber min={-1} max={1024} />
                                    </Form.Item>
                                    <Form.Item
                                      label="xudp UDP 443"
                                      name={['mux', 'xudpProxyUDP443']}
                                    >
                                      <Select
                                        options={['reject', 'allow', 'skip'].map((v) => ({
                                          value: v,
                                          label: v,
                                        }))}
                                      />
                                    </Form.Item>
                                  </>
                                )}
                              </>
                            );
                          }}
                        </Form.Item>
                      );
                    })()}
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
