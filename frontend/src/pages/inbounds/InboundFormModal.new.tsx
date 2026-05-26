import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import {
  Button,
  Checkbox,
  Form,
  Input,
  InputNumber,
  Modal,
  Radio,
  Select,
  Space,
  Switch,
  Tabs,
  Tooltip,
  Typography,
  message,
} from 'antd';
import { MinusOutlined, PlusOutlined, SyncOutlined } from '@ant-design/icons';

import { HttpUtil, NumberFormatter, RandomUtil, SizeFormatter, Wireguard } from '@/utils';
import {
  rawInboundToFormValues,
  formValuesToWirePayload,
} from '@/lib/xray/inbound-form-adapter';
import { createDefaultInboundSettings } from '@/lib/xray/inbound-defaults';
import {
  canEnableReality,
  canEnableStream,
  canEnableTls,
  isSS2022,
} from '@/lib/xray/protocol-capabilities';
import { SSMethodSchema } from '@/schemas/protocols/inbound/shadowsocks';
import { getRandomRealityTarget } from '@/models/reality-targets';
import {
  InboundFormBaseSchema,
  InboundFormSchema,
  type InboundFormValues,
} from '@/schemas/forms/inbound-form';
import { antdRule } from '@/utils/zodForm';
import {
  ALPN_OPTION,
  DOMAIN_STRATEGY_OPTION,
  Protocols,
  SNIFFING_OPTION,
  TCP_CONGESTION_OPTION,
  TLS_CIPHER_OPTION,
  TLS_VERSION_OPTION,
  USAGE_OPTION,
  UTLS_FINGERPRINT,
} from '@/schemas/primitives';
import { SockoptStreamSettingsSchema } from '@/schemas/protocols/stream/sockopt';
import { TlsStreamSettingsSchema } from '@/schemas/protocols/security/tls';
import { RealityStreamSettingsSchema } from '@/schemas/protocols/security/reality';
import DateTimePicker from '@/components/DateTimePicker';
import InputAddon from '@/components/InputAddon';

const { TextArea } = Input;
import type { DBInbound } from '@/models/dbinbound';
import type { NodeRecord } from '@/api/queries/useNodesQuery';

// Pattern A rewrite of InboundFormModal. Built as a sibling file so the
// build stays green while the rewrite progresses section by section.
// InboundsPage continues to render the old InboundFormModal.tsx until the
// atomic swap at the end (Core Decision 7).

const { Text } = Typography;

const PROTOCOL_OPTIONS = Object.values(Protocols).map((p) => ({ value: p, label: p }));
const TRAFFIC_RESETS = ['never', 'hourly', 'daily', 'weekly', 'monthly'] as const;
const NODE_ELIGIBLE_PROTOCOLS = new Set<string>([
  Protocols.VLESS,
  Protocols.VMESS,
  Protocols.TROJAN,
  Protocols.SHADOWSOCKS,
  Protocols.HYSTERIA,
  Protocols.WIREGUARD,
]);

interface InboundFormModalProps {
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
  mode: 'add' | 'edit';
  dbInbound: DBInbound | null;
  dbInbounds: DBInbound[];
  availableNodes?: NodeRecord[];
}

function buildAddModeValues(): InboundFormValues {
  const settings = createDefaultInboundSettings('vless') ?? undefined;
  return rawInboundToFormValues({
    protocol: 'vless',
    settings,
    streamSettings: { network: 'tcp', security: 'none' },
    sniffing: {},
    port: RandomUtil.randomInteger(10000, 60000),
    listen: '',
    tag: '',
    enable: true,
    trafficReset: 'never',
  });
}

