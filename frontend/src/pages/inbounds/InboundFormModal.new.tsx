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
import { canEnableStream, isSS2022 } from '@/lib/xray/protocol-capabilities';
import { SSMethodSchema } from '@/schemas/protocols/inbound/shadowsocks';
import {
  InboundFormBaseSchema,
  InboundFormSchema,
  type InboundFormValues,
} from '@/schemas/forms/inbound-form';
import { antdRule } from '@/utils/zodForm';
import { Protocols, SNIFFING_OPTION } from '@/schemas/primitives';
import DateTimePicker from '@/components/DateTimePicker';
import InputAddon from '@/components/InputAddon';
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
  const streamEnabled = canEnableStream({ protocol });
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
              ? [{ key: 'stream', label: t('pages.inbounds.streamTab'), children: streamTab }]
              : []),
            { key: 'sniffing', label: t('pages.inbounds.sniffingTab'), children: sniffingTab },
          ]} />
        </Form>
      </Modal>
    </>
  );
}
