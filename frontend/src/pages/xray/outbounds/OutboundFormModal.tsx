import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
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
import { FinalMaskForm } from '@/lib/xray/forms/transport';
import { JsonEditor } from '@/components/form';
import { Wireguard } from '@/utils';
import {
  XMUX_DEFAULTS,
  formValuesToWirePayload,
  rawOutboundToFormValues,
} from '@/lib/xray/outbound-form-adapter';
import { parseOutboundLink } from '@/lib/xray/outbound-link-parser';
import {
  OutboundFormBaseSchema,
  type OutboundFormValues,
} from '@/schemas/forms/outbound-form';
import { SNIFFING_OPTION } from '@/schemas/primitives';
import {
  canEnableReality,
  canEnableStream,
  canEnableTls,
  canEnableTlsFlow,
} from '@/lib/xray/protocol-capabilities';
import { antdRule } from '@/utils/zodForm';

import {
  FLOW_OPTIONS,
  HYSTERIA_NETWORK_OPTION,
  NETWORK_OPTIONS,
  PROTOCOL_OPTIONS,
  SERVER_PROTOCOLS,
} from './outbound-form-constants';
import {
  buildAddModeValues,
  hysteriaStreamSlice,
  newStreamSlice,
} from './outbound-form-helpers';
import {
  BlackholeFields,
  DnsFields,
  FreedomFields,
  HttpFields,
  LoopbackFields,
  ServerTarget,
  ShadowsocksFields,
  SocksFields,
  TrojanFields,
  VlessFields,
  VmessFields,
  WireguardFields,
} from './protocols';
import {
  GrpcForm,
  HttpUpgradeForm,
  HysteriaForm,
  KcpForm,
  MuxForm,
  RawForm,
  SockoptForm,
  WsForm,
  XhttpForm,
} from './transport';
import { RealityForm, TlsForm } from './security';
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

                    {SERVER_PROTOCOLS.has(protocol) && <ServerTarget />}
                    {protocol === 'vmess' && <VmessFields />}
                    {protocol === 'vless' && <VlessFields />}
                    {protocol === 'trojan' && <TrojanFields />}
                    {protocol === 'shadowsocks' && <ShadowsocksFields />}
                    {protocol === 'http' && <HttpFields />}
                    {protocol === 'socks' && <SocksFields />}

                    {protocol === 'loopback' && <LoopbackFields />}
                    {protocol === 'blackhole' && <BlackholeFields />}
                    {protocol === 'dns' && <DnsFields />}

                    {protocol === 'freedom' && <FreedomFields form={form} />}

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

                    {protocol === 'wireguard' && <WireguardFields form={form} />}

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

                        {network === 'tcp' && <RawForm form={form} />}

                        {network === 'kcp' && <KcpForm />}

                        {network === 'ws' && <WsForm />}

                        {network === 'grpc' && <GrpcForm />}

                        {network === 'httpupgrade' && <HttpUpgradeForm />}

                        {network === 'xhttp' && <XhttpForm form={form} onXmuxToggle={onXmuxToggle} />}

                        {network === 'hysteria' && <HysteriaForm form={form} />}
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

                    {security === 'tls' && tlsAllowed && <TlsForm />}

                    {security === 'reality' && realityAllowed && <RealityForm />}

                    {((streamAllowed && network) || !streamAllowed) && <SockoptForm form={form} />}

                    <FinalMaskForm
                      name={['streamSettings', 'finalmask']}
                      network={network}
                      protocol={protocol}
                      form={form}
                    />

                    <MuxForm form={form} protocol={protocol} network={network} />
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
