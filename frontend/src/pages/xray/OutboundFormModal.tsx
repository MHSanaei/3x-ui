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
import { DeleteOutlined, MinusOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';

import FinalMaskForm from '@/components/FinalMaskForm';
import HeaderMapEditor from '@/components/HeaderMapEditor';
import HysteriaMasqueradeForm from '@/components/HysteriaMasqueradeForm';
import InputAddon from '@/components/InputAddon';
import JsonEditor from '@/components/JsonEditor';
import { Wireguard } from '@/utils';
import {
  XMUX_DEFAULTS,
  formValuesToWirePayload,
  rawOutboundToFormValues,
} from '@/lib/xray/outbound-form-adapter';
import { parseOutboundLink } from '@/lib/xray/outbound-link-parser';
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
  DOMAIN_STRATEGY_OPTION,
  MODE_OPTION,
  OutboundDomainStrategies,
  OutboundProtocols as Protocols,
  SNIFFING_OPTION,
  TCP_CONGESTION_OPTION,
  TLS_FLOW_CONTROL,
  USERS_SECURITY,
  UTLS_FINGERPRINT,
  WireguardDomainStrategy,
} from '@/schemas/primitives';
import {
  HappyEyeballsSchema,
  SockoptStreamSettingsSchema,
} from '@/schemas/protocols/stream/sockopt';
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
  { value: 'tcp', label: 'RAW' },
  { value: 'kcp', label: 'mKCP' },
  { value: 'ws', label: 'WebSocket' },
  { value: 'grpc', label: 'gRPC' },
  { value: 'httpupgrade', label: 'HTTPUpgrade' },
  { value: 'xhttp', label: 'XHTTP' },
];

// The hysteria protocol is locked to its own QUIC transport: the selector
// shows only this option when the parent protocol is hysteria.
const HYSTERIA_NETWORK_OPTION = { value: 'hysteria', label: 'Hysteria' };

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
    case 'hysteria':
      return {
        network: 'hysteria',
        hysteriaSettings: {
          version: 2,
          auth: '',
          udpIdleTimeout: 60,
        },
      };
    default:
      return { network: 'tcp', tcpSettings: { header: { type: 'none' } } };
  }
}