export default function InboundFormModalNew({
  open,
  onClose,
  onSaved,
  mode,
  dbInbound,
  availableNodes,
}: InboundFormModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [form] = Form.useForm<InboundFormValues>();
  const [saving, setSaving] = useState(false);

  const selectableNodes = (availableNodes || []).filter((n) => n.enable);
  const protocol = (Form.useWatch('protocol', form) ?? '') as string;
  const isNodeEligible = NODE_ELIGIBLE_PROTOCOLS.has(protocol);
  const sniffingEnabled = Form.useWatch(['sniffing', 'enabled'], form) ?? false;
  const vlessEncryption = Form.useWatch(['settings', 'encryption'], form) ?? '';
  const ssMethod = Form.useWatch(['settings', 'method'], form);
  const isSSWith2022 = isSS2022({
    protocol,
    settings: typeof ssMethod === 'string' ? { method: ssMethod } : {},
  });
  const mixedUdpOn = Form.useWatch(['settings', 'udp'], form) ?? false;
  const network = Form.useWatch(['streamSettings', 'network'], form) ?? '';
  const security = Form.useWatch(['streamSettings', 'security'], form) ?? 'none';
  const streamEnabled = canEnableStream({ protocol });
  const tlsAllowed = canEnableTls({ protocol, streamSettings: { network, security } });
  const realityAllowed = canEnableReality({ protocol, streamSettings: { network, security } });

  const genRealityKeypair = async () => {
    setSaving(true);
    try {
      const msg = await HttpUtil.get('/panel/api/server/getNewX25519Cert');
      if (msg?.success) {
        const obj = msg.obj as { privateKey: string; publicKey: string };
        form.setFieldValue(['streamSettings', 'realitySettings', 'privateKey'], obj.privateKey);
        form.setFieldValue(['streamSettings', 'realitySettings', 'settings', 'publicKey'], obj.publicKey);
      }
    } finally {
      setSaving(false);
    }
  };

  const clearRealityKeypair = () => {
    form.setFieldValue(['streamSettings', 'realitySettings', 'privateKey'], '');
    form.setFieldValue(['streamSettings', 'realitySettings', 'settings', 'publicKey'], '');
  };

  const genMldsa65 = async () => {
    setSaving(true);
    try {
      const msg = await HttpUtil.get('/panel/api/server/getNewmldsa65');
      if (msg?.success) {
        const obj = msg.obj as { seed: string; verify: string };
        form.setFieldValue(['streamSettings', 'realitySettings', 'mldsa65Seed'], obj.seed);
        form.setFieldValue(['streamSettings', 'realitySettings', 'settings', 'mldsa65Verify'], obj.verify);
      }
    } finally {
      setSaving(false);
    }
  };

  const clearMldsa65 = () => {
    form.setFieldValue(['streamSettings', 'realitySettings', 'mldsa65Seed'], '');
    form.setFieldValue(['streamSettings', 'realitySettings', 'settings', 'mldsa65Verify'], '');
  };

  const randomizeRealityTarget = () => {
    const tgt = getRandomRealityTarget() as { target: string; sni: string };
    form.setFieldValue(['streamSettings', 'realitySettings', 'target'], tgt.target);
    form.setFieldValue(
      ['streamSettings', 'realitySettings', 'serverNames'],
      tgt.sni.split(',').map((s) => s.trim()).filter(Boolean),
    );
  };

  const randomizeShortIds = () => {
    form.setFieldValue(
      ['streamSettings', 'realitySettings', 'shortIds'],
      RandomUtil.randomShortIds().split(',').map((s) => s.trim()).filter(Boolean),
    );
  };

  const getNewEchCert = async () => {
    const sni = form.getFieldValue(['streamSettings', 'tlsSettings', 'serverName']);
    setSaving(true);
    try {
      const msg = await HttpUtil.post('/panel/api/server/getNewEchCert', { sni });
      if (msg?.success) {
        const obj = msg.obj as { echServerKeys: string; echConfigList: string };
        form.setFieldValue(['streamSettings', 'tlsSettings', 'echServerKeys'], obj.echServerKeys);
        form.setFieldValue(['streamSettings', 'tlsSettings', 'settings', 'echConfigList'], obj.echConfigList);
      }
    } finally {
      setSaving(false);
    }
  };

  const clearEchCert = () => {
    form.setFieldValue(['streamSettings', 'tlsSettings', 'echServerKeys'], '');
    form.setFieldValue(['streamSettings', 'tlsSettings', 'settings', 'echConfigList'], '');
  };

  const onSecurityChange = (next: string) => {
    const current = (form.getFieldValue('streamSettings') as Record<string, unknown>) ?? {};
    const cleaned: Record<string, unknown> = { ...current, security: next };
    delete cleaned.tlsSettings;
    delete cleaned.realitySettings;
    if (next === 'tls') cleaned.tlsSettings = TlsStreamSettingsSchema.parse({});
    if (next === 'reality') cleaned.realitySettings = RealityStreamSettingsSchema.parse({});
    form.setFieldValue('streamSettings', cleaned);
  };
  const xhttpMode = Form.useWatch(['streamSettings', 'xhttpSettings', 'mode'], form);
  const xhttpObfsMode = Form.useWatch(['streamSettings', 'xhttpSettings', 'xPaddingObfsMode'], form) ?? false;
  const xhttpSessionPlacement = Form.useWatch(['streamSettings', 'xhttpSettings', 'sessionPlacement'], form);
  const xhttpSeqPlacement = Form.useWatch(['streamSettings', 'xhttpSettings', 'seqPlacement'], form);
  const xhttpUplinkPlacement = Form.useWatch(['streamSettings', 'xhttpSettings', 'uplinkDataPlacement'], form);
  const externalProxyArr = Form.useWatch(['streamSettings', 'externalProxy'], form);
  const externalProxyOn = Array.isArray(externalProxyArr) && externalProxyArr.length > 0;
  const sockoptValue = Form.useWatch(['streamSettings', 'sockopt'], form);
  const sockoptOn = !!sockoptValue && typeof sockoptValue === 'object' && Object.keys(sockoptValue as object).length > 0;

  const toggleExternalProxy = (on: boolean) => {
    if (on) {
      const port = (form.getFieldValue('port') as number) ?? 443;
      form.setFieldValue(['streamSettings', 'externalProxy'], [{
        forceTls: 'same',
        dest: typeof window !== 'undefined' ? window.location.hostname : '',
        port,
        remark: '',
        sni: '',
        fingerprint: '',
        alpn: [],
      }]);
    } else {
      form.setFieldValue(['streamSettings', 'externalProxy'], []);
    }
  };

  const toggleSockopt = (on: boolean) => {
    if (on) {
      form.setFieldValue(
        ['streamSettings', 'sockopt'],
        SockoptStreamSettingsSchema.parse({}),
      );
    } else {
      form.setFieldValue(['streamSettings', 'sockopt'], undefined);
    }
  };
  const wgSecretKey = Form.useWatch(['settings', 'secretKey'], form);
  const wgPubKey = typeof wgSecretKey === 'string' && wgSecretKey.length > 0
    ? Wireguard.generateKeypair(wgSecretKey).publicKey
    : '';

  const regenInboundWg = () => {
    const kp = Wireguard.generateKeypair();
    form.setFieldValue(['settings', 'secretKey'], kp.privateKey);
  };

  const regenWgPeerKeypair = (peerName: number) => {
    const kp = Wireguard.generateKeypair();
    form.setFieldValue(['settings', 'peers', peerName, 'privateKey'], kp.privateKey);
    form.setFieldValue(['settings', 'peers', peerName, 'publicKey'], kp.publicKey);
  };

  const matchesVlessAuth = (
    block: { id?: string; label?: string } | undefined | null,
    authId: string,
  ) => {
    if (block?.id === authId) return true;
    const label = (block?.label || '').toLowerCase().replace(/[-_\s]/g, '');
    if (authId === 'mlkem768') return label.includes('mlkem768');
    if (authId === 'x25519') return label.includes('x25519');
    return false;
  };

  const getNewVlessEnc = async (authId: string) => {
    if (!authId) return;
    setSaving(true);
    try {
      const msg = await HttpUtil.get('/panel/api/server/getNewVlessEnc');
      if (!msg?.success) return;
      const obj = msg.obj as {
        auths?: { decryption: string; encryption: string; label?: string; id?: string }[];
      };
      const block = (obj.auths || []).find((a) => matchesVlessAuth(a, authId));
      if (!block) return;
      form.setFieldValue(['settings', 'decryption'], block.decryption);
      form.setFieldValue(['settings', 'encryption'], block.encryption);
    } finally {
      setSaving(false);
    }
  };

  const clearVlessEnc = () => {
    form.setFieldValue(['settings', 'decryption'], 'none');
    form.setFieldValue(['settings', 'encryption'], 'none');
  };

  const selectedVlessAuth = (() => {
    const enc = typeof vlessEncryption === 'string' ? vlessEncryption : '';
    if (!enc || enc === 'none') return 'None';
    const parts = enc.split('.').filter(Boolean);
    const authKey = parts[parts.length - 1] || '';
    if (!authKey) return t('pages.inbounds.vlessAuthCustom');
    return authKey.length > 300
      ? t('pages.inbounds.vlessAuthMlkem768')
      : t('pages.inbounds.vlessAuthX25519');
  })();

  useEffect(() => {
    if (!open) return;
    const initial = mode === 'edit' && dbInbound
      ? rawInboundToFormValues(dbInbound)
      : buildAddModeValues();
    form.resetFields();
    form.setFieldsValue(initial);
  }, [open, mode, dbInbound, form]);

  // Why: protocol picker reset cascades through the form — clearing the
  // settings DU branch and dropping a nodeId that no longer applies. The
  // legacy modal did this imperatively in onProtocolChange; here we hook
  // into AntD's onValuesChange and let setFieldValue keep the rest of
  // the form state intact.
  const onValuesChange = (changed: Partial<InboundFormValues>) => {
    if (mode === 'edit') return;
    if ('protocol' in changed && typeof changed.protocol === 'string') {
      const next = changed.protocol;
      const settings = createDefaultInboundSettings(next) ?? undefined;
      form.setFieldValue('settings', settings);
      if (!NODE_ELIGIBLE_PROTOCOLS.has(next)) {
        form.setFieldValue('nodeId', null);
      }
    }
  };

  const submit = async () => {
    let values: InboundFormValues;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    const parsed = InboundFormSchema.safeParse(values);
    if (!parsed.success) {
      const issue = parsed.error.issues[0];
      messageApi.error(
        t(issue?.message ?? 'somethingWentWrong', {
          defaultValue: issue?.message ?? 'invalid',
        }),
      );
      return;
    }
    setSaving(true);
    try {
      const payload = formValuesToWirePayload(parsed.data);
      const url = mode === 'edit' && dbInbound
        ? `/panel/api/inbounds/update/${dbInbound.id}`
        : '/panel/api/inbounds/add';
      const msg = await HttpUtil.post(url, payload);
      if (msg?.success) {
        onSaved();
        onClose();
      }
    } finally {
      setSaving(false);
    }
  };

  const title = mode === 'edit'
    ? t('pages.inbounds.modifyInbound')
    : t('pages.inbounds.addInbound');

  const okText = mode === 'edit'
    ? t('pages.clients.submitEdit')
    : t('create');

  const basicTab = (
    <>
      <Form.Item name="enable" label={t('enable')} valuePropName="checked">
        <Switch />
      </Form.Item>

      <Form.Item name="remark" label={t('pages.inbounds.remark')}>
        <Input />
      </Form.Item>

      {selectableNodes.length > 0 && isNodeEligible && (
        <Form.Item name="nodeId" label={t('pages.inbounds.deployTo')}>
          <Select
            disabled={mode === 'edit'}
            placeholder={t('pages.inbounds.localPanel')}
            allowClear
          >
            <Select.Option value={null}>{t('pages.inbounds.localPanel')}</Select.Option>
            {selectableNodes.map((n) => (
              <Select.Option
                key={n.id}
                value={n.id}
                disabled={n.status === 'offline'}
              >
                {n.name}{n.status === 'offline' ? ' (offline)' : ''}
              </Select.Option>
            ))}
          </Select>
        </Form.Item>
      )}

      <Form.Item name="protocol" label={t('pages.inbounds.protocol')}>
        <Select disabled={mode === 'edit'} options={PROTOCOL_OPTIONS} />
      </Form.Item>

      <Form.Item name="listen" label={t('pages.inbounds.address')}>
        <Input placeholder={t('pages.inbounds.monitorDesc')} />
      </Form.Item>

      <Form.Item
        name="port"
        label={t('pages.inbounds.port')}
        rules={[antdRule(InboundFormBaseSchema.shape.port, t)]}
      >
        <InputNumber min={1} max={65535} />
      </Form.Item>

      <Form.Item
        label={
          <Tooltip title={t('pages.inbounds.meansNoLimit')}>
            {t('pages.inbounds.totalFlow')}
          </Tooltip>
        }
      >
        <Form.Item
          noStyle
          shouldUpdate={(prev, curr) => prev.total !== curr.total}
        >
          {({ getFieldValue, setFieldValue }) => {
            const totalBytes = (getFieldValue('total') as number) ?? 0;
            const totalGB = totalBytes
              ? Math.round((totalBytes / SizeFormatter.ONE_GB) * 100) / 100
              : 0;
            return (
              <InputNumber
                value={totalGB}
                min={0}
                step={1}
                onChange={(v) => {
                  const bytes = NumberFormatter.toFixed(
                    (Number(v) || 0) * SizeFormatter.ONE_GB,
                    0,
                  );
                  setFieldValue('total', bytes);
                }}
              />
            );
          }}
        </Form.Item>
      </Form.Item>

      <Form.Item name="trafficReset" label={t('pages.inbounds.periodicTrafficResetTitle')}>
        <Select>
          {TRAFFIC_RESETS.map((r) => (
            <Select.Option key={r} value={r}>
              {t(`pages.inbounds.periodicTrafficReset.${r}`)}
            </Select.Option>
          ))}
        </Select>
      </Form.Item>

      <Form.Item
        label={
          <Tooltip title={t('pages.inbounds.leaveBlankToNeverExpire')}>
            {t('pages.inbounds.expireDate')}
          </Tooltip>
        }
      >
        <Form.Item
          noStyle
          shouldUpdate={(prev, curr) => prev.expiryTime !== curr.expiryTime}
        >
          {({ getFieldValue, setFieldValue }) => {
            const expiry = (getFieldValue('expiryTime') as number) ?? 0;
            return (
              <DateTimePicker
                value={expiry > 0 ? dayjs(expiry) : null}
                onChange={(d) => setFieldValue('expiryTime', d ? d.valueOf() : 0)}
              />
            );
          }}
        </Form.Item>
      </Form.Item>
    </>
  );

  const protocolTab = (
    <>
      {protocol === Protocols.WIREGUARD && (
        <>
          <Form.Item
            name={['settings', 'secretKey']}
            label={
              <>
                Secret key{' '}
                <SyncOutlined className="random-icon" onClick={regenInboundWg} />
              </>
            }
          >
            <Input />
          </Form.Item>
          <Form.Item label="Public key">
            <Input value={wgPubKey} disabled />
          </Form.Item>
          <Form.Item name={['settings', 'mtu']} label="MTU">
            <InputNumber />
          </Form.Item>
          <Form.Item
            name={['settings', 'noKernelTun']}
            label="No-kernel TUN"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.List name={['settings', 'peers']}>
            {(fields, { add, remove }) => (
              <>
                <Form.Item label="Peers">
                  <Button
                    size="small"
                    onClick={() => add({
                      publicKey: '',
                      allowedIPs: [],
                    })}
                  >
                    <PlusOutlined /> Add peer
                  </Button>
                </Form.Item>
                {fields.map((field, idx) => (
                  <div key={field.key} className="wg-peer">
                    <Form.Item label={`Peer ${idx + 1}`}>
                      {fields.length > 1 && (
                        <Button
                          size="small"
                          danger
                          onClick={() => remove(field.name)}
                        >
                          <MinusOutlined />
                        </Button>
                      )}
                    </Form.Item>
                    <Form.Item
                      name={[field.name, 'privateKey']}
                      label={
                        <>
                          Secret key{' '}
                          <SyncOutlined
                            className="random-icon"
                            onClick={() => regenWgPeerKeypair(field.name)}
                          />
                        </>
                      }
                    >
                      <Input />
                    </Form.Item>
                    <Form.Item name={[field.name, 'publicKey']} label="Public key">
                      <Input />
                    </Form.Item>
                    <Form.Item name={[field.name, 'preSharedKey']} label="PSK">
                      <Input />
                    </Form.Item>
                    <Form.List name={[field.name, 'allowedIPs']}>
                      {(ipFields, { add: addIp, remove: removeIp }) => (
                        <Form.Item label="Allowed IPs">
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
                    <Form.Item name={[field.name, 'keepAlive']} label="Keep-alive">
                      <InputNumber min={0} />
                    </Form.Item>
                  </div>
                ))}
              </>
            )}
          </Form.List>
        </>
      )}

      {protocol === Protocols.TUN && (
        <>
          <Form.Item name={['settings', 'name']} label="Interface name">
            <Input placeholder="xray0" />
          </Form.Item>
          <Form.Item name={['settings', 'mtu']} label="MTU">
            <InputNumber min={0} />
          </Form.Item>
          <Form.List name={['settings', 'gateway']}>
            {(fields, { add, remove }) => (
              <Form.Item label="Gateway">
                <Button size="small" onClick={() => add('')}>
                  <PlusOutlined />
                </Button>
                {fields.map((field, j) => (
                  <Space.Compact key={field.key} block className="mt-4">
                    <Form.Item name={field.name} noStyle>
                      <Input placeholder={j === 0 ? '10.0.0.1/16' : 'fc00::1/64'} />
                    </Form.Item>
                    <Button size="small" onClick={() => remove(field.name)}>
                      <MinusOutlined />
                    </Button>
                  </Space.Compact>
                ))}
              </Form.Item>
            )}
          </Form.List>
          <Form.List name={['settings', 'dns']}>
            {(fields, { add, remove }) => (
              <Form.Item label="DNS">
                <Button size="small" onClick={() => add('')}>
                  <PlusOutlined />
                </Button>
                {fields.map((field, j) => (
                  <Space.Compact key={field.key} block className="mt-4">
                    <Form.Item name={field.name} noStyle>
                      <Input placeholder={j === 0 ? '1.1.1.1' : '8.8.8.8'} />
                    </Form.Item>
                    <Button size="small" onClick={() => remove(field.name)}>
                      <MinusOutlined />
                    </Button>
                  </Space.Compact>
                ))}
              </Form.Item>
            )}
          </Form.List>
          <Form.Item name={['settings', 'userLevel']} label="User level">
            <InputNumber min={0} />
          </Form.Item>
          <Form.List name={['settings', 'autoSystemRoutingTable']}>
            {(fields, { add, remove }) => (
              <Form.Item
                label={
                  <Tooltip title="Windows-only. CIDRs added to the system routing table automatically so matching traffic goes through TUN.">
                    Auto system routes
                  </Tooltip>
                }
              >
                <Button size="small" onClick={() => add('')}>
                  <PlusOutlined />
                </Button>
                {fields.map((field, j) => (
                  <Space.Compact key={field.key} block className="mt-4">
                    <Form.Item name={field.name} noStyle>
                      <Input placeholder={j === 0 ? '0.0.0.0/0' : '::/0'} />
                    </Form.Item>
                    <Button size="small" onClick={() => remove(field.name)}>
                      <MinusOutlined />
                    </Button>
                  </Space.Compact>
                ))}
              </Form.Item>
            )}
          </Form.List>
          <Form.Item
            name={['settings', 'autoOutboundsInterface']}
            label={
              <Tooltip title="Physical interface for outbound traffic. Use 'auto' to detect; auto-enabled when Auto system routes is set.">
                Auto outbounds interface
              </Tooltip>
            }
          >
            <Input placeholder="auto" />
          </Form.Item>
        </>
      )}

      {protocol === Protocols.TUNNEL && (
        <>
          <Form.Item name={['settings', 'rewriteAddress']} label="Rewrite address">
            <Input />
          </Form.Item>
          <Form.Item name={['settings', 'rewritePort']} label="Rewrite port">
            <InputNumber min={0} max={65535} />
          </Form.Item>
          <Form.Item name={['settings', 'allowedNetwork']} label="Allowed network">
            <Select>
              <Select.Option value="tcp,udp">TCP, UDP</Select.Option>
              <Select.Option value="tcp">TCP</Select.Option>
              <Select.Option value="udp">UDP</Select.Option>
            </Select>
          </Form.Item>
          <Form.List name={['settings', 'portMap']}>
            {(fields, { add, remove }) => (
              <>
                <Form.Item label="Port map">
                  <Button size="small" onClick={() => add({ name: '', value: '' })}>
                    <PlusOutlined />
                  </Button>
                </Form.Item>
                {fields.length > 0 && (
                  <Form.Item wrapperCol={{ span: 24 }}>
                    {fields.map((field, idx) => (
                      <Space.Compact key={field.key} className="mb-8" block>
                        <InputAddon>{String(idx + 1)}</InputAddon>
                        <Form.Item name={[field.name, 'name']} noStyle>
                          <Input placeholder="5555" />
                        </Form.Item>
                        <Form.Item name={[field.name, 'value']} noStyle>
                          <Input placeholder="1.1.1.1:7777" />
                        </Form.Item>
                        <Button onClick={() => remove(field.name)}>
                          <MinusOutlined />
                        </Button>
                      </Space.Compact>
                    ))}
                  </Form.Item>
                )}
              </>
            )}
          </Form.List>
          <Form.Item
            name={['settings', 'followRedirect']}
            label="Follow redirect"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
        </>
      )}

      {(protocol === Protocols.HTTP || protocol === Protocols.MIXED) && (
        <>
          <Form.List name={['settings', 'accounts']}>
            {(fields, { add, remove }) => (
              <>
                <Form.Item label="Accounts">
                  <Button size="small" onClick={() => add({ user: '', pass: '' })}>
                    <PlusOutlined /> Add
                  </Button>
                </Form.Item>
                {fields.length > 0 && (
                  <Form.Item wrapperCol={{ span: 24 }}>
                    {fields.map((field, idx) => (
                      <Space.Compact key={field.key} className="mb-8" block>
                        <InputAddon>{String(idx + 1)}</InputAddon>
                        <Form.Item name={[field.name, 'user']} noStyle>
                          <Input placeholder="Username" />
                        </Form.Item>
                        <Form.Item name={[field.name, 'pass']} noStyle>
                          <Input placeholder="Password" />
                        </Form.Item>
                        <Button onClick={() => remove(field.name)}>
                          <MinusOutlined />
                        </Button>
                      </Space.Compact>
                    ))}
                  </Form.Item>
                )}
              </>
            )}
          </Form.List>
          {protocol === Protocols.HTTP && (
            <Form.Item
              name={['settings', 'allowTransparent']}
              label="Allow transparent"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          )}
          {protocol === Protocols.MIXED && (
            <>
              <Form.Item name={['settings', 'auth']} label="Auth">
                <Select>
                  <Select.Option value="noauth">noauth</Select.Option>
                  <Select.Option value="password">password</Select.Option>
                </Select>
              </Form.Item>
              <Form.Item
                name={['settings', 'udp']}
                label="UDP"
                valuePropName="checked"
              >
                <Switch />
              </Form.Item>
              {mixedUdpOn && (
                <Form.Item name={['settings', 'ip']} label="UDP IP">
                  <Input />
                </Form.Item>
              )}
            </>
          )}
        </>
      )}

      {protocol === Protocols.SHADOWSOCKS && (
        <>
          <Form.Item name={['settings', 'method']} label="Encryption method">
            <Select
              onChange={(v) => {
                form.setFieldValue(
                  ['settings', 'password'],
                  RandomUtil.randomShadowsocksPassword(v as string),
                );
              }}
            >
              {SSMethodSchema.options.map((m) => (
                <Select.Option key={m} value={m}>{m}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          {isSSWith2022 && (
            <Form.Item
              name={['settings', 'password']}
              label={
                <>
                  Password{' '}
                  <SyncOutlined
                    className="random-icon"
                    onClick={() => {
                      const method = form.getFieldValue(['settings', 'method']);
                      form.setFieldValue(
                        ['settings', 'password'],
                        RandomUtil.randomShadowsocksPassword(method as string),
                      );
                    }}
                  />
                </>
              }
            >
              <Input />
            </Form.Item>
          )}
          <Form.Item name={['settings', 'network']} label="Network">
            <Select style={{ width: 120 }}>
              <Select.Option value="tcp,udp">TCP, UDP</Select.Option>
              <Select.Option value="tcp">TCP</Select.Option>
              <Select.Option value="udp">UDP</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item
            name={['settings', 'ivCheck']}
            label="ivCheck"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
        </>
      )}

      {protocol === Protocols.VLESS && (
        <>
          <Form.Item name={['settings', 'decryption']} label={t('pages.inbounds.decryption')}>
            <Input />
          </Form.Item>
          <Form.Item name={['settings', 'encryption']} label={t('pages.inbounds.encryption')}>
            <Input />
          </Form.Item>
          <Form.Item label=" ">
            <Space size={8} wrap>
              <Button type="primary" loading={saving} onClick={() => getNewVlessEnc('x25519')}>
                {t('pages.inbounds.vlessAuthX25519')}
              </Button>
              <Button type="primary" loading={saving} onClick={() => getNewVlessEnc('mlkem768')}>
                {t('pages.inbounds.vlessAuthMlkem768')}
              </Button>
              <Button danger onClick={clearVlessEnc}>{t('clear')}</Button>
            </Space>
            <Text type="secondary" className="vless-auth-state">
              {t('pages.inbounds.vlessAuthSelected', { auth: selectedVlessAuth })}
            </Text>
          </Form.Item>
        </>
      )}
    </>
  );

  // Switching `network` swaps which per-network key (tcpSettings, wsSettings,
  // grpcSettings, ...) appears on the wire. We clear the previously selected
  // network's settings blob and seed a default empty object for the new one
  // so AntD's Form.Items aren't pointed at undefined nested paths.
  const onNetworkChange = (next: string) => {
    const ALL = ['tcpSettings', 'kcpSettings', 'wsSettings', 'grpcSettings', 'httpupgradeSettings', 'xhttpSettings'];
    const current = (form.getFieldValue('streamSettings') as Record<string, unknown>) ?? {};
    const cleaned: Record<string, unknown> = { ...current, network: next };
    for (const k of ALL) {
      if (k !== `${next}Settings`) delete cleaned[k];
    }
    cleaned[`${next}Settings`] = {};
    form.setFieldValue('streamSettings', cleaned);
  };

  const streamTab = (
    <>
      {protocol !== Protocols.HYSTERIA && (
        <Form.Item label="Transmission">
          <Select
            value={network}
            style={{ width: '75%' }}
            onChange={onNetworkChange}
          >
            <Select.Option value="tcp">TCP (RAW)</Select.Option>
            <Select.Option value="kcp">mKCP</Select.Option>
            <Select.Option value="ws">WebSocket</Select.Option>
            <Select.Option value="grpc">gRPC</Select.Option>
            <Select.Option value="httpupgrade">HTTPUpgrade</Select.Option>
            <Select.Option value="xhttp">XHTTP</Select.Option>
          </Select>
        </Form.Item>
      )}

      {network === 'tcp' && (
        <>
          <Form.Item
            name={['streamSettings', 'tcpSettings', 'acceptProxyProtocol']}
            label="Proxy Protocol"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item label={`HTTP ${t('camouflage')}`}>
            <Form.Item
              noStyle
              shouldUpdate={(prev, curr) =>
                prev.streamSettings?.tcpSettings?.header?.type
                !== curr.streamSettings?.tcpSettings?.header?.type
              }
            >
              {({ getFieldValue, setFieldValue }) => {
                const headerType = getFieldValue(
                  ['streamSettings', 'tcpSettings', 'header', 'type'],
                ) as string | undefined;
                return (
                  <Switch
                    checked={headerType === 'http'}
                    onChange={(v) => {
                      setFieldValue(
                        ['streamSettings', 'tcpSettings', 'header'],
                        v ? { type: 'http' } : { type: 'none' },
                      );
                    }}
                  />
                );
              }}
            </Form.Item>
          </Form.Item>
        </>
      )}

      {network === 'ws' && (
        <>
          <Form.Item
            name={['streamSettings', 'wsSettings', 'acceptProxyProtocol']}
            label="Proxy Protocol"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item name={['streamSettings', 'wsSettings', 'host']} label={t('host')}>
            <Input />
          </Form.Item>
          <Form.Item name={['streamSettings', 'wsSettings', 'path']} label={t('path')}>
            <Input />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'wsSettings', 'heartbeatPeriod']}
            label="Heartbeat Period"
          >
            <InputNumber min={0} />
          </Form.Item>
        </>
      )}

      {network === 'grpc' && (
        <>
          <Form.Item
            name={['streamSettings', 'grpcSettings', 'serviceName']}
            label="Service Name"
          >
            <Input />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'grpcSettings', 'authority']}
            label="Authority"
          >
            <Input />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'grpcSettings', 'multiMode']}
            label="Multi Mode"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
        </>
      )}

      {network === 'xhttp' && (
        <>
          <Form.Item name={['streamSettings', 'xhttpSettings', 'host']} label={t('host')}>
            <Input />
          </Form.Item>
          <Form.Item name={['streamSettings', 'xhttpSettings', 'path']} label={t('path')}>
            <Input />
          </Form.Item>
          <Form.Item name={['streamSettings', 'xhttpSettings', 'mode']} label="Mode">
            <Select style={{ width: '50%' }}>
              {(['auto', 'packet-up', 'stream-up', 'stream-one'] as const).map((m) => (
                <Select.Option key={m} value={m}>{m}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          {xhttpMode === 'packet-up' && (
            <>
              <Form.Item
                name={['streamSettings', 'xhttpSettings', 'scMaxBufferedPosts']}
                label="Max Buffered Upload"
              >
                <InputNumber />
              </Form.Item>
              <Form.Item
                name={['streamSettings', 'xhttpSettings', 'scMaxEachPostBytes']}
                label="Max Upload Size (Byte)"
              >
                <Input />
              </Form.Item>
            </>
          )}
          {xhttpMode === 'stream-up' && (
            <Form.Item
              name={['streamSettings', 'xhttpSettings', 'scStreamUpServerSecs']}
              label="Stream-Up Server"
            >
              <Input />
            </Form.Item>
          )}
          <Form.Item
            name={['streamSettings', 'xhttpSettings', 'serverMaxHeaderBytes']}
            label="Server Max Header Bytes"
          >
            <InputNumber min={0} placeholder="0 (default)" />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'xhttpSettings', 'xPaddingBytes']}
            label="Padding Bytes"
          >
            <Input />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'xhttpSettings', 'uplinkHTTPMethod']}
            label="Uplink HTTP Method"
          >
            <Select>
              <Select.Option value="">Default (POST)</Select.Option>
              <Select.Option value="POST">POST</Select.Option>
              <Select.Option value="PUT">PUT</Select.Option>
              <Select.Option value="GET" disabled={xhttpMode !== 'packet-up'}>
                GET (packet-up only)
              </Select.Option>
            </Select>
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'xhttpSettings', 'xPaddingObfsMode']}
            label="Padding Obfs Mode"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          {xhttpObfsMode && (
            <>
              <Form.Item
                name={['streamSettings', 'xhttpSettings', 'xPaddingKey']}
                label="Padding Key"
              >
                <Input placeholder="x_padding" />
              </Form.Item>
              <Form.Item
                name={['streamSettings', 'xhttpSettings', 'xPaddingHeader']}
                label="Padding Header"
              >
                <Input placeholder="X-Padding" />
              </Form.Item>
              <Form.Item
                name={['streamSettings', 'xhttpSettings', 'xPaddingPlacement']}
                label="Padding Placement"
              >
                <Select>
                  <Select.Option value="">Default (queryInHeader)</Select.Option>
                  <Select.Option value="queryInHeader">queryInHeader</Select.Option>
                  <Select.Option value="header">header</Select.Option>
                  <Select.Option value="cookie">cookie</Select.Option>
                  <Select.Option value="query">query</Select.Option>
                </Select>
              </Form.Item>
              <Form.Item
                name={['streamSettings', 'xhttpSettings', 'xPaddingMethod']}
                label="Padding Method"
              >
                <Select>
                  <Select.Option value="">Default (repeat-x)</Select.Option>
                  <Select.Option value="repeat-x">repeat-x</Select.Option>
                  <Select.Option value="tokenish">tokenish</Select.Option>
                </Select>
              </Form.Item>
            </>
          )}
          <Form.Item
            name={['streamSettings', 'xhttpSettings', 'sessionPlacement']}
            label="Session Placement"
          >
            <Select>
              <Select.Option value="">Default (path)</Select.Option>
              <Select.Option value="path">path</Select.Option>
              <Select.Option value="header">header</Select.Option>
              <Select.Option value="cookie">cookie</Select.Option>
              <Select.Option value="query">query</Select.Option>
            </Select>
          </Form.Item>
          {xhttpSessionPlacement && xhttpSessionPlacement !== 'path' && (
            <Form.Item
              name={['streamSettings', 'xhttpSettings', 'sessionKey']}
              label="Session Key"
            >
              <Input placeholder="x_session" />
            </Form.Item>
          )}
          <Form.Item
            name={['streamSettings', 'xhttpSettings', 'seqPlacement']}
            label="Sequence Placement"
          >
            <Select>
              <Select.Option value="">Default (path)</Select.Option>
              <Select.Option value="path">path</Select.Option>
              <Select.Option value="header">header</Select.Option>
              <Select.Option value="cookie">cookie</Select.Option>
              <Select.Option value="query">query</Select.Option>
            </Select>
          </Form.Item>
          {xhttpSeqPlacement && xhttpSeqPlacement !== 'path' && (
            <Form.Item
              name={['streamSettings', 'xhttpSettings', 'seqKey']}
              label="Sequence Key"
            >
              <Input placeholder="x_seq" />
            </Form.Item>
          )}
          {xhttpMode === 'packet-up' && (
            <>
              <Form.Item
                name={['streamSettings', 'xhttpSettings', 'uplinkDataPlacement']}
                label="Uplink Data Placement"
              >
                <Select>
                  <Select.Option value="">Default (body)</Select.Option>
                  <Select.Option value="body">body</Select.Option>
                  <Select.Option value="header">header</Select.Option>
                  <Select.Option value="cookie">cookie</Select.Option>
                  <Select.Option value="query">query</Select.Option>
                </Select>
              </Form.Item>
              {xhttpUplinkPlacement && xhttpUplinkPlacement !== 'body' && (
                <Form.Item
                  name={['streamSettings', 'xhttpSettings', 'uplinkDataKey']}
                  label="Uplink Data Key"
                >
                  <Input placeholder="x_data" />
                </Form.Item>
              )}
            </>
          )}
          <Form.Item
            name={['streamSettings', 'xhttpSettings', 'noSSEHeader']}
            label="No SSE Header"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
        </>
      )}

      {network === 'httpupgrade' && (
        <>
          <Form.Item
            name={['streamSettings', 'httpupgradeSettings', 'acceptProxyProtocol']}
            label="Proxy Protocol"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'httpupgradeSettings', 'host']}
            label={t('host')}
          >
            <Input />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'httpupgradeSettings', 'path']}
            label={t('path')}
          >
            <Input />
          </Form.Item>
        </>
      )}

      {network === 'kcp' && (
        <>
          <Form.Item name={['streamSettings', 'kcpSettings', 'mtu']} label="MTU">
            <InputNumber min={576} max={1460} />
          </Form.Item>
          <Form.Item name={['streamSettings', 'kcpSettings', 'tti']} label="TTI (ms)">
            <InputNumber min={10} max={100} />
          </Form.Item>
          <Form.Item name={['streamSettings', 'kcpSettings', 'upCap']} label="Uplink (MB/s)">
            <InputNumber min={0} />
          </Form.Item>
          <Form.Item name={['streamSettings', 'kcpSettings', 'downCap']} label="Downlink (MB/s)">
            <InputNumber min={0} />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'kcpSettings', 'cwndMultiplier']}
            label="CWND Multiplier"
          >
            <InputNumber min={1} />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'kcpSettings', 'maxSendingWindow']}
            label="Max Sending Window"
          >
            <InputNumber min={0} />
          </Form.Item>
        </>
      )}

      <Form.Item label="External Proxy">
        <Switch checked={externalProxyOn} onChange={toggleExternalProxy} />
      </Form.Item>
      {externalProxyOn && (
        <Form.List name={['streamSettings', 'externalProxy']}>
          {(fields, { add, remove }) => (
            <>
              <Form.Item label=" " colon={false}>
                <Button
                  size="small"
                  type="primary"
                  onClick={() => add({
                    forceTls: 'same',
                    dest: '',
                    port: 443,
                    remark: '',
                    sni: '',
                    fingerprint: '',
                    alpn: [],
                  })}
                >
                  <PlusOutlined />
                </Button>
              </Form.Item>
              <Form.Item wrapperCol={{ span: 24 }}>
                {fields.map((field) => (
                  <div key={field.key} style={{ margin: '8px 0' }}>
                    <Space.Compact block>
                      <Form.Item name={[field.name, 'forceTls']} noStyle>
                        <Select style={{ width: '20%' }}>
                          <Select.Option value="same">{t('pages.inbounds.same')}</Select.Option>
                          <Select.Option value="none">{t('none')}</Select.Option>
                          <Select.Option value="tls">TLS</Select.Option>
                        </Select>
                      </Form.Item>
                      <Form.Item name={[field.name, 'dest']} noStyle>
                        <Input style={{ width: '30%' }} placeholder={t('host')} />
                      </Form.Item>
                      <Form.Item name={[field.name, 'port']} noStyle>
                        <InputNumber style={{ width: '15%' }} min={1} max={65535} />
                      </Form.Item>
                      <Form.Item name={[field.name, 'remark']} noStyle>
                        <Input style={{ width: '25%' }} placeholder={t('pages.inbounds.remark')} />
                      </Form.Item>
                      <InputAddon onClick={() => remove(field.name)}>
                        <MinusOutlined />
                      </InputAddon>
                    </Space.Compact>
                    <Form.Item
                      noStyle
                      shouldUpdate={(prev, curr) =>
                        prev.streamSettings?.externalProxy?.[field.name]?.forceTls
                        !== curr.streamSettings?.externalProxy?.[field.name]?.forceTls
                      }
                    >
                      {({ getFieldValue }) => {
                        const ft = getFieldValue([
                          'streamSettings', 'externalProxy', field.name, 'forceTls',
                        ]);
                        if (ft !== 'tls') return null;
                        return (
                          <Space.Compact style={{ marginTop: 6 }} block>
                            <Form.Item name={[field.name, 'sni']} noStyle>
                              <Input style={{ width: '30%' }} placeholder="SNI (defaults to host)" />
                            </Form.Item>
                            <Form.Item name={[field.name, 'fingerprint']} noStyle>
                              <Select style={{ width: '30%' }} placeholder="Fingerprint">
                                <Select.Option value="">Default</Select.Option>
                                {Object.values(UTLS_FINGERPRINT).map((fp) => (
                                  <Select.Option key={fp} value={fp}>{fp}</Select.Option>
                                ))}
                              </Select>
                            </Form.Item>
                            <Form.Item name={[field.name, 'alpn']} noStyle>
                              <Select mode="multiple" style={{ width: '40%' }} placeholder="ALPN">
                                {Object.values(ALPN_OPTION).map((a) => (
                                  <Select.Option key={a} value={a}>{a}</Select.Option>
                                ))}
                              </Select>
                            </Form.Item>
                          </Space.Compact>
                        );
                      }}
                    </Form.Item>
                  </div>
                ))}
              </Form.Item>
            </>
          )}
        </Form.List>
      )}

      <Form.Item label="Sockopt">
        <Switch checked={sockoptOn} onChange={toggleSockopt} />
      </Form.Item>
      {sockoptOn && (
        <>
          <Form.Item name={['streamSettings', 'sockopt', 'mark']} label="Route Mark">
            <InputNumber min={0} />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'sockopt', 'tcpKeepAliveInterval']}
            label="TCP Keep Alive Interval"
          >
            <InputNumber min={0} />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'sockopt', 'tcpKeepAliveIdle']}
            label="TCP Keep Alive Idle"
          >
            <InputNumber min={0} />
          </Form.Item>
          <Form.Item name={['streamSettings', 'sockopt', 'tcpMaxSeg']} label="TCP Max Seg">
            <InputNumber min={0} />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'sockopt', 'tcpUserTimeout']}
            label="TCP User Timeout"
          >
            <InputNumber min={0} />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'sockopt', 'tcpWindowClamp']}
            label="TCP Window Clamp"
          >
            <InputNumber min={0} />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'sockopt', 'acceptProxyProtocol']}
            label="Proxy Protocol"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'sockopt', 'tcpFastOpen']}
            label="TCP Fast Open"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'sockopt', 'tcpMptcp']}
            label="Multipath TCP"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'sockopt', 'penetrate']}
            label="Penetrate"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'sockopt', 'V6Only']}
            label="V6 Only"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'sockopt', 'domainStrategy']}
            label="Domain Strategy"
          >
            <Select style={{ width: '50%' }}>
              {Object.values(DOMAIN_STRATEGY_OPTION).map((d) => (
                <Select.Option key={d} value={d}>{d}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'sockopt', 'tcpcongestion']}
            label="TCP Congestion"
          >
            <Select style={{ width: '50%' }}>
              {Object.values(TCP_CONGESTION_OPTION).map((c) => (
                <Select.Option key={c} value={c}>{c}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item name={['streamSettings', 'sockopt', 'tproxy']} label="TProxy">
            <Select style={{ width: '50%' }}>
              <Select.Option value="off">Off</Select.Option>
              <Select.Option value="redirect">Redirect</Select.Option>
              <Select.Option value="tproxy">TProxy</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name={['streamSettings', 'sockopt', 'dialerProxy']} label="Dialer Proxy">
            <Input />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'sockopt', 'interfaceName']}
            label="Interface Name"
          >
            <Input />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'sockopt', 'trustedXForwardedFor']}
            label="Trusted X-Forwarded-For"
          >
            <Select mode="tags" style={{ width: '100%' }} tokenSeparators={[',']}>
              <Select.Option value="CF-Connecting-IP">CF-Connecting-IP</Select.Option>
              <Select.Option value="X-Real-IP">X-Real-IP</Select.Option>
              <Select.Option value="True-Client-IP">True-Client-IP</Select.Option>
              <Select.Option value="X-Client-IP">X-Client-IP</Select.Option>
            </Select>
          </Form.Item>
        </>
      )}
    </>
  );

  const securityTab = (
    <>
      <Form.Item label={t('pages.inbounds.securityTab')}>
        <Form.Item
          noStyle
          shouldUpdate={(prev, curr) =>
            prev.streamSettings?.security !== curr.streamSettings?.security
          }
        >
          {({ getFieldValue }) => {
            const sec = getFieldValue(['streamSettings', 'security']) ?? 'none';
            return (
              <Select
                value={sec}
                disabled={!tlsAllowed}
                onChange={onSecurityChange}
                style={{ width: 180 }}
              >
                <Select.Option value="none">none</Select.Option>
                <Select.Option value="tls">tls</Select.Option>
                {realityAllowed && <Select.Option value="reality">reality</Select.Option>}
              </Select>
            );
          }}
        </Form.Item>
      </Form.Item>

      {security === 'tls' && (
        <>
          <Form.Item name={['streamSettings', 'tlsSettings', 'serverName']} label="SNI">
            <Input placeholder="Server Name Indication" />
          </Form.Item>
          <Form.Item name={['streamSettings', 'tlsSettings', 'cipherSuites']} label="Cipher Suites">
            <Select>
              <Select.Option value="">Auto</Select.Option>
              {Object.entries(TLS_CIPHER_OPTION).map(([k, v]) => (
                <Select.Option key={v} value={v}>{k}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item label="Min/Max Version">
            <Space.Compact block>
              <Form.Item name={['streamSettings', 'tlsSettings', 'minVersion']} noStyle>
                <Select style={{ width: '50%' }}>
                  {Object.values(TLS_VERSION_OPTION).map((v) => (
                    <Select.Option key={v} value={v}>{v}</Select.Option>
                  ))}
                </Select>
              </Form.Item>
              <Form.Item name={['streamSettings', 'tlsSettings', 'maxVersion']} noStyle>
                <Select style={{ width: '50%' }}>
                  {Object.values(TLS_VERSION_OPTION).map((v) => (
                    <Select.Option key={v} value={v}>{v}</Select.Option>
                  ))}
                </Select>
              </Form.Item>
            </Space.Compact>
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'tlsSettings', 'settings', 'fingerprint']}
            label="uTLS"
          >
            <Select>
              <Select.Option value="">None</Select.Option>
              {Object.values(UTLS_FINGERPRINT).map((fp) => (
                <Select.Option key={fp} value={fp}>{fp}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item name={['streamSettings', 'tlsSettings', 'alpn']} label="ALPN">
            <Select mode="multiple" tokenSeparators={[',']} style={{ width: '100%' }}>
              {Object.values(ALPN_OPTION).map((a) => (
                <Select.Option key={a} value={a}>{a}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'tlsSettings', 'rejectUnknownSni']}
            label="Reject Unknown SNI"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'tlsSettings', 'disableSystemRoot']}
            label="Disable System Root"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'tlsSettings', 'enableSessionResumption']}
            label="Session Resumption"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>

          <Form.List name={['streamSettings', 'tlsSettings', 'certificates']}>
            {(certFields, { add, remove }) => (
              <>
                <Form.Item label={t('certificate')}>
                  <Button
                    type="primary"
                    size="small"
                    onClick={() => add({
                      useFile: true,
                      certificateFile: '',
                      keyFile: '',
                      certificate: [],
                      key: [],
                      oneTimeLoading: false,
                      usage: 'encipherment',
                      buildChain: false,
                    })}
                  >
                    <PlusOutlined />
                  </Button>
                </Form.Item>
                {certFields.map((certField, idx) => (
                  <div key={certField.key}>
                    <Form.Item
                      name={[certField.name, 'useFile']}
                      label={`${t('certificate')} ${idx + 1}`}
                    >
                      <Radio.Group buttonStyle="solid">
                        <Radio.Button value={true}>
                          {t('pages.inbounds.certificatePath')}
                        </Radio.Button>
                        <Radio.Button value={false}>
                          {t('pages.inbounds.certificateContent')}
                        </Radio.Button>
                      </Radio.Group>
                    </Form.Item>
                    {certFields.length > 1 && (
                      <Form.Item label=" ">
                        <Button
                          size="small"
                          danger
                          onClick={() => remove(certField.name)}
                        >
                          <MinusOutlined /> Remove
                        </Button>
                      </Form.Item>
                    )}
                    <Form.Item
                      noStyle
                      shouldUpdate={(prev, curr) =>
                        prev.streamSettings?.tlsSettings?.certificates?.[certField.name]?.useFile
                        !== curr.streamSettings?.tlsSettings?.certificates?.[certField.name]?.useFile
                      }
                    >
                      {({ getFieldValue }) => {
                        const useFile = getFieldValue([
                          'streamSettings', 'tlsSettings', 'certificates',
                          certField.name, 'useFile',
                        ]);
                        return useFile ? (
                          <>
                            <Form.Item
                              name={[certField.name, 'certificateFile']}
                              label={t('pages.inbounds.publicKey')}
                            >
                              <Input />
                            </Form.Item>
                            <Form.Item
                              name={[certField.name, 'keyFile']}
                              label={t('pages.inbounds.privatekey')}
                            >
                              <Input />
                            </Form.Item>
                          </>
                        ) : (
                          <>
                            <Form.Item
                              name={[certField.name, 'certificate']}
                              label={t('pages.inbounds.publicKey')}
                              normalize={(v) => typeof v === 'string'
                                ? v.split('\n')
                                : v}
                              getValueProps={(v) => ({
                                value: Array.isArray(v) ? v.join('\n') : v,
                              })}
                            >
                              <TextArea autoSize={{ minRows: 3, maxRows: 8 }} />
                            </Form.Item>
                            <Form.Item
                              name={[certField.name, 'key']}
                              label={t('pages.inbounds.privatekey')}
                              normalize={(v) => typeof v === 'string'
                                ? v.split('\n')
                                : v}
                              getValueProps={(v) => ({
                                value: Array.isArray(v) ? v.join('\n') : v,
                              })}
                            >
                              <TextArea autoSize={{ minRows: 3, maxRows: 8 }} />
                            </Form.Item>
                          </>
                        );
                      }}
                    </Form.Item>
                    <Form.Item
                      name={[certField.name, 'oneTimeLoading']}
                      label="One Time Loading"
                      valuePropName="checked"
                    >
                      <Switch />
                    </Form.Item>
                    <Form.Item
                      name={[certField.name, 'usage']}
                      label="Usage Option"
                    >
                      <Select style={{ width: '50%' }}>
                        {Object.values(USAGE_OPTION).map((u) => (
                          <Select.Option key={u} value={u}>{u}</Select.Option>
                        ))}
                      </Select>
                    </Form.Item>
                    <Form.Item
                      noStyle
                      shouldUpdate={(prev, curr) =>
                        prev.streamSettings?.tlsSettings?.certificates?.[certField.name]?.usage
                        !== curr.streamSettings?.tlsSettings?.certificates?.[certField.name]?.usage
                      }
                    >
                      {({ getFieldValue }) => {
                        const usage = getFieldValue([
                          'streamSettings', 'tlsSettings', 'certificates',
                          certField.name, 'usage',
                        ]);
                        if (usage !== 'issue') return null;
                        return (
                          <Form.Item
                            name={[certField.name, 'buildChain']}
                            label="Build Chain"
                            valuePropName="checked"
                          >
                            <Switch />
                          </Form.Item>
                        );
                      }}
                    </Form.Item>
                  </div>
                ))}
              </>
            )}
          </Form.List>

          <Form.Item name={['streamSettings', 'tlsSettings', 'echServerKeys']} label="ECH key">
            <Input />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'tlsSettings', 'settings', 'echConfigList']}
            label="ECH config"
          >
            <Input />
          </Form.Item>
          <Form.Item label=" ">
            <Space>
              <Button type="primary" loading={saving} onClick={getNewEchCert}>
                Get New ECH Cert
              </Button>
              <Button danger onClick={clearEchCert}>Clear</Button>
            </Space>
          </Form.Item>
        </>
      )}

      {security === 'reality' && (
        <>
          <Form.Item
            name={['streamSettings', 'realitySettings', 'show']}
            label="Show"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item name={['streamSettings', 'realitySettings', 'xver']} label="Xver">
            <InputNumber min={0} />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'realitySettings', 'settings', 'fingerprint']}
            label="uTLS"
          >
            <Select>
              {Object.values(UTLS_FINGERPRINT).map((fp) => (
                <Select.Option key={fp} value={fp}>{fp}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'realitySettings', 'target']}
            label={
              <>
                Target{' '}
                <SyncOutlined className="random-icon" onClick={randomizeRealityTarget} />
              </>
            }
          >
            <Input />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'realitySettings', 'serverNames']}
            label={
              <>
                SNI{' '}
                <SyncOutlined className="random-icon" onClick={randomizeRealityTarget} />
              </>
            }
          >
            <Select mode="tags" tokenSeparators={[',']} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'realitySettings', 'maxTimediff']}
            label="Max Time Diff (ms)"
          >
            <InputNumber min={0} />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'realitySettings', 'minClientVer']}
            label="Min Client Ver"
          >
            <Input placeholder="25.9.11" />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'realitySettings', 'maxClientVer']}
            label="Max Client Ver"
          >
            <Input placeholder="25.9.11" />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'realitySettings', 'shortIds']}
            label={
              <>
                Short IDs{' '}
                <SyncOutlined className="random-icon" onClick={randomizeShortIds} />
              </>
            }
          >
            <Select mode="tags" tokenSeparators={[',']} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'realitySettings', 'settings', 'spiderX']}
            label="SpiderX"
          >
            <Input />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'realitySettings', 'settings', 'publicKey']}
            label={t('pages.inbounds.publicKey')}
          >
            <Input.TextArea autoSize={{ minRows: 1, maxRows: 4 }} />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'realitySettings', 'privateKey']}
            label={t('pages.inbounds.privatekey')}
          >
            <Input.TextArea autoSize={{ minRows: 1, maxRows: 4 }} />
          </Form.Item>
          <Form.Item label=" ">
            <Space>
              <Button type="primary" loading={saving} onClick={genRealityKeypair}>
                Get New Cert
              </Button>
              <Button danger onClick={clearRealityKeypair}>Clear</Button>
            </Space>
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'realitySettings', 'mldsa65Seed']}
            label="mldsa65 Seed"
          >
            <Input.TextArea autoSize={{ minRows: 2, maxRows: 6 }} />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'realitySettings', 'settings', 'mldsa65Verify']}
            label="mldsa65 Verify"
          >
            <Input.TextArea autoSize={{ minRows: 2, maxRows: 6 }} />
          </Form.Item>
          <Form.Item label=" ">
            <Space>
              <Button type="primary" loading={saving} onClick={genMldsa65}>
                Get New Seed
              </Button>
              <Button danger onClick={clearMldsa65}>Clear</Button>
            </Space>
          </Form.Item>
        </>
      )}
    </>
  );

  const sniffingTab = (
    <>
      <Form.Item name={['sniffing', 'enabled']} label={t('enable')} valuePropName="checked">
        <Switch />
      </Form.Item>

      {sniffingEnabled && (
        <>
          <Form.Item name={['sniffing', 'destOverride']} wrapperCol={{ span: 24 }}>
            <Checkbox.Group>
              {Object.entries(SNIFFING_OPTION).map(([key, value]) => (
                <Checkbox key={key} value={value}>{key}</Checkbox>
              ))}
            </Checkbox.Group>
          </Form.Item>

          <Form.Item
            name={['sniffing', 'metadataOnly']}
            label={t('pages.inbounds.sniffingMetadataOnly')}
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>

          <Form.Item
            name={['sniffing', 'routeOnly']}
            label={t('pages.inbounds.sniffingRouteOnly')}
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>

          <Form.Item
            name={['sniffing', 'ipsExcluded']}
            label={t('pages.inbounds.sniffingIpsExcluded')}
          >
            <Select
              mode="tags"
              tokenSeparators={[',']}
              placeholder="IP/CIDR/geoip:*/ext:*"
              style={{ width: '100%' }}
            />
          </Form.Item>

          <Form.Item
            name={['sniffing', 'domainsExcluded']}
            label={t('pages.inbounds.sniffingDomainsExcluded')}
          >
            <Select
              mode="tags"
              tokenSeparators={[',']}
              placeholder="domain:*/ext:*"
              style={{ width: '100%' }}
            />
          </Form.Item>
        </>
      )}
    </>
  );

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={title}
        okText={okText}
        cancelText={t('close')}
        confirmLoading={saving}
        mask={{ closable: false }}
        width={780}
        onOk={submit}
        onCancel={onClose}
        destroyOnHidden
      >
        <Form
          form={form}
          colon={false}
          labelCol={{ sm: { span: 8 } }}
          wrapperCol={{ sm: { span: 14 } }}
          onValuesChange={onValuesChange}
        >
          <Tabs items={[
            { key: 'basic', label: t('pages.xray.basicTemplate'), children: basicTab },
            ...(([
              Protocols.VLESS,
              Protocols.SHADOWSOCKS,
              Protocols.HTTP,
              Protocols.MIXED,
              Protocols.TUNNEL,
              Protocols.TUN,
              Protocols.WIREGUARD,
            ] as string[]).includes(protocol)
              ? [{ key: 'protocol', label: t('pages.inbounds.protocol'), children: protocolTab }]
              : []),
            ...(streamEnabled
              ? [
                  { key: 'stream', label: t('pages.inbounds.streamTab'), children: streamTab },
                  { key: 'security', label: t('pages.inbounds.securityTab'), children: securityTab },
                ]
              : []),
            { key: 'sniffing', label: t('pages.inbounds.sniffingTab'), children: sniffingTab },
          ]} />
        </Form>
      </Modal>
    </>
  );
}