// Hysteria2 always rides its own QUIC transport with TLS — the panel never
// offers another transport or 'none' security for it.
function hysteriaStreamSlice(): Record<string, unknown> {
  return {
    ...newStreamSlice('hysteria'),
    security: 'tls',
    tlsSettings: {
      serverName: '', alpn: ['h3'], fingerprint: '',
      echConfigList: '', verifyPeerCertByName: '', pinnedPeerCertSha256: '',
    },
  };
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
  const [linkInput, setLinkInput] = useState('');

  // Parse a share link (vmess:// / vless:// / trojan:// / ss:// /
  // hysteria2:// / wireguard://) and replace form state with the result.
  // The current tag is preserved when the parsed link doesn't carry one.
  function importLink() {
    const link = linkInput.trim();
    if (!link) return;
    const parsed = parseOutboundLink(link);
    if (!parsed) {
      messageApi.error('Wrong Link!');
      return;
    }
    const currentTag = form.getFieldValue('tag') as string | undefined;
    if (!parsed.tag && currentTag) parsed.tag = currentTag;
    const next = rawOutboundToFormValues(parsed);
    form.resetFields();
    form.setFieldsValue(next);
    setJsonText(JSON.stringify(formValuesToWirePayload(next), null, 2));
    setJsonDirty(false);
    setLinkInput('');
    messageApi.success('Link imported successfully');
    switchTab('1');
  }

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
  const network = (Form.useWatch(['streamSettings', 'network'], { form, preserve: true }) ?? '') as string;
  const security = (Form.useWatch(['streamSettings', 'security'], { form, preserve: true }) ?? 'none') as string;
  const streamAllowed = canEnableStream({ protocol });
  const tlsAllowed = canEnableTls({ protocol, streamSettings: { network, security } });
  const realityAllowed = canEnableReality({ protocol, streamSettings: { network, security } });
  const tlsFlowAllowed = canEnableTlsFlow({ protocol, streamSettings: { network, security } });

  useEffect(() => {
    if (!streamAllowed) return;
    if (network) return;
    form.setFieldValue('streamSettings', { ...newStreamSlice('tcp'), security: 'none' });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [streamAllowed, network]);

  useEffect(() => {
    if (protocol !== 'hysteria') return;
    if (network === 'hysteria' && security === 'tls') return;
    const existing = (form.getFieldValue('streamSettings') ?? {}) as Record<string, unknown>;
    const slice = hysteriaStreamSlice();
    if (existing.hysteriaSettings) slice.hysteriaSettings = existing.hysteriaSettings;
    if (existing.tlsSettings) slice.tlsSettings = existing.tlsSettings;
    form.setFieldValue('streamSettings', slice);
  }, [protocol, network, security]);

  const wgSecretKey = Form.useWatch(['settings', 'secretKey'], form) as string | undefined;
  useEffect(() => {
    if (protocol !== 'wireguard') return;
    const sk = (wgSecretKey ?? '').trim();
    if (!sk) {
      form.setFieldValue(['settings', 'pubKey'], '');
      return;
    }
    try {
      const { publicKey } = Wireguard.generateKeypair(sk);
      form.setFieldValue(['settings', 'pubKey'], publicKey);
    } catch {
      form.setFieldValue(['settings', 'pubKey'], '');
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [protocol, wgSecretKey]);

  function onValuesChange(changed: Partial<OutboundFormValues>) {
    if ('protocol' in changed && changed.protocol) {
      const next = rawOutboundToFormValues({ protocol: changed.protocol });
      form.setFieldValue('settings', next.settings);
      if (changed.protocol === 'hysteria') {
        form.setFieldValue('streamSettings', hysteriaStreamSlice());
      } else if ((form.getFieldValue(['streamSettings', 'network']) ?? '') === 'hysteria') {
        form.setFieldValue('streamSettings', { ...newStreamSlice('tcp'), security: 'none' });
      }
    }
  }

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
    if (next === 'hysteria') {
      form.setFieldValue('streamSettings', hysteriaStreamSlice());
      return;
    }
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

  function onXmuxToggle(checked: boolean) {
    if (!checked) return;
    const existing = form.getFieldValue(['streamSettings', 'xhttpSettings', 'xmux']);
    const hasValues = existing && typeof existing === 'object' && Object.keys(existing).length > 0;
    if (hasValues) return;
    form.setFieldValue(['streamSettings', 'xhttpSettings', 'xmux'], { ...XMUX_DEFAULTS });
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

  function switchTab(key: string) {
    if (typeof document !== 'undefined') {
      (document.activeElement as HTMLElement | null)?.blur?.();
    }
    setActiveKey(key);
  }

  function onTabChange(key: string) {
    if (key === '2') {
      const values = form.getFieldsValue(true) as OutboundFormValues;
      setJsonText(JSON.stringify(formValuesToWirePayload(values), null, 2));
      setJsonDirty(false);
      switchTab(key);
      return;
    }
    if (key === '1' && activeKey === '2') {
      if (!applyJsonToForm()) return;
    }
    switchTab(key);
  }

  async function onOk() {
    let values: OutboundFormValues;
    if (activeKey === '2') {
      const raw = jsonText.trim();
      if (!raw) return;
      let parsed: Record<string, unknown>;
      try {
        parsed = JSON.parse(raw) as Record<string, unknown>;
      } catch (e) {
        messageApi.error(`JSON: ${(e as Error).message}`);
        return;
      }
      values = rawOutboundToFormValues(parsed);
      form.resetFields();
      form.setFieldsValue(values);
      setJsonDirty(false);
    } else {
      try {
        await form.validateFields();
      } catch {
        return;
      }
      values = form.getFieldsValue(true) as OutboundFormValues;
    }
    const tagValue = (values.tag ?? '').trim();
    if (!tagValue) {
      messageApi.error(t('pages.xray.outboundForm.tagRequired'));
      return;
    }
    const isDuplicateTag = (existingTags || []).includes(tagValue)
      && !(isEdit && (outboundProp?.tag as string | undefined) === tagValue);
    if (isDuplicateTag) {
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
                      label={t('pages.xray.outbound.tag')}
                      name="tag"
                      validateStatus={duplicateTag ? 'warning' : undefined}
                      help={duplicateTag ? t('pages.xray.outboundForm.tagDuplicate') : undefined}
                      rules={[
                        { required: true, message: t('pages.xray.outboundForm.tagRequired') },
                      ]}
                    >
                      <Input placeholder={t('pages.xray.outboundForm.tagPlaceholder')} />
                    </Form.Item>

                    <Form.Item label={t('pages.xray.outbound.sendThrough')} name="sendThrough">
                      <Input placeholder={t('pages.xray.outboundForm.localIpPlaceholder')} />
                    </Form.Item>

                    {/* Shared connect target (address + port) for protocols
                        whose form schema carries them flat at settings root.
                        Hidden for freedom/blackhole/dns/loopback/wireguard. */}
                    {SERVER_PROTOCOLS.has(protocol) && (
                      <>
                        <Form.Item
                          label={t('pages.inbounds.address')}
                          name={['settings', 'address']}
                          rules={[{ required: true, message: t('pages.xray.outboundForm.addressRequired') }]}
                        >
                          <Input />
                        </Form.Item>
                        <Form.Item
                          label={t('pages.inbounds.port')}
                          name={['settings', 'port']}
                          rules={[{ required: true, message: t('pages.xray.outboundForm.portRequired') }]}
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

                    {protocol === 'loopback' && (
                      <Form.Item label={t('pages.xray.outboundForm.inboundTag')} name={['settings', 'inboundTag']}>
                        <Input placeholder={t('pages.xray.outboundForm.inboundTagPlaceholder')} />
                      </Form.Item>
                    )}

                    {protocol === 'blackhole' && (
                      <Form.Item label={t('pages.xray.outboundForm.responseType')} name={['settings', 'type']}>
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
                        <Form.Item label={t('pages.xray.outboundForm.rewriteNetwork')} name={['settings', 'rewriteNetwork']}>
                          <Select
                            allowClear
                            placeholder={t('pages.xray.outboundForm.unchanged')}
                            options={[
                              { value: 'udp', label: 'udp' },
                              { value: 'tcp', label: 'tcp' },
                            ]}
                          />
                        </Form.Item>
                        <Form.Item label={t('pages.inbounds.form.rewriteAddress')} name={['settings', 'rewriteAddress']}>
                          <Input placeholder={t('pages.xray.outboundForm.unchangedAddress')} />
                        </Form.Item>
                        <Form.Item label={t('pages.inbounds.form.rewritePort')} name={['settings', 'rewritePort']}>
                          <InputNumber min={0} max={65535} style={{ width: '100%' }} />
                        </Form.Item>
                        <Form.Item label={t('pages.xray.tun.userLevel')} name={['settings', 'userLevel']}>
                          <InputNumber min={0} style={{ width: '100%' }} />
                        </Form.Item>
                        <Form.List name={['settings', 'rules']}>
                          {(fields, { add, remove }) => (
                            <>
                              <Form.Item label={t('pages.xray.outboundForm.rules')}>
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
                                      <span>{t('pages.xray.outboundForm.ruleN', { n: index + 1 })}</span>
                                      <DeleteOutlined
                                        className="danger-icon"
                                        onClick={() => remove(field.name)}
                                      />
                                    </div>
                                  </Form.Item>
                                  <Form.Item label={t('pages.xray.outboundForm.action')} name={[field.name, 'action']}>
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
                        <Form.Item label={t('pages.xray.balancer.balancerStrategy')} name={['settings', 'domainStrategy']}>
                          <Select
                            options={[
                              { value: '', label: `(${t('none')})` },
                              ...OutboundDomainStrategies.map((s) => ({ value: s, label: s })),
                            ]}
                          />
                        </Form.Item>
                        <Form.Item label={t('pages.xray.outboundForm.redirect')} name={['settings', 'redirect']}>
                          <Input />
                        </Form.Item>
                        <Form.Item label={t('pages.xray.outboundForm.proxyProtocol')} name={['settings', 'proxyProtocol']}>
                          <Select
                            options={[
                              { value: 0, label: `(${t('none')})` },
                              { value: 1, label: 'v1' },
                              { value: 2, label: 'v2' },
                            ]}
                          />
                        </Form.Item>

                        <Form.Item label={t('pages.xray.outboundForm.fragment')} shouldUpdate noStyle>
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
                                      label={t('pages.settings.subFormats.packets')}
                                      name={['settings', 'fragment', 'packets']}
                                    >
                                      <Select
                                        options={[
                                          { value: '1-3', label: '1-3' },
                                          { value: 'tlshello', label: 'tlshello' },
                                        ]}
                                      />
                                    </Form.Item>
                                    <Form.Item label={t('pages.settings.subFormats.length')} name={['settings', 'fragment', 'length']}>
                                      <Input />
                                    </Form.Item>
                                    <Form.Item
                                      label={t('pages.settings.subFormats.interval')}
                                      name={['settings', 'fragment', 'interval']}
                                    >
                                      <Input />
                                    </Form.Item>
                                    <Form.Item
                                      label={t('pages.settings.subFormats.maxSplit')}
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
                              <Form.Item label={t('pages.settings.subFormats.noises')}>
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
                                      <span>{t('pages.settings.subFormats.noiseItem', { n: index + 1 })}</span>
                                      {fields.length > 1 && (
                                        <DeleteOutlined
                                          className="danger-icon"
                                          onClick={() => remove(field.name)}
                                        />
                                      )}
                                    </div>
                                  </Form.Item>
                                  <Form.Item label={t('pages.settings.subFormats.type')} name={[field.name, 'type']}>
                                    <Select
                                      options={['rand', 'base64', 'str', 'hex'].map((v) => ({
                                        value: v,
                                        label: v,
                                      }))}
                                    />
                                  </Form.Item>
                                  <Form.Item label={t('pages.settings.subFormats.packet')} name={[field.name, 'packet']}>
                                    <Input />
                                  </Form.Item>
                                  <Form.Item label={t('pages.settings.subFormats.delayMs')} name={[field.name, 'delay']}>
                                    <Input />
                                  </Form.Item>
                                  <Form.Item label={t('pages.settings.subFormats.applyTo')} name={[field.name, 'applyTo']}>
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
                              <Form.Item label={t('pages.xray.outboundForm.finalRules')}>
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
                                  {t('pages.xray.outboundForm.overrideXrayPrivateIp')}
                                </span>
                              </Form.Item>
                              {fields.map((field, index) => (
                                <div key={field.key}>
                                  <Form.Item wrapperCol={{ md: { span: 14, offset: 8 } }}>
                                    <div className="item-heading">
                                      <span>{t('pages.xray.outboundForm.ruleN', { n: index + 1 })}</span>
                                      <DeleteOutlined
                                        className="danger-icon"
                                        onClick={() => remove(field.name)}
                                      />
                                    </div>
                                  </Form.Item>
                                  <Form.Item label={t('pages.xray.outboundForm.action')} name={[field.name, 'action']}>
                                    <Select
                                      options={['allow', 'block'].map((v) => ({
                                        value: v,
                                        label: v,
                                      }))}
                                    />
                                  </Form.Item>
                                  <Form.Item label={t('pages.inbounds.network')} name={[field.name, 'network']}>
                                    <Select
                                      allowClear
                                      placeholder="(any)"
                                      options={['tcp', 'udp', 'tcp,udp'].map((v) => ({
                                        value: v,
                                        label: v,
                                      }))}
                                    />
                                  </Form.Item>
                                  <Form.Item label={t('pages.inbounds.port')} name={[field.name, 'port']}>
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
                                          label={t('pages.xray.outboundForm.blockDelay')}
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
                                label={t('pages.xray.outboundForm.reverseSniffing')}
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
                                    label={t('pages.inbounds.sniffingMetadataOnly')}
                                    name={['settings', 'reverseSniffing', 'metadataOnly']}
                                    valuePropName="checked"
                                  >
                                    <Switch />
                                  </Form.Item>
                                  <Form.Item
                                    label={t('pages.inbounds.sniffingRouteOnly')}
                                    name={['settings', 'reverseSniffing', 'routeOnly']}
                                    valuePropName="checked"
                                  >
                                    <Switch />
                                  </Form.Item>
                                  <Form.Item
                                    label={t('pages.inbounds.sniffingIpsExcluded')}
                                    name={['settings', 'reverseSniffing', 'ipsExcluded']}
                                  >
                                    <Select
                                      mode="tags"
                                      tokenSeparators={[',']}
                                      placeholder="IP/CIDR/geoip:*"
                                    />
                                  </Form.Item>
                                  <Form.Item
                                    label={t('pages.inbounds.sniffingDomainsExcluded')}
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
                            options={
                              protocol === 'hysteria'
                                ? [HYSTERIA_NETWORK_OPTION]
                                : NETWORK_OPTIONS
                            }
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
                                            ? {
                                              type: 'http',
                                              request: {
                                                version: '1.1',
                                                method: 'GET',
                                                path: ['/'],
                                                headers: {},
                                              },
                                              response: {
                                                version: '1.1',
                                                status: '200',
                                                reason: 'OK',
                                                headers: {},
                                              },
                                            }
                                            : { type: 'none' },
                                        )
                                      }
                                    />
                                  </Form.Item>
                                  {type === 'http' && (
                                    <>
                                      <Form.Item
                                        label={t('pages.inbounds.form.requestMethod')}
                                        name={[
                                          'streamSettings', 'tcpSettings', 'header',
                                          'request', 'method',
                                        ]}
                                      >
                                        <Input placeholder="GET" />
                                      </Form.Item>
                                      <Form.Item
                                        label={t('pages.inbounds.form.requestVersion')}
                                        name={[
                                          'streamSettings', 'tcpSettings', 'header',
                                          'request', 'version',
                                        ]}
                                      >
                                        <Input placeholder="1.1" />
                                      </Form.Item>
                                      <Form.Item
                                        label={t('pages.inbounds.form.requestHeaders')}
                                        name={[
                                          'streamSettings', 'tcpSettings', 'header',
                                          'request', 'headers',
                                        ]}
                                      >
                                        <HeaderMapEditor mode="v2" />
                                      </Form.Item>

                                      <Form.Item
                                        label={t('pages.inbounds.form.responseVersion')}
                                        name={[
                                          'streamSettings', 'tcpSettings', 'header',
                                          'response', 'version',
                                        ]}
                                      >
                                        <Input placeholder="1.1" />
                                      </Form.Item>
                                      <Form.Item
                                        label={t('pages.inbounds.form.responseStatus')}
                                        name={[
                                          'streamSettings', 'tcpSettings', 'header',
                                          'response', 'status',
                                        ]}
                                      >
                                        <Input placeholder="200" />
                                      </Form.Item>
                                      <Form.Item
                                        label={t('pages.inbounds.form.responseReason')}
                                        name={[
                                          'streamSettings', 'tcpSettings', 'header',
                                          'response', 'reason',
                                        ]}
                                      >
                                        <Input placeholder="OK" />
                                      </Form.Item>
                                      <Form.Item
                                        label={t('pages.inbounds.form.responseHeaders')}
                                        name={[
                                          'streamSettings', 'tcpSettings', 'header',
                                          'response', 'headers',
                                        ]}
                                      >
                                        <HeaderMapEditor mode="v2" />
                                      </Form.Item>
                                    </>
                                  )}
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
                            <Form.Item label={t('pages.inbounds.form.ttiMs')} name={['streamSettings', 'kcpSettings', 'tti']}>
                              <InputNumber min={0} />
                            </Form.Item>
                            <Form.Item
                              label={t('pages.inbounds.form.uplinkMbps')}
                              name={['streamSettings', 'kcpSettings', 'uplinkCapacity']}
                            >
                              <InputNumber min={0} />
                            </Form.Item>
                            <Form.Item
                              label={t('pages.inbounds.form.downlinkMbps')}
                              name={['streamSettings', 'kcpSettings', 'downlinkCapacity']}
                            >
                              <InputNumber min={0} />
                            </Form.Item>
                            <Form.Item
                              label={t('pages.inbounds.form.cwndMultiplier')}
                              name={['streamSettings', 'kcpSettings', 'cwndMultiplier']}
                            >
                              <InputNumber min={1} />
                            </Form.Item>
                            <Form.Item
                              label={t('pages.inbounds.form.maxSendingWindow')}
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
                              label={t('pages.inbounds.form.heartbeatPeriod')}
                              name={['streamSettings', 'wsSettings', 'heartbeatPeriod']}
                            >
                              <InputNumber min={0} />
                            </Form.Item>
                            <Form.Item
                              label={t('pages.inbounds.form.headers')}
                              name={['streamSettings', 'wsSettings', 'headers']}
                            >
                              <HeaderMapEditor mode="v1" />
                            </Form.Item>
                          </>
                        )}

                        {network === 'grpc' && (
                          <>
                            <Form.Item
                              label={t('pages.inbounds.form.serviceName')}
                              name={['streamSettings', 'grpcSettings', 'serviceName']}
                            >
                              <Input />
                            </Form.Item>
                            <Form.Item
                              label={t('pages.inbounds.form.authority')}
                              name={['streamSettings', 'grpcSettings', 'authority']}
                            >
                              <Input />
                            </Form.Item>
                            <Form.Item
                              label={t('pages.inbounds.form.multiMode')}
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
                            <Form.Item
                              label={t('pages.inbounds.form.headers')}
                              name={['streamSettings', 'httpupgradeSettings', 'headers']}
                            >
                              <HeaderMapEditor mode="v1" />
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
                              label={t('pages.inbounds.info.mode')}
                              name={['streamSettings', 'xhttpSettings', 'mode']}
                            >
                              <Select options={MODE_OPTIONS} />
                            </Form.Item>
                            <Form.Item
                              label={t('pages.inbounds.form.paddingBytes')}
                              name={['streamSettings', 'xhttpSettings', 'xPaddingBytes']}
                            >
                              <Input />
                            </Form.Item>
                            <Form.Item
                              label={t('pages.inbounds.form.headers')}
                              name={['streamSettings', 'xhttpSettings', 'headers']}
                            >
                              <HeaderMapEditor mode="v1" />
                            </Form.Item>

                            {/* Padding obfs sub-section: gated by a Switch.
                                When on, four extra knobs (key/header/placement/
                                method) tune how Xray injects random padding to
                                disguise the post body shape. */}
                            <Form.Item
                              label={t('pages.inbounds.form.paddingObfsMode')}
                              name={['streamSettings', 'xhttpSettings', 'xPaddingObfsMode']}
                              valuePropName="checked"
                            >
                              <Switch />
                            </Form.Item>
                            <Form.Item shouldUpdate noStyle>
                              {() => {
                                const obfs = !!form.getFieldValue([
                                  'streamSettings', 'xhttpSettings', 'xPaddingObfsMode',
                                ]);
                                if (!obfs) return null;
                                return (
                                  <>
                                    <Form.Item
                                      label={t('pages.inbounds.form.paddingKey')}
                                      name={['streamSettings', 'xhttpSettings', 'xPaddingKey']}
                                    >
                                      <Input placeholder="x_padding" />
                                    </Form.Item>
                                    <Form.Item
                                      label={t('pages.inbounds.form.paddingHeader')}
                                      name={['streamSettings', 'xhttpSettings', 'xPaddingHeader']}
                                    >
                                      <Input placeholder="X-Padding" />
                                    </Form.Item>
                                    <Form.Item
                                      label={t('pages.inbounds.form.paddingPlacement')}
                                      name={['streamSettings', 'xhttpSettings', 'xPaddingPlacement']}
                                    >
                                      <Select
                                        options={[
                                          { value: '', label: 'Default (queryInHeader)' },
                                          { value: 'queryInHeader', label: 'queryInHeader' },
                                          { value: 'header', label: 'header' },
                                          { value: 'cookie', label: 'cookie' },
                                          { value: 'query', label: 'query' },
                                        ]}
                                      />
                                    </Form.Item>
                                    <Form.Item
                                      label={t('pages.inbounds.form.paddingMethod')}
                                      name={['streamSettings', 'xhttpSettings', 'xPaddingMethod']}
                                    >
                                      <Select
                                        options={[
                                          { value: '', label: 'Default (repeat-x)' },
                                          { value: 'repeat-x', label: 'repeat-x' },
                                          { value: 'tokenish', label: 'tokenish' },
                                        ]}
                                      />
                                    </Form.Item>
                                  </>
                                );
                              }}
                            </Form.Item>

                            <Form.Item
                              noStyle
                              shouldUpdate={(prev, curr) =>
                                prev?.streamSettings?.xhttpSettings?.mode !==
                                curr?.streamSettings?.xhttpSettings?.mode
                              }
                            >
                              {() => {
                                const mode = form.getFieldValue([
                                  'streamSettings', 'xhttpSettings', 'mode',
                                ]);
                                return (
                                  <Form.Item
                                    label={t('pages.inbounds.form.uplinkHttpMethod')}
                                    name={['streamSettings', 'xhttpSettings', 'uplinkHTTPMethod']}
                                  >
                                    <Select
                                      placeholder="Default (POST)"
                                      options={[
                                        { value: '', label: 'Default (POST)' },
                                        { value: 'POST', label: 'POST' },
                                        { value: 'PUT', label: 'PUT' },
                                        { value: 'GET', label: 'GET (packet-up only)', disabled: mode !== 'packet-up' },
                                      ]}
                                    />
                                  </Form.Item>
                                );
                              }}
                            </Form.Item>

                            {/* Session + sequence + uplinkData placements:
                                three orthogonal slots Xray uses to thread
                                request metadata through the transport
                                (path / header / cookie / query). Key field
                                only matters when placement is not 'path'. */}
                            <Form.Item
                              label={t('pages.inbounds.form.sessionPlacement')}
                              name={['streamSettings', 'xhttpSettings', 'sessionPlacement']}
                            >
                              <Select
                                placeholder="Default (path)"
                                options={[
                                  { value: '', label: 'Default (path)' },
                                  { value: 'path', label: 'path' },
                                  { value: 'header', label: 'header' },
                                  { value: 'cookie', label: 'cookie' },
                                  { value: 'query', label: 'query' },
                                ]}
                              />
                            </Form.Item>
                            <Form.Item shouldUpdate noStyle>
                              {() => {
                                const placement = form.getFieldValue([
                                  'streamSettings', 'xhttpSettings', 'sessionPlacement',
                                ]);
                                if (!placement || placement === 'path') return null;
                                return (
                                  <Form.Item
                                    label={t('pages.inbounds.form.sessionKey')}
                                    name={['streamSettings', 'xhttpSettings', 'sessionKey']}
                                  >
                                    <Input placeholder="x_session" />
                                  </Form.Item>
                                );
                              }}
                            </Form.Item>
                            <Form.Item
                              label={t('pages.inbounds.form.sequencePlacement')}
                              name={['streamSettings', 'xhttpSettings', 'seqPlacement']}
                            >
                              <Select
                                placeholder="Default (path)"
                                options={[
                                  { value: '', label: 'Default (path)' },
                                  { value: 'path', label: 'path' },
                                  { value: 'header', label: 'header' },
                                  { value: 'cookie', label: 'cookie' },
                                  { value: 'query', label: 'query' },
                                ]}
                              />
                            </Form.Item>
                            <Form.Item shouldUpdate noStyle>
                              {() => {
                                const placement = form.getFieldValue([
                                  'streamSettings', 'xhttpSettings', 'seqPlacement',
                                ]);
                                if (!placement || placement === 'path') return null;
                                return (
                                  <Form.Item
                                    label={t('pages.inbounds.form.sequenceKey')}
                                    name={['streamSettings', 'xhttpSettings', 'seqKey']}
                                  >
                                    <Input placeholder="x_seq" />
                                  </Form.Item>
                                );
                              }}
                            </Form.Item>

                            {/* Mode-conditional sub-sections. */}
                            <Form.Item shouldUpdate noStyle>
                              {() => {
                                const mode = form.getFieldValue([
                                  'streamSettings', 'xhttpSettings', 'mode',
                                ]);
                                if (mode !== 'packet-up') return null;
                                return (
                                  <>
                                    <Form.Item
                                      label={t('pages.xray.outboundForm.minUploadInterval')}
                                      name={['streamSettings', 'xhttpSettings', 'scMinPostsIntervalMs']}
                                    >
                                      <Input placeholder="30" />
                                    </Form.Item>
                                    <Form.Item
                                      label={t('pages.xray.outboundForm.maxUploadSizeBytes')}
                                      name={['streamSettings', 'xhttpSettings', 'scMaxEachPostBytes']}
                                    >
                                      <Input placeholder="1000000" />
                                    </Form.Item>
                                    <Form.Item
                                      label={t('pages.inbounds.form.uplinkDataPlacement')}
                                      name={['streamSettings', 'xhttpSettings', 'uplinkDataPlacement']}
                                    >
                                      <Select
                                        options={[
                                          { value: '', label: 'Default (body)' },
                                          { value: 'body', label: 'body' },
                                          { value: 'header', label: 'header' },
                                          { value: 'cookie', label: 'cookie' },
                                          { value: 'query', label: 'query' },
                                        ]}
                                      />
                                    </Form.Item>
                                    <Form.Item shouldUpdate noStyle>
                                      {() => {
                                        const place = form.getFieldValue([
                                          'streamSettings', 'xhttpSettings', 'uplinkDataPlacement',
                                        ]);
                                        if (!place || place === 'body') return null;
                                        return (
                                          <>
                                            <Form.Item
                                              label={t('pages.inbounds.form.uplinkDataKey')}
                                              name={['streamSettings', 'xhttpSettings', 'uplinkDataKey']}
                                            >
                                              <Input placeholder="x_data" />
                                            </Form.Item>
                                            <Form.Item
                                              label={t('pages.xray.outboundForm.uplinkChunkSize')}
                                              name={['streamSettings', 'xhttpSettings', 'uplinkChunkSize']}
                                            >
                                              <InputNumber
                                                min={0}
                                                placeholder="0 (unlimited)"
                                                style={{ width: '100%' }}
                                              />
                                            </Form.Item>
                                          </>
                                        );
                                      }}
                                    </Form.Item>
                                  </>
                                );
                              }}
                            </Form.Item>
                            <Form.Item shouldUpdate noStyle>
                              {() => {
                                const mode = form.getFieldValue([
                                  'streamSettings', 'xhttpSettings', 'mode',
                                ]);
                                if (mode !== 'stream-up' && mode !== 'stream-one') return null;
                                return (
                                  <Form.Item
                                    label={t('pages.xray.outboundForm.noGrpcHeader')}
                                    name={['streamSettings', 'xhttpSettings', 'noGRPCHeader']}
                                    valuePropName="checked"
                                  >
                                    <Switch />
                                  </Form.Item>
                                );
                              }}
                            </Form.Item>

                            {/* XMUX is the connection-multiplexing layer
                                xHTTP uses to fan out parallel requests over
                                a small pool of upstream connections. UI-only
                                toggle (enableXmux) hides the 6 nested knobs
                                when off. */}
                            <Form.Item
                              label="XMUX"
                              name={['streamSettings', 'xhttpSettings', 'enableXmux']}
                              valuePropName="checked"
                            >
                              <Switch onChange={onXmuxToggle} />
                            </Form.Item>
                            <Form.Item shouldUpdate noStyle>
                              {() => {
                                if (!form.getFieldValue([
                                  'streamSettings', 'xhttpSettings', 'enableXmux',
                                ])) return null;
                                return (
                                  <>
                                    <Form.Item
                                      label={t('pages.xray.outboundForm.maxConcurrency')}
                                      name={['streamSettings', 'xhttpSettings', 'xmux', 'maxConcurrency']}
                                    >
                                      <Input placeholder="16-32" />
                                    </Form.Item>
                                    <Form.Item
                                      label={t('pages.xray.outboundForm.maxConnections')}
                                      name={['streamSettings', 'xhttpSettings', 'xmux', 'maxConnections']}
                                    >
                                      <Input placeholder="0" />
                                    </Form.Item>
                                    <Form.Item
                                      label={t('pages.xray.outboundForm.maxReuseTimes')}
                                      name={['streamSettings', 'xhttpSettings', 'xmux', 'cMaxReuseTimes']}
                                    >
                                      <Input />
                                    </Form.Item>
                                    <Form.Item
                                      label={t('pages.xray.outboundForm.maxRequestTimes')}
                                      name={['streamSettings', 'xhttpSettings', 'xmux', 'hMaxRequestTimes']}
                                    >
                                      <Input placeholder="600-900" />
                                    </Form.Item>
                                    <Form.Item
                                      label={t('pages.xray.outboundForm.maxReusableSecs')}
                                      name={['streamSettings', 'xhttpSettings', 'xmux', 'hMaxReusableSecs']}
                                    >
                                      <Input placeholder="1800-3000" />
                                    </Form.Item>
                                    <Form.Item
                                      label={t('pages.xray.outboundForm.keepAlivePeriod')}
                                      name={['streamSettings', 'xhttpSettings', 'xmux', 'hKeepAlivePeriod']}
                                    >
                                      <InputNumber min={0} style={{ width: '100%' }} />
                                    </Form.Item>
                                  </>
                                );
                              }}
                            </Form.Item>
                          </>
                        )}

                        {network === 'hysteria' && (
                          <>
                            <Form.Item
                              label={t('pages.inbounds.form.version')}
                              name={['streamSettings', 'hysteriaSettings', 'version']}
                            >
                              <InputNumber min={2} max={2} disabled style={{ width: '100%' }} />
                            </Form.Item>
                            <Form.Item
                              label={t('pages.xray.outboundForm.authPassword')}
                              name={['streamSettings', 'hysteriaSettings', 'auth']}
                            >
                              <Input />
                            </Form.Item>
                            <Form.Item
                              label={t('pages.inbounds.form.udpIdleTimeout')}
                              name={['streamSettings', 'hysteriaSettings', 'udpIdleTimeout']}
                            >
                              <InputNumber min={1} style={{ width: '100%' }} />
                            </Form.Item>
                            <HysteriaMasqueradeForm form={form} />
                          </>
                        )}
                      </>
                    )}

                    {tlsFlowAllowed && (
                      <Form.Item label={t('pages.clients.flow')} name={['settings', 'flow']}>
                        <Select
                          allowClear
                          placeholder={t('none')}
                          options={[{ value: '', label: t('none') }, ...FLOW_OPTIONS]}
                        />
                      </Form.Item>
                    )}

                    {/* Vision seed knobs only meaningful for the exact
                        xtls-rprx-vision flow, on TCP+(tls|reality). The
                        legacy class gated this on `canEnableVisionSeed()`
                        — same condition encoded inline here. */}
                    <Form.Item shouldUpdate noStyle>
                      {() => {
                        const flow =
                          (form.getFieldValue(['settings', 'flow']) ?? '') as string;
                        if (!(tlsFlowAllowed && flow === 'xtls-rprx-vision')) return null;
                        return (
                          <>
                            <Form.Item label={t('pages.xray.outboundForm.visionTestpre')} name={['settings', 'testpre']}>
                              <InputNumber min={0} style={{ width: '100%' }} />
                            </Form.Item>
                            <Form.Item label={t('pages.inbounds.form.visionTestseed')}>
                              <Space.Compact block>
                                {[900, 500, 900, 256].map((def, i) => (
                                  <Form.Item key={i} name={['settings', 'testseed', i]} noStyle initialValue={def}>
                                    <InputNumber min={1} style={{ width: '25%' }} />
                                  </Form.Item>
                                ))}
                              </Space.Compact>
                            </Form.Item>
                          </>
                        );
                      }}
                    </Form.Item>

                    {streamAllowed && network && (
                      <Form.Item label={t('security')}>
                        <Radio.Group
                          value={security}
                          buttonStyle="solid"
                          onChange={(e) => onSecurityChange(e.target.value as string)}
                        >
                          {network !== 'hysteria' && <Radio.Button value="none">{t('none')}</Radio.Button>}
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
                          <Input placeholder={t('pages.xray.outboundForm.serverNamePlaceholder')} />
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
                          label={t('pages.xray.outboundForm.verifyPeerName')}
                          name={['streamSettings', 'tlsSettings', 'verifyPeerCertByName']}
                        >
                          <Input placeholder="cloudflare-dns.com" />
                        </Form.Item>
                        <Form.Item
                          label={t('pages.xray.outboundForm.pinnedSha256')}
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
                          label={t('pages.xray.outboundForm.shortId')}
                          name={['streamSettings', 'realitySettings', 'shortId']}
                        >
                          <Input />
                        </Form.Item>
                        <Form.Item
                          label={t('pages.inbounds.form.spiderX')}
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
                          label={t('pages.inbounds.form.mldsa65Verify')}
                          name={['streamSettings', 'realitySettings', 'mldsa65Verify']}
                        >
                          <Input.TextArea autoSize={{ minRows: 2 }} />
                        </Form.Item>
                      </>
                    )}

                    {((streamAllowed && network) || !streamAllowed) && (
                      <Form.Item shouldUpdate noStyle>
                        {() => {
                          const hasSockopt = !!form.getFieldValue([
                            'streamSettings',
                            'sockopt',
                          ]);
                          return (
                            <>
                              <Form.Item label={t('pages.xray.outboundForm.sockopts')}>
                                <Switch
                                  checked={hasSockopt}
                                  onChange={(checked) => {
                                    form.setFieldValue(
                                      ['streamSettings', 'sockopt'],
                                      checked ? SockoptStreamSettingsSchema.parse({}) : undefined,
                                    );
                                  }}
                                />
                              </Form.Item>
                              {hasSockopt && (
                                <>
                                  <Form.Item
                                    label={t('pages.inbounds.form.dialerProxy')}
                                    name={['streamSettings', 'sockopt', 'dialerProxy']}
                                  >
                                    <Input />
                                  </Form.Item>
                                  <Form.Item
                                    label={t('pages.xray.wireguard.domainStrategy')}
                                    name={['streamSettings', 'sockopt', 'domainStrategy']}
                                  >
                                    <Select
                                      options={Object.values(DOMAIN_STRATEGY_OPTION).map((v) => ({
                                        value: v,
                                        label: v,
                                      }))}
                                    />
                                  </Form.Item>
                                  <Form.Item
                                    label={t('pages.inbounds.form.addressPortStrategy')}
                                    name={['streamSettings', 'sockopt', 'addressPortStrategy']}
                                  >
                                    <Select options={ADDRESS_PORT_STRATEGY_OPTIONS} />
                                  </Form.Item>
                                  <Form.Item
                                    label={t('pages.xray.outboundForm.keepAliveInterval')}
                                    name={['streamSettings', 'sockopt', 'tcpKeepAliveInterval']}
                                  >
                                    <InputNumber min={0} />
                                  </Form.Item>
                                  <Form.Item
                                    label={t('pages.inbounds.form.tcpFastOpen')}
                                    name={['streamSettings', 'sockopt', 'tcpFastOpen']}
                                    valuePropName="checked"
                                  >
                                    <Switch />
                                  </Form.Item>
                                  <Form.Item
                                    label={t('pages.inbounds.form.multipathTcp')}
                                    name={['streamSettings', 'sockopt', 'tcpMptcp']}
                                    valuePropName="checked"
                                  >
                                    <Switch />
                                  </Form.Item>
                                  <Form.Item
                                    label={t('pages.inbounds.form.penetrate')}
                                    name={['streamSettings', 'sockopt', 'penetrate']}
                                    valuePropName="checked"
                                  >
                                    <Switch />
                                  </Form.Item>
                                  <Form.Item
                                    label={t('pages.xray.outboundForm.markFwmark')}
                                    name={['streamSettings', 'sockopt', 'mark']}
                                  >
                                    <InputNumber min={0} />
                                  </Form.Item>
                                  <Form.Item
                                    label={t('pages.xray.outboundForm.interface')}
                                    name={['streamSettings', 'sockopt', 'interfaceName']}
                                  >
                                    <Input />
                                  </Form.Item>
                                  <Form.Item
                                    label="TProxy"
                                    name={['streamSettings', 'sockopt', 'tproxy']}
                                  >
                                    <Select
                                      options={[
                                        { value: 'off', label: 'off' },
                                        { value: 'redirect', label: 'redirect' },
                                        { value: 'tproxy', label: 'tproxy' },
                                      ]}
                                    />
                                  </Form.Item>
                                  <Form.Item
                                    label={t('pages.inbounds.form.tcpCongestion')}
                                    name={['streamSettings', 'sockopt', 'tcpcongestion']}
                                  >
                                    <Select
                                      options={Object.values(TCP_CONGESTION_OPTION).map((v) => ({
                                        value: v,
                                        label: v,
                                      }))}
                                    />
                                  </Form.Item>
                                  <Form.Item
                                    label={t('pages.xray.outboundForm.ipv6Only')}
                                    name={['streamSettings', 'sockopt', 'V6Only']}
                                    valuePropName="checked"
                                  >
                                    <Switch />
                                  </Form.Item>
                                  <Form.Item
                                    label={t('pages.xray.outboundForm.acceptProxyProtocol')}
                                    name={['streamSettings', 'sockopt', 'acceptProxyProtocol']}
                                    valuePropName="checked"
                                  >
                                    <Switch />
                                  </Form.Item>
                                  <Form.Item
                                    label={t('pages.xray.outboundForm.tcpUserTimeoutMs')}
                                    name={['streamSettings', 'sockopt', 'tcpUserTimeout']}
                                  >
                                    <InputNumber min={0} style={{ width: '100%' }} />
                                  </Form.Item>
                                  <Form.Item
                                    label={t('pages.xray.outboundForm.tcpKeepAliveIdleS')}
                                    name={['streamSettings', 'sockopt', 'tcpKeepAliveIdle']}
                                  >
                                    <InputNumber min={0} style={{ width: '100%' }} />
                                  </Form.Item>
                                  <Form.Item
                                    label={t('pages.inbounds.form.tcpMaxSeg')}
                                    name={['streamSettings', 'sockopt', 'tcpMaxSeg']}
                                  >
                                    <InputNumber min={0} style={{ width: '100%' }} />
                                  </Form.Item>
                                  <Form.Item
                                    label={t('pages.inbounds.form.tcpWindowClamp')}
                                    name={['streamSettings', 'sockopt', 'tcpWindowClamp']}
                                  >
                                    <InputNumber min={0} style={{ width: '100%' }} />
                                  </Form.Item>
                                  <Form.Item
                                    label={t('pages.inbounds.form.trustedXForwardedFor')}
                                    name={['streamSettings', 'sockopt', 'trustedXForwardedFor']}
                                  >
                                    <Select
                                      mode="tags"
                                      tokenSeparators={[',', ' ']}
                                      placeholder="trusted-proxy.example,10.0.0.0/8"
                                    />
                                  </Form.Item>
                                  <Form.Item shouldUpdate noStyle>
                                    {() => {
                                      const he = form.getFieldValue([
                                        'streamSettings', 'sockopt', 'happyEyeballs',
                                      ]);
                                      const hasHe = he != null;
                                      return (
                                        <>
                                          <Form.Item label="Happy Eyeballs">
                                            <Switch
                                              checked={hasHe}
                                              onChange={(v) => {
                                                form.setFieldValue(
                                                  ['streamSettings', 'sockopt', 'happyEyeballs'],
                                                  v ? HappyEyeballsSchema.parse({}) : undefined,
                                                );
                                              }}
                                            />
                                          </Form.Item>
                                          {hasHe && (
                                            <>
                                              <Form.Item
                                                label={t('pages.inbounds.form.tryDelayMs')}
                                                name={['streamSettings', 'sockopt', 'happyEyeballs', 'tryDelayMs']}
                                              >
                                                <InputNumber min={0} style={{ width: '100%' }} placeholder="0 (disabled) — 250 recommended" />
                                              </Form.Item>
                                              <Form.Item
                                                label={t('pages.inbounds.form.prioritizeIPv6')}
                                                name={['streamSettings', 'sockopt', 'happyEyeballs', 'prioritizeIPv6']}
                                                valuePropName="checked"
                                              >
                                                <Switch />
                                              </Form.Item>
                                              <Form.Item
                                                label={t('pages.inbounds.form.interleave')}
                                                name={['streamSettings', 'sockopt', 'happyEyeballs', 'interleave']}
                                              >
                                                <InputNumber min={1} style={{ width: '100%' }} />
                                              </Form.Item>
                                              <Form.Item
                                                label={t('pages.inbounds.form.maxConcurrentTry')}
                                                name={['streamSettings', 'sockopt', 'happyEyeballs', 'maxConcurrentTry']}
                                              >
                                                <InputNumber min={0} style={{ width: '100%' }} />
                                              </Form.Item>
                                            </>
                                          )}
                                        </>
                                      );
                                    }}
                                  </Form.Item>
                                  <Form.List name={['streamSettings', 'sockopt', 'customSockopt']}>
                                    {(fields, { add, remove }) => (
                                      <>
                                        <Form.Item label={t('pages.inbounds.form.customSockopt')}>
                                          <Button
                                            type="dashed"
                                            size="small"
                                            onClick={() => add({ type: 'int', level: '6', opt: '', value: '' })}
                                          >
                                            + {t('pages.inbounds.form.addCustomOption')}
                                          </Button>
                                        </Form.Item>
                                        {fields.map((field) => (
                                          <Space.Compact key={field.key} style={{ display: 'flex', marginBottom: 8 }}>
                                            <Form.Item name={[field.name, 'system']} noStyle>
                                              <Select
                                                placeholder="all"
                                                allowClear
                                                style={{ width: 100 }}
                                                options={[
                                                  { value: 'linux', label: 'linux' },
                                                  { value: 'windows', label: 'windows' },
                                                  { value: 'darwin', label: 'darwin' },
                                                ]}
                                              />
                                            </Form.Item>
                                            <Form.Item name={[field.name, 'type']} noStyle>
                                              <Select
                                                style={{ width: 80 }}
                                                options={[
                                                  { value: 'int', label: 'int' },
                                                  { value: 'str', label: 'str' },
                                                ]}
                                              />
                                            </Form.Item>
                                            <Form.Item name={[field.name, 'level']} noStyle>
                                              <Input placeholder="level (6=TCP)" style={{ width: 100 }} />
                                            </Form.Item>
                                            <Form.Item name={[field.name, 'opt']} noStyle>
                                              <Input placeholder="opt (decimal)" style={{ width: 120 }} />
                                            </Form.Item>
                                            <Form.Item name={[field.name, 'value']} noStyle>
                                              <Input placeholder="value" style={{ flex: 1 }} />
                                            </Form.Item>
                                            <Button danger onClick={() => remove(field.name)}>−</Button>
                                          </Space.Compact>
                                        ))}
                                      </>
                                    )}
                                  </Form.List>
                                </>
                              )}
                            </>
                          );
                        }}
                      </Form.Item>
                    )}

                    <FinalMaskForm
                      name={['streamSettings', 'finalmask']}
                      network={network}
                      protocol={protocol}
                      form={form}
                    />

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
                                      label={t('pages.settings.subFormats.concurrency')}
                                      name={['mux', 'concurrency']}
                                    >
                                      <InputNumber min={-1} max={1024} />
                                    </Form.Item>
                                    <Form.Item
                                      label={t('pages.settings.subFormats.xudpConcurrency')}
                                      name={['mux', 'xudpConcurrency']}
                                    >
                                      <InputNumber min={-1} max={1024} />
                                    </Form.Item>
                                    <Form.Item
                                      label={t('pages.settings.subFormats.xudpUdp443')}
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
                    <Input.Search
                      value={linkInput}
                      placeholder="vmess:// vless:// trojan:// ss:// hysteria2:// wireguard://"
                      enterButton="Import"
                      onChange={(e) => setLinkInput(e.target.value)}
                      onSearch={importLink}
                    />
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
