import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Form,
  Input,
  InputNumber,
  message,
  Modal,
  Radio,
  Select,
  Space,
  Switch,
  Tabs,
  Checkbox,
} from 'antd';
import { SyncOutlined, PlusOutlined, MinusOutlined, DeleteOutlined } from '@ant-design/icons';

import { Wireguard } from '@/utils';
import InputAddon from '@/components/InputAddon';
import {
  Outbound,
  Protocols,
  SSMethods,
  TLS_FLOW_CONTROL,
  UTLS_FINGERPRINT,
  ALPN_OPTION,
  SNIFFING_OPTION,
  USERS_SECURITY,
  OutboundDomainStrategies,
  WireguardDomainStrategy,
  Address_Port_Strategy,
  MODE_OPTION,
  DNSRuleActions,
} from '@/models/outbound.js';
import FinalMaskForm from '@/components/FinalMaskForm';
import JsonEditor from '@/components/JsonEditor';
import './OutboundFormModal.css';

interface OutboundFormModalProps {
  open: boolean;
  outbound: Record<string, unknown> | null;
  existingTags: string[];
  onClose: () => void;
  onConfirm: (outbound: Record<string, unknown>) => void;
}

const PROTOCOL_OPTIONS = Object.values(Protocols) as string[];
const SECURITY_OPTIONS = Object.values(USERS_SECURITY) as string[];
const FLOW_OPTIONS = Object.values(TLS_FLOW_CONTROL) as string[];
const UTLS_OPTIONS = Object.values(UTLS_FINGERPRINT) as string[];
const ALPN_OPTIONS = Object.values(ALPN_OPTION) as string[];
const NETWORKS = ['tcp', 'kcp', 'ws', 'grpc', 'httpupgrade', 'xhttp'];
const NETWORK_LABELS: Record<string, string> = {
  tcp: 'TCP (RAW)',
  kcp: 'mKCP',
  ws: 'WebSocket',
  grpc: 'gRPC',
  httpupgrade: 'HTTPUpgrade',
  xhttp: 'XHTTP',
};

export default function OutboundFormModal({
  open,
  outbound: outboundProp,
  existingTags,
  onClose,
  onConfirm,
}: OutboundFormModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const outboundRef = useRef<any>(null);
  const [, setTick] = useState(0);
  const [activeKey, setActiveKey] = useState('1');
  const [linkInput, setLinkInput] = useState('');
  const [advancedJson, setAdvancedJson] = useState('');
  const revertingTabRef = useRef(false);

  const isEdit = outboundProp != null;

  const refresh = useCallback(() => setTick((n) => n + 1), []);

  const primeAdvancedJson = useCallback(() => {
    const ob = outboundRef.current;
    if (!ob) {
      setAdvancedJson('');
      return;
    }
    try {
      setAdvancedJson(JSON.stringify(ob.toJson(), null, 2));
    } catch {
      setAdvancedJson('');
    }
  }, []);

  useEffect(() => {
    if (!open) return;
    outboundRef.current = outboundProp
      ? Outbound.fromJson(outboundProp)
      : new Outbound();
    setActiveKey('1');
    setLinkInput('');
    primeAdvancedJson();
    refresh();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, outboundProp]);

  function applyAdvancedJsonToForm(): boolean {
    const raw = advancedJson.trim();
    if (!raw) return true;
    const ob = outboundRef.current;
    let currentJson = '';
    try {
      currentJson = JSON.stringify(ob?.toJson() ?? {}, null, 2);
    } catch {
      /* ignore */
    }
    if (raw === currentJson.trim()) return true;
    let parsed;
    try {
      parsed = JSON.parse(raw);
    } catch (e) {
      messageApi.error(`JSON: ${(e as Error).message}`);
      return false;
    }
    try {
      const fallbackTag = ob?.tag;
      const next = Outbound.fromJson(parsed);
      if (!next.tag && fallbackTag) next.tag = fallbackTag;
      outboundRef.current = next;
      refresh();
      return true;
    } catch (e) {
      messageApi.error(`JSON: ${(e as Error).message}`);
      return false;
    }
  }

  function onTabChange(key: string) {
    if (document.activeElement instanceof HTMLElement) {
      document.activeElement.blur();
    }
    if (revertingTabRef.current) {
      revertingTabRef.current = false;
      setActiveKey(key);
      return;
    }
    const prev = activeKey;
    if (key === '2') {
      primeAdvancedJson();
      setActiveKey(key);
    } else if (key === '1' && prev === '2') {
      if (!applyAdvancedJsonToForm()) {
        revertingTabRef.current = true;
        setActiveKey('2');
      } else {
        setActiveKey(key);
      }
    } else {
      setActiveKey(key);
    }
  }

  const ob = outboundRef.current;

  const proto = ob?.protocol;
  const isVMess = proto === Protocols.VMess;
  const isVLESS = proto === Protocols.VLESS;
  const isVMessOrVLess = isVMess || isVLESS;
  const isTrojan = proto === Protocols.Trojan;
  const isShadowsocks = proto === Protocols.Shadowsocks;
  const isFreedom = proto === Protocols.Freedom;
  const isBlackhole = proto === Protocols.Blackhole;
  const isDNS = proto === Protocols.DNS;
  const isWireguard = proto === Protocols.Wireguard;
  const isHysteria = proto === Protocols.Hysteria;
  const isLoopback = proto === Protocols.Loopback;

  function onProtocolChange(next: string) {
    if (!ob) return;
    ob.protocol = next;
    refresh();
  }

  function streamNetworkChange(next: string) {
    if (!ob?.stream) return;
    ob.stream.network = next;
    if (!ob.canEnableTls()) ob.stream.security = 'none';
    refresh();
  }

  function regenerateWgKeys() {
    if (!ob?.settings) return;
    const pair = Wireguard.generateKeypair();
    ob.settings.secretKey = pair.privateKey;
    ob.settings.pubKey = pair.publicKey;
    refresh();
  }

  const duplicateTag = useMemo(() => {
    if (!ob?.tag) return false;
    const myTag = ob.tag.trim();
    if (!myTag) return false;
    if (isEdit && (outboundProp?.tag as string | undefined) === myTag) return false;
    return (existingTags || []).includes(myTag);
  }, [ob?.tag, existingTags, isEdit, outboundProp]);

  const tagEmpty = !ob?.tag?.trim();

  const tagValidateStatus: 'error' | 'warning' | 'success' = tagEmpty
    ? 'error'
    : duplicateTag
      ? 'warning'
      : 'success';

  const tagHelp = tagEmpty
    ? 'Tag is required'
    : duplicateTag
      ? 'Tag already used by another outbound'
      : '';

  function onOk() {
    if (!ob) return;
    if (activeKey === '2' && !applyAdvancedJsonToForm()) return;
    if (!ob.tag?.trim()) {
      messageApi.error('Tag is required');
      return;
    }
    if (duplicateTag) {
      messageApi.error('Tag already used by another outbound');
      return;
    }
    onConfirm(ob.toJson());
  }

  function convertLink() {
    const link = linkInput.trim();
    if (!link) return;
    try {
      const next = Outbound.fromLink(link);
      if (!next) {
        messageApi.error('Wrong Link!');
        return;
      }
      outboundRef.current = next;
      primeAdvancedJson();
      setLinkInput('');
      messageApi.success('Link imported successfully...');
      setActiveKey('1');
      refresh();
    } catch (e) {
      messageApi.error(`Link parse: ${(e as Error).message}`);
    }
  }

  const title = isEdit
    ? `${t('edit')} ${t('pages.xray.Outbounds')}`
    : `+ ${t('pages.xray.Outbounds')}`;
  const okText = isEdit ? t('pages.clients.submitEdit') : t('create');

  if (!ob) {
    return (
      <>
        {messageContextHolder}
        <Modal open={open} title={title} footer={null} onCancel={onClose} />
      </>
    );
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
              <Form colon={false} labelCol={{ md: { span: 8 } }} wrapperCol={{ md: { span: 14 } }}>
                <Form.Item label={t('protocol')}>
                  <Select
                    value={proto}
                    onChange={onProtocolChange}
                    options={PROTOCOL_OPTIONS.map((p) => ({ value: p, label: p }))}
                  />
                </Form.Item>

                <Form.Item label="Tag" validateStatus={tagValidateStatus} help={tagHelp} hasFeedback>
                  <Input
                    value={ob.tag}
                    placeholder="unique-tag"
                    onChange={(e) => {
                      ob.tag = e.target.value;
                      refresh();
                    }}
                  />
                </Form.Item>

                <Form.Item label="Send through">
                  <Input
                    value={ob.sendThrough || ''}
                    placeholder="local IP"
                    onChange={(e) => {
                      ob.sendThrough = e.target.value;
                      refresh();
                    }}
                  />
                </Form.Item>

                {isFreedom && <FreedomFields ob={ob} refresh={refresh} />}
                {isBlackhole && (
                  <Form.Item label="Response Type">
                    <Select
                      value={ob.settings.type || ''}
                      onChange={(v) => { ob.settings.type = v; refresh(); }}
                      options={[
                        { value: '', label: '(empty)' },
                        { value: 'none', label: 'none' },
                        { value: 'http', label: 'http' },
                      ]}
                    />
                  </Form.Item>
                )}
                {isLoopback && (
                  <Form.Item label="Inbound tag">
                    <Input
                      value={ob.settings.inboundTag || ''}
                      placeholder="inbound tag using in routing rules"
                      onChange={(e) => { ob.settings.inboundTag = e.target.value; refresh(); }}
                    />
                  </Form.Item>
                )}
                {isDNS && <DnsFields ob={ob} refresh={refresh} t={t} />}
                {isWireguard && <WireguardFields ob={ob} refresh={refresh} regenerate={regenerateWgKeys} t={t} />}

                {ob.hasAddressPort() && (
                  <>
                    <Form.Item label={t('pages.inbounds.address')}>
                      <Input
                        value={ob.settings.address || ''}
                        onChange={(e) => { ob.settings.address = e.target.value; refresh(); }}
                      />
                    </Form.Item>
                    <Form.Item label={t('pages.inbounds.port')}>
                      <InputNumber
                        value={ob.settings.port || 0}
                        min={1}
                        max={65535}
                        onChange={(v) => { ob.settings.port = Number(v) || 0; refresh(); }}
                      />
                    </Form.Item>
                  </>
                )}

                {isVMessOrVLess && (
                  <VMessVLessFields ob={ob} refresh={refresh} isVMess={isVMess} isVLESS={isVLESS} t={t} />
                )}

                {(isTrojan || isShadowsocks) && (
                  <Form.Item label={t('password')}>
                    <Input
                      value={ob.settings.password || ''}
                      onChange={(e) => { ob.settings.password = e.target.value; refresh(); }}
                    />
                  </Form.Item>
                )}

                {isShadowsocks && (
                  <>
                    <Form.Item label={t('encryption')}>
                      <Select
                        value={ob.settings.method}
                        onChange={(v) => { ob.settings.method = v; refresh(); }}
                        options={Object.entries(SSMethods).map(([k, v]) => ({ value: v as string, label: k }))}
                      />
                    </Form.Item>
                    <Form.Item label="UDP over TCP">
                      <Switch checked={!!ob.settings.uot} onChange={(v) => { ob.settings.uot = v; refresh(); }} />
                    </Form.Item>
                    <Form.Item label="UoT version">
                      <InputNumber
                        value={ob.settings.UoTVersion ?? 1}
                        min={1}
                        max={2}
                        onChange={(v) => { ob.settings.UoTVersion = Number(v) || 1; refresh(); }}
                      />
                    </Form.Item>
                  </>
                )}

                {ob.hasUsername() && (
                  <>
                    <Form.Item label={t('username')}>
                      <Input
                        value={ob.settings.user || ''}
                        onChange={(e) => { ob.settings.user = e.target.value; refresh(); }}
                      />
                    </Form.Item>
                    <Form.Item label={t('password')}>
                      <Input
                        value={ob.settings.pass || ''}
                        onChange={(e) => { ob.settings.pass = e.target.value; refresh(); }}
                      />
                    </Form.Item>
                  </>
                )}

                {isHysteria && (
                  <Form.Item label="Version">
                    <InputNumber value={ob.settings.version || 2} min={2} max={2} disabled />
                  </Form.Item>
                )}

                {ob.canEnableStream() && (
                  <StreamFields ob={ob} refresh={refresh} streamNetworkChange={streamNetworkChange} isHysteria={isHysteria} t={t} />
                )}

                {ob.canEnableTls() && <TlsFields ob={ob} refresh={refresh} t={t} />}

                {ob.stream && <SockoptFields ob={ob} refresh={refresh} />}

                {ob.canEnableMux() && <MuxFields ob={ob} refresh={refresh} t={t} />}
              </Form>
              {ob.stream && ob.canEnableStream() && (
                <FinalMaskForm stream={ob.stream} protocol={proto} onChange={refresh} />
              )}
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
                  placeholder="vmess:// vless:// trojan:// ss:// hysteria2://"
                  enterButton="Convert"
                  onChange={(e) => setLinkInput(e.target.value)}
                  onSearch={convertLink}
                />
                <JsonEditor
                  value={advancedJson}
                  onChange={setAdvancedJson}
                  minHeight="360px"
                  maxHeight="600px"
                />
              </Space>
            ),
          },
        ]}
      />
      </Modal>
    </>
  );
}

/* eslint-disable @typescript-eslint/no-explicit-any */
type OB = any;

interface FieldProps {
  ob: OB;
  refresh: () => void;
}

interface TFieldProps extends FieldProps {
  t: (k: string) => string;
}

function FreedomFields({ ob, refresh }: FieldProps) {
  const fragment = (ob.settings.fragment || {}) as Record<string, string>;
  const noises = (ob.settings.noises || []) as Array<{ type: string; packet: string; delay: string; applyTo: string }>;
  const finalRules = (ob.settings.finalRules || []) as Array<{ action: string; network?: string; port?: string; ip?: string[]; blockDelay?: string }>;

  return (
    <>
      <Form.Item label="Strategy">
        <Select
          value={ob.settings.domainStrategy}
          onChange={(v) => { ob.settings.domainStrategy = v; refresh(); }}
          options={(OutboundDomainStrategies as string[]).map((s) => ({ value: s, label: s }))}
        />
      </Form.Item>
      <Form.Item label="Redirect">
        <Input
          value={ob.settings.redirect || ''}
          onChange={(e) => { ob.settings.redirect = e.target.value; refresh(); }}
        />
      </Form.Item>

      <Form.Item label="Fragment">
        <Switch
          checked={!!ob.settings.fragment && Object.keys(ob.settings.fragment).length > 0}
          onChange={(checked) => {
            ob.settings.fragment = checked
              ? { packets: 'tlshello', length: '100-200', interval: '10-20', maxSplit: '300-400' }
              : {};
            refresh();
          }}
        />
      </Form.Item>
      {ob.settings.fragment && Object.keys(ob.settings.fragment).length > 0 && (
        <>
          <Form.Item label="Packets">
            <Select
              value={fragment.packets}
              onChange={(v) => { (ob.settings.fragment as Record<string, string>).packets = v; refresh(); }}
              options={[
                { value: '1-3', label: '1-3' },
                { value: 'tlshello', label: 'tlshello' },
              ]}
            />
          </Form.Item>
          {(['length', 'interval', 'maxSplit'] as const).map((field) => (
            <Form.Item key={field} label={field === 'maxSplit' ? 'Max Split' : field[0].toUpperCase() + field.slice(1)}>
              <Input
                value={fragment[field] || ''}
                onChange={(e) => { (ob.settings.fragment as Record<string, string>)[field] = e.target.value; refresh(); }}
              />
            </Form.Item>
          ))}
        </>
      )}

      <Form.Item label="Noises">
        <Switch
          checked={noises.length > 0}
          onChange={(checked) => {
            ob.settings.noises = checked ? [new Outbound.FreedomSettings.Noise()] : [];
            refresh();
          }}
        />
        {noises.length > 0 && (
          <Button
            size="small"
            type="primary"
            className="ml-8"
            icon={<PlusOutlined />}
            onClick={() => { (ob.settings.noises as unknown[]).push(new Outbound.FreedomSettings.Noise()); refresh(); }}
          />
        )}
      </Form.Item>
      {noises.map((noise, index) => (
        <div key={index}>
          <Form.Item wrapperCol={{ md: { span: 14, offset: 8 } }}>
            <div className="item-heading">
              <span>Noise {index + 1}</span>
              {noises.length > 1 && (
                <DeleteOutlined
                  className="danger-icon"
                  onClick={() => { (ob.settings.noises as unknown[]).splice(index, 1); refresh(); }}
                />
              )}
            </div>
          </Form.Item>
          <Form.Item label="Type">
            <Select
              value={noise.type}
              onChange={(v) => { noise.type = v; refresh(); }}
              options={['rand', 'base64', 'str', 'hex'].map((x) => ({ value: x, label: x }))}
            />
          </Form.Item>
          <Form.Item label="Packet">
            <Input value={noise.packet} onChange={(e) => { noise.packet = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="Delay (ms)">
            <Input value={noise.delay} onChange={(e) => { noise.delay = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="Apply to">
            <Select
              value={noise.applyTo}
              onChange={(v) => { noise.applyTo = v; refresh(); }}
              options={['ip', 'ipv4', 'ipv6'].map((x) => ({ value: x, label: x }))}
            />
          </Form.Item>
        </div>
      ))}

      <Form.Item label="Final Rules">
        <Button
          size="small"
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => { ob.settings.addFinalRule('allow'); refresh(); }}
        />
        <span className="ml-8" style={{ opacity: 0.6 }}>
          Override Xray&apos;s default private-IP block (needed for LAN access through proxy)
        </span>
      </Form.Item>
      {finalRules.map((rule, index) => (
        <div key={`fr-${index}`}>
          <Form.Item wrapperCol={{ md: { span: 14, offset: 8 } }}>
            <div className="item-heading">
              <span>Rule {index + 1}</span>
              <DeleteOutlined
                className="danger-icon"
                onClick={() => { ob.settings.delFinalRule(index); refresh(); }}
              />
            </div>
          </Form.Item>
          <Form.Item label="Action">
            <Select
              value={rule.action}
              onChange={(v) => { rule.action = v; refresh(); }}
              options={['allow', 'block'].map((x) => ({ value: x, label: x }))}
            />
          </Form.Item>
          <Form.Item label="Network">
            <Select
              value={rule.network}
              allowClear
              placeholder="(any)"
              onChange={(v) => { rule.network = v; refresh(); }}
              options={['tcp', 'udp', 'tcp,udp'].map((x) => ({ value: x, label: x }))}
            />
          </Form.Item>
          <Form.Item label="Port">
            <Input
              value={rule.port}
              placeholder="e.g. 80,443 or 1000-2000"
              onChange={(e) => { rule.port = e.target.value; refresh(); }}
            />
          </Form.Item>
          <Form.Item label="IP / CIDR / geoip">
            <Select
              mode="tags"
              value={rule.ip || []}
              tokenSeparators={[',', ' ']}
              placeholder="e.g. 10.0.0.0/8, geoip:private, ext:cn.dat:cn"
              onChange={(v) => { rule.ip = v as string[]; refresh(); }}
            />
          </Form.Item>
          {rule.action === 'block' && (
            <Form.Item label="Block delay (ms)">
              <Input
                value={rule.blockDelay}
                placeholder="optional: 5000-10000"
                onChange={(e) => { rule.blockDelay = e.target.value; refresh(); }}
              />
            </Form.Item>
          )}
        </div>
      ))}
    </>
  );
}

function DnsFields({ ob, refresh, t }: TFieldProps) {
  const rules = (ob.settings.rules || []) as Array<{ action: string; qtype?: string; domain?: string }>;
  return (
    <>
      <Form.Item label="Rewrite network">
        <Select
          value={ob.settings.rewriteNetwork}
          allowClear
          placeholder="(unchanged)"
          onChange={(v) => { ob.settings.rewriteNetwork = v; refresh(); }}
          options={['udp', 'tcp'].map((x) => ({ value: x, label: x }))}
        />
      </Form.Item>
      <Form.Item label="Rewrite address">
        <Input
          value={ob.settings.rewriteAddress || ''}
          placeholder="(unchanged) e.g. 1.1.1.1"
          onChange={(e) => { ob.settings.rewriteAddress = e.target.value; refresh(); }}
        />
      </Form.Item>
      <Form.Item label="Rewrite port">
        <InputNumber
          value={ob.settings.rewritePort || undefined}
          min={0}
          max={65535}
          style={{ width: '100%' }}
          placeholder="(unchanged)"
          onChange={(v) => { ob.settings.rewritePort = Number(v) || 0; refresh(); }}
        />
      </Form.Item>
      <Form.Item label="User level">
        <InputNumber
          value={ob.settings.userLevel || 0}
          min={0}
          style={{ width: '100%' }}
          onChange={(v) => { ob.settings.userLevel = Number(v) || 0; refresh(); }}
        />
      </Form.Item>
      <Form.Item label="Rules">
        <Button
          size="small"
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => { (ob.settings.rules || (ob.settings.rules = [])).push(new Outbound.DNSRule()); refresh(); }}
        />
      </Form.Item>
      {rules.map((rule, index) => (
        <div key={index}>
          <Form.Item wrapperCol={{ md: { span: 14, offset: 8 } }}>
            <div className="item-heading">
              <span>Rule {index + 1}</span>
              <DeleteOutlined
                className="danger-icon"
                onClick={() => { (ob.settings.rules as unknown[]).splice(index, 1); refresh(); }}
              />
            </div>
          </Form.Item>
          <Form.Item label="Action">
            <Select
              value={rule.action}
              onChange={(v) => { rule.action = v; refresh(); }}
              options={(DNSRuleActions as string[]).map((a) => ({ value: a, label: a }))}
            />
          </Form.Item>
          <Form.Item label="QType">
            <Input
              value={rule.qtype}
              placeholder="1,3,23-24"
              onChange={(e) => { rule.qtype = e.target.value; refresh(); }}
            />
          </Form.Item>
          <Form.Item label={t('domainName')}>
            <Input
              value={rule.domain}
              placeholder="domain:example.com"
              onChange={(e) => { rule.domain = e.target.value; refresh(); }}
            />
          </Form.Item>
        </div>
      ))}
    </>
  );
}

function WireguardFields({ ob, refresh, regenerate, t }: TFieldProps & { regenerate: () => void }) {
  const peers = (ob.settings.peers || []) as Array<{ endpoint?: string; publicKey?: string; psk?: string; allowedIPs?: string[]; keepAlive?: number }>;
  return (
    <>
      <Form.Item label={t('pages.inbounds.address')}>
        <Input
          value={ob.settings.address || ''}
          onChange={(e) => { ob.settings.address = e.target.value; refresh(); }}
        />
      </Form.Item>
      <Form.Item
        label={
          <>
            {t('pages.inbounds.privatekey')}
            <SyncOutlined className="random-icon" onClick={regenerate} />
          </>
        }
      >
        <Input
          value={ob.settings.secretKey || ''}
          onChange={(e) => { ob.settings.secretKey = e.target.value; refresh(); }}
        />
      </Form.Item>
      <Form.Item label={t('pages.inbounds.publicKey')}>
        <Input value={ob.settings.pubKey || ''} disabled />
      </Form.Item>
      <Form.Item label="Domain strategy">
        <Select
          value={ob.settings.domainStrategy || ''}
          onChange={(v) => { ob.settings.domainStrategy = v; refresh(); }}
          options={['', ...(WireguardDomainStrategy as string[])].map((x) => ({ value: x, label: x || `(${t('none')})` }))}
        />
      </Form.Item>
      <Form.Item label="MTU">
        <InputNumber value={ob.settings.mtu || 0} min={0} onChange={(v) => { ob.settings.mtu = Number(v) || 0; refresh(); }} />
      </Form.Item>
      <Form.Item label="Workers">
        <InputNumber value={ob.settings.workers || 0} min={0} onChange={(v) => { ob.settings.workers = Number(v) || 0; refresh(); }} />
      </Form.Item>
      <Form.Item label="No-kernel TUN">
        <Switch checked={!!ob.settings.noKernelTun} onChange={(v) => { ob.settings.noKernelTun = v; refresh(); }} />
      </Form.Item>
      <Form.Item label="Reserved">
        <Input value={ob.settings.reserved || ''} onChange={(e) => { ob.settings.reserved = e.target.value; refresh(); }} />
      </Form.Item>
      <Form.Item label="Peers">
        <Button
          size="small"
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => { (ob.settings.peers || (ob.settings.peers = [])).push(new Outbound.WireguardSettings.Peer()); refresh(); }}
        />
      </Form.Item>
      {peers.map((peer, index) => (
        <div key={index}>
          <Form.Item wrapperCol={{ md: { span: 14, offset: 8 } }}>
            <div className="item-heading">
              <span>Peer {index + 1}</span>
              {peers.length > 1 && (
                <DeleteOutlined
                  className="danger-icon"
                  onClick={() => { (ob.settings.peers as unknown[]).splice(index, 1); refresh(); }}
                />
              )}
            </div>
          </Form.Item>
          <Form.Item label="Endpoint">
            <Input value={peer.endpoint} onChange={(e) => { peer.endpoint = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label={t('pages.inbounds.publicKey')}>
            <Input value={peer.publicKey} onChange={(e) => { peer.publicKey = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="PSK">
            <Input value={peer.psk} onChange={(e) => { peer.psk = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="Allowed IPs">
            {(peer.allowedIPs || []).map((ip, idx) => (
              <Space.Compact key={idx} block style={{ marginBottom: 4 }}>
                <Input
                  value={ip}
                  onChange={(e) => { peer.allowedIPs![idx] = e.target.value; refresh(); }}
                />
                {(peer.allowedIPs || []).length > 1 && (
                  <InputAddon onClick={() => { peer.allowedIPs!.splice(idx, 1); refresh(); }}>
                    <MinusOutlined />
                  </InputAddon>
                )}
              </Space.Compact>
            ))}
            <Button
              size="small"
              icon={<PlusOutlined />}
              onClick={() => { (peer.allowedIPs = peer.allowedIPs || []).push(''); refresh(); }}
            />
          </Form.Item>
          <Form.Item label="Keep alive">
            <InputNumber value={peer.keepAlive || 0} min={0} onChange={(v) => { peer.keepAlive = Number(v) || 0; refresh(); }} />
          </Form.Item>
        </div>
      ))}
    </>
  );
}

function VMessVLessFields({ ob, refresh, isVMess, isVLESS, t }: TFieldProps & { isVMess: boolean; isVLESS: boolean }) {
  const rev = ob.settings.reverseSniffing || {};
  return (
    <>
      <Form.Item label="ID">
        <Input value={ob.settings.id || ''} onChange={(e) => { ob.settings.id = e.target.value; refresh(); }} />
      </Form.Item>
      {isVMess && (
        <Form.Item label={t('security')}>
          <Select
            value={ob.settings.security}
            onChange={(v) => { ob.settings.security = v; refresh(); }}
            options={SECURITY_OPTIONS.map((s) => ({ value: s, label: s }))}
          />
        </Form.Item>
      )}
      {isVLESS && (
        <Form.Item label={t('encryption')}>
          <Input
            value={ob.settings.encryption || ''}
            onChange={(e) => { ob.settings.encryption = e.target.value; refresh(); }}
          />
        </Form.Item>
      )}
      {isVLESS && (
        <Form.Item label="Reverse tag">
          <Input
            value={ob.settings.reverseTag || ''}
            placeholder="optional"
            onChange={(e) => { ob.settings.reverseTag = e.target.value; refresh(); }}
          />
        </Form.Item>
      )}
      {isVLESS && ob.settings.reverseTag && (
        <>
          <Form.Item label="Reverse Sniffing">
            <Switch checked={!!rev.enabled} onChange={(v) => { rev.enabled = v; refresh(); }} />
          </Form.Item>
          {rev.enabled && (
            <>
              <Form.Item wrapperCol={{ md: { span: 14, offset: 8 } }}>
                <Checkbox.Group
                  className="sniffing-options"
                  value={rev.destOverride || []}
                  onChange={(v) => { rev.destOverride = v as string[]; refresh(); }}
                  options={Object.entries(SNIFFING_OPTION).map(([label, value]) => ({ label, value: value as string }))}
                />
              </Form.Item>
              <Form.Item label="Metadata Only">
                <Switch checked={!!rev.metadataOnly} onChange={(v) => { rev.metadataOnly = v; refresh(); }} />
              </Form.Item>
              <Form.Item label="Route Only">
                <Switch checked={!!rev.routeOnly} onChange={(v) => { rev.routeOnly = v; refresh(); }} />
              </Form.Item>
              <Form.Item label="IPs Excluded">
                <Select
                  mode="tags"
                  value={rev.ipsExcluded || []}
                  tokenSeparators={[',']}
                  placeholder="IP/CIDR/geoip:*/ext:*"
                  style={{ width: '100%' }}
                  onChange={(v) => { rev.ipsExcluded = v as string[]; refresh(); }}
                />
              </Form.Item>
              <Form.Item label="Domains Excluded">
                <Select
                  mode="tags"
                  value={rev.domainsExcluded || []}
                  tokenSeparators={[',']}
                  placeholder="domain:*/ext:*"
                  style={{ width: '100%' }}
                  onChange={(v) => { rev.domainsExcluded = v as string[]; refresh(); }}
                />
              </Form.Item>
            </>
          )}
        </>
      )}
      {ob.canEnableTlsFlow() && (
        <Form.Item label="Flow">
          <Select
            value={ob.settings.flow || ''}
            onChange={(v) => { ob.settings.flow = v; refresh(); }}
            options={[{ value: '', label: t('none') }, ...FLOW_OPTIONS.map((k) => ({ value: k, label: k }))]}
          />
        </Form.Item>
      )}
    </>
  );
}

function StreamFields({ ob, refresh, streamNetworkChange, isHysteria, t }: TFieldProps & { streamNetworkChange: (next: string) => void; isHysteria: boolean }) {
  const networks = isHysteria ? [...NETWORKS, 'hysteria'] : NETWORKS;
  return (
    <>
      <Form.Item label={t('transmission')}>
        <Select
          value={ob.stream.network}
          onChange={streamNetworkChange}
          options={networks.map((net) => ({ value: net, label: NETWORK_LABELS[net] || net }))}
        />
      </Form.Item>

      {ob.stream.network === 'tcp' && (
        <>
          <Form.Item label={`HTTP ${t('camouflage')}`}>
            <Switch
              checked={ob.stream.tcp.type === 'http'}
              onChange={(checked) => { ob.stream.tcp.type = checked ? 'http' : 'none'; refresh(); }}
            />
          </Form.Item>
          {ob.stream.tcp.type === 'http' && (
            <>
              <Form.Item label={t('host')}>
                <Input value={ob.stream.tcp.host || ''} onChange={(e) => { ob.stream.tcp.host = e.target.value; refresh(); }} />
              </Form.Item>
              <Form.Item label={t('path')}>
                <Input value={ob.stream.tcp.path || ''} onChange={(e) => { ob.stream.tcp.path = e.target.value; refresh(); }} />
              </Form.Item>
            </>
          )}
        </>
      )}

      {ob.stream.network === 'kcp' && (
        <>
          {(
            [
              ['mtu', 'MTU', 0],
              ['tti', 'TTI (ms)', 0],
              ['upCap', 'Uplink (MB/s)', 0],
              ['downCap', 'Downlink (MB/s)', 0],
              ['cwndMultiplier', 'CWND multiplier', 1],
              ['maxSendingWindow', 'Max sending window', 0],
            ] as const
          ).map(([field, label, min]) => (
            <Form.Item key={field} label={label}>
              <InputNumber
                value={ob.stream.kcp[field] ?? 0}
                min={min}
                onChange={(v) => { ob.stream.kcp[field] = Number(v) || 0; refresh(); }}
              />
            </Form.Item>
          ))}
        </>
      )}

      {ob.stream.network === 'ws' && (
        <>
          <Form.Item label={t('host')}>
            <Input value={ob.stream.ws.host || ''} onChange={(e) => { ob.stream.ws.host = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label={t('path')}>
            <Input value={ob.stream.ws.path || ''} onChange={(e) => { ob.stream.ws.path = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="Heartbeat (s)">
            <InputNumber
              value={ob.stream.ws.heartbeatPeriod || 0}
              min={0}
              onChange={(v) => { ob.stream.ws.heartbeatPeriod = Number(v) || 0; refresh(); }}
            />
          </Form.Item>
        </>
      )}

      {ob.stream.network === 'grpc' && (
        <>
          <Form.Item label="Service name">
            <Input value={ob.stream.grpc.serviceName || ''} onChange={(e) => { ob.stream.grpc.serviceName = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="Authority">
            <Input value={ob.stream.grpc.authority || ''} onChange={(e) => { ob.stream.grpc.authority = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="Multi mode">
            <Switch checked={!!ob.stream.grpc.multiMode} onChange={(v) => { ob.stream.grpc.multiMode = v; refresh(); }} />
          </Form.Item>
        </>
      )}

      {ob.stream.network === 'httpupgrade' && (
        <>
          <Form.Item label={t('host')}>
            <Input value={ob.stream.httpupgrade.host || ''} onChange={(e) => { ob.stream.httpupgrade.host = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label={t('path')}>
            <Input value={ob.stream.httpupgrade.path || ''} onChange={(e) => { ob.stream.httpupgrade.path = e.target.value; refresh(); }} />
          </Form.Item>
        </>
      )}

      {ob.stream.network === 'xhttp' && <XhttpFields ob={ob} refresh={refresh} t={t} />}

      {ob.stream.network === 'hysteria' && <HysteriaTransportFields ob={ob} refresh={refresh} />}
    </>
  );
}

function XhttpFields({ ob, refresh, t }: TFieldProps) {
  const xh = ob.stream.xhttp;
  return (
    <>
      <Form.Item label={t('host')}>
        <Input value={xh.host || ''} onChange={(e) => { xh.host = e.target.value; refresh(); }} />
      </Form.Item>
      <Form.Item label={t('path')}>
        <Input value={xh.path || ''} onChange={(e) => { xh.path = e.target.value; refresh(); }} />
      </Form.Item>
      <Form.Item label={t('pages.inbounds.stream.tcp.requestHeader')}>
        <Button size="small" icon={<PlusOutlined />} onClick={() => { xh.addHeader('', ''); refresh(); }} />
      </Form.Item>
      <Form.Item wrapperCol={{ span: 24 }}>
        {(xh.headers as Array<{ name: string; value: string }>).map((header, idx) => (
          <Space.Compact key={idx} block className="mb-8">
            <InputAddon>{`${idx + 1}`}</InputAddon>
            <Input
              value={header.name}
              placeholder="Name"
              onChange={(e) => { header.name = e.target.value; refresh(); }}
            />
            <Input
              value={header.value}
              placeholder="Value"
              onChange={(e) => { header.value = e.target.value; refresh(); }}
            />
            <Button icon={<MinusOutlined />} onClick={() => { xh.removeHeader(idx); refresh(); }} />
          </Space.Compact>
        ))}
      </Form.Item>

      <Form.Item label="Mode">
        <Select
          value={xh.mode}
          onChange={(v) => { xh.mode = v; refresh(); }}
          options={Object.values(MODE_OPTION).map((m) => ({ value: m as string, label: m as string }))}
        />
      </Form.Item>
      {xh.mode === 'packet-up' && (
        <>
          <Form.Item label="Max Upload Size (Byte)">
            <Input value={xh.scMaxEachPostBytes} onChange={(e) => { xh.scMaxEachPostBytes = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="Min Upload Interval (Ms)">
            <Input value={xh.scMinPostsIntervalMs} onChange={(e) => { xh.scMinPostsIntervalMs = e.target.value; refresh(); }} />
          </Form.Item>
        </>
      )}

      <Form.Item label="Padding Bytes">
        <Input value={xh.xPaddingBytes} onChange={(e) => { xh.xPaddingBytes = e.target.value; refresh(); }} />
      </Form.Item>
      <Form.Item label="Padding Obfs Mode">
        <Switch checked={!!xh.xPaddingObfsMode} onChange={(v) => { xh.xPaddingObfsMode = v; refresh(); }} />
      </Form.Item>
      {xh.xPaddingObfsMode && (
        <>
          <Form.Item label="Padding Key">
            <Input value={xh.xPaddingKey} placeholder="x_padding" onChange={(e) => { xh.xPaddingKey = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="Padding Header">
            <Input value={xh.xPaddingHeader} placeholder="X-Padding" onChange={(e) => { xh.xPaddingHeader = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="Padding Placement">
            <Select
              value={xh.xPaddingPlacement || ''}
              onChange={(v) => { xh.xPaddingPlacement = v; refresh(); }}
              options={[
                { value: '', label: 'Default (queryInHeader)' },
                { value: 'queryInHeader', label: 'queryInHeader' },
                { value: 'header', label: 'header' },
                { value: 'cookie', label: 'cookie' },
                { value: 'query', label: 'query' },
              ]}
            />
          </Form.Item>
          <Form.Item label="Padding Method">
            <Select
              value={xh.xPaddingMethod || ''}
              onChange={(v) => { xh.xPaddingMethod = v; refresh(); }}
              options={[
                { value: '', label: 'Default (repeat-x)' },
                { value: 'repeat-x', label: 'repeat-x' },
                { value: 'tokenish', label: 'tokenish' },
              ]}
            />
          </Form.Item>
        </>
      )}

      <Form.Item label="Uplink HTTP Method">
        <Select
          value={xh.uplinkHTTPMethod || ''}
          onChange={(v) => { xh.uplinkHTTPMethod = v; refresh(); }}
          options={[
            { value: '', label: 'Default (POST)' },
            { value: 'POST', label: 'POST' },
            { value: 'PUT', label: 'PUT' },
            { value: 'GET', label: 'GET (packet-up only)', disabled: xh.mode !== 'packet-up' },
          ]}
        />
      </Form.Item>

      <Form.Item label="Session Placement">
        <Select
          value={xh.sessionPlacement || ''}
          onChange={(v) => { xh.sessionPlacement = v; refresh(); }}
          options={[
            { value: '', label: 'Default (path)' },
            { value: 'path', label: 'path' },
            { value: 'header', label: 'header' },
            { value: 'cookie', label: 'cookie' },
            { value: 'query', label: 'query' },
          ]}
        />
      </Form.Item>
      {xh.sessionPlacement && xh.sessionPlacement !== 'path' && (
        <Form.Item label="Session Key">
          <Input value={xh.sessionKey} placeholder="x_session" onChange={(e) => { xh.sessionKey = e.target.value; refresh(); }} />
        </Form.Item>
      )}

      <Form.Item label="Sequence Placement">
        <Select
          value={xh.seqPlacement || ''}
          onChange={(v) => { xh.seqPlacement = v; refresh(); }}
          options={[
            { value: '', label: 'Default (path)' },
            { value: 'path', label: 'path' },
            { value: 'header', label: 'header' },
            { value: 'cookie', label: 'cookie' },
            { value: 'query', label: 'query' },
          ]}
        />
      </Form.Item>
      {xh.seqPlacement && xh.seqPlacement !== 'path' && (
        <Form.Item label="Sequence Key">
          <Input value={xh.seqKey} placeholder="x_seq" onChange={(e) => { xh.seqKey = e.target.value; refresh(); }} />
        </Form.Item>
      )}

      {xh.mode === 'packet-up' && (
        <Form.Item label="Uplink Data Placement">
          <Select
            value={xh.uplinkDataPlacement || ''}
            onChange={(v) => { xh.uplinkDataPlacement = v; refresh(); }}
            options={[
              { value: '', label: 'Default (body)' },
              { value: 'body', label: 'body' },
              { value: 'header', label: 'header' },
              { value: 'cookie', label: 'cookie' },
              { value: 'query', label: 'query' },
            ]}
          />
        </Form.Item>
      )}
      {xh.mode === 'packet-up' && xh.uplinkDataPlacement && xh.uplinkDataPlacement !== 'body' && (
        <>
          <Form.Item label="Uplink Data Key">
            <Input value={xh.uplinkDataKey} placeholder="x_data" onChange={(e) => { xh.uplinkDataKey = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="Uplink Chunk Size">
            <InputNumber
              value={xh.uplinkChunkSize}
              min={0}
              placeholder="0 (unlimited)"
              onChange={(v) => { xh.uplinkChunkSize = Number(v) || 0; refresh(); }}
            />
          </Form.Item>
        </>
      )}

      {(xh.mode === 'stream-up' || xh.mode === 'stream-one') && (
        <Form.Item label="No gRPC Header">
          <Switch checked={!!xh.noGRPCHeader} onChange={(v) => { xh.noGRPCHeader = v; refresh(); }} />
        </Form.Item>
      )}

      <Form.Item label="XMUX">
        <Switch checked={!!xh.enableXmux} onChange={(v) => { xh.enableXmux = v; refresh(); }} />
      </Form.Item>
      {xh.enableXmux && (
        <>
          {!xh.xmux.maxConnections && (
            <Form.Item label="Max Concurrency">
              <Input value={xh.xmux.maxConcurrency} onChange={(e) => { xh.xmux.maxConcurrency = e.target.value; refresh(); }} />
            </Form.Item>
          )}
          {!xh.xmux.maxConcurrency && (
            <Form.Item label="Max Connections">
              <Input value={xh.xmux.maxConnections} onChange={(e) => { xh.xmux.maxConnections = e.target.value; refresh(); }} />
            </Form.Item>
          )}
          <Form.Item label="Max Reuse Times">
            <Input value={xh.xmux.cMaxReuseTimes} onChange={(e) => { xh.xmux.cMaxReuseTimes = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="Max Request Times">
            <Input value={xh.xmux.hMaxRequestTimes} onChange={(e) => { xh.xmux.hMaxRequestTimes = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="Max Reusable Secs">
            <Input value={xh.xmux.hMaxReusableSecs} onChange={(e) => { xh.xmux.hMaxReusableSecs = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="Keep Alive Period">
            <InputNumber
              value={xh.xmux.hKeepAlivePeriod}
              min={0}
              onChange={(v) => { xh.xmux.hKeepAlivePeriod = Number(v) || 0; refresh(); }}
            />
          </Form.Item>
        </>
      )}
    </>
  );
}

function HysteriaTransportFields({ ob, refresh }: FieldProps) {
  const h = ob.stream.hysteria;
  return (
    <>
      <Form.Item label="Auth password">
        <Input value={h.auth || ''} onChange={(e) => { h.auth = e.target.value; refresh(); }} />
      </Form.Item>
      <Form.Item label="Congestion">
        <Select
          value={h.congestion || ''}
          onChange={(v) => { h.congestion = v; refresh(); }}
          options={[
            { value: '', label: 'BBR (auto)' },
            { value: 'brutal', label: 'Brutal' },
          ]}
        />
      </Form.Item>
      <Form.Item label="Upload">
        <Input value={h.up} placeholder="100 mbps" onChange={(e) => { h.up = e.target.value; refresh(); }} />
      </Form.Item>
      <Form.Item label="Download">
        <Input value={h.down} placeholder="100 mbps" onChange={(e) => { h.down = e.target.value; refresh(); }} />
      </Form.Item>
      <Form.Item label="UDP hop port">
        <Input value={h.udphopPort} placeholder="1145-1919" onChange={(e) => { h.udphopPort = e.target.value; refresh(); }} />
      </Form.Item>
      <Form.Item label="Max idle (s)">
        <InputNumber value={h.maxIdleTimeout} min={4} max={120} onChange={(v) => { h.maxIdleTimeout = Number(v) || 0; refresh(); }} />
      </Form.Item>
      <Form.Item label="Keep alive (s)">
        <InputNumber value={h.keepAlivePeriod} min={2} max={60} onChange={(v) => { h.keepAlivePeriod = Number(v) || 0; refresh(); }} />
      </Form.Item>
      <Form.Item label="Disable Path MTU">
        <Switch checked={!!h.disablePathMTUDiscovery} onChange={(v) => { h.disablePathMTUDiscovery = v; refresh(); }} />
      </Form.Item>
    </>
  );
}

function TlsFields({ ob, refresh, t }: TFieldProps) {
  return (
    <>
      <Form.Item label={t('security')}>
        <Radio.Group
          value={ob.stream.security}
          buttonStyle="solid"
          onChange={(e) => { ob.stream.security = e.target.value; refresh(); }}
        >
          <Radio.Button value="none">{t('none')}</Radio.Button>
          <Radio.Button value="tls">TLS</Radio.Button>
          {ob.canEnableReality() && <Radio.Button value="reality">Reality</Radio.Button>}
        </Radio.Group>
      </Form.Item>

      {ob.stream.isTls && (
        <>
          <Form.Item label="SNI">
            <Input value={ob.stream.tls.serverName} placeholder="server name" onChange={(e) => { ob.stream.tls.serverName = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="uTLS">
            <Select
              value={ob.stream.tls.fingerprint || ''}
              onChange={(v) => { ob.stream.tls.fingerprint = v; refresh(); }}
              options={[{ value: '', label: t('none') }, ...UTLS_OPTIONS.map((k) => ({ value: k, label: k }))]}
            />
          </Form.Item>
          <Form.Item label="ALPN">
            <Select
              mode="multiple"
              value={ob.stream.tls.alpn || []}
              onChange={(v) => { ob.stream.tls.alpn = v; refresh(); }}
              options={ALPN_OPTIONS.map((alpn) => ({ value: alpn, label: alpn }))}
            />
          </Form.Item>
          <Form.Item label="ECH">
            <Input value={ob.stream.tls.echConfigList} onChange={(e) => { ob.stream.tls.echConfigList = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="Verify peer name">
            <Input value={ob.stream.tls.verifyPeerCertByName} placeholder="cloudflare-dns.com" onChange={(e) => { ob.stream.tls.verifyPeerCertByName = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="Pinned SHA256">
            <Input value={ob.stream.tls.pinnedPeerCertSha256} placeholder="base64 SHA256" onChange={(e) => { ob.stream.tls.pinnedPeerCertSha256 = e.target.value; refresh(); }} />
          </Form.Item>
        </>
      )}

      {ob.stream.isReality && (
        <>
          <Form.Item label="SNI">
            <Input value={ob.stream.reality.serverName} onChange={(e) => { ob.stream.reality.serverName = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="uTLS">
            <Select
              value={ob.stream.reality.fingerprint}
              onChange={(v) => { ob.stream.reality.fingerprint = v; refresh(); }}
              options={UTLS_OPTIONS.map((k) => ({ value: k, label: k }))}
            />
          </Form.Item>
          <Form.Item label="Short ID">
            <Input value={ob.stream.reality.shortId} onChange={(e) => { ob.stream.reality.shortId = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="SpiderX">
            <Input value={ob.stream.reality.spiderX} onChange={(e) => { ob.stream.reality.spiderX = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label={t('pages.inbounds.publicKey')}>
            <Input.TextArea
              value={ob.stream.reality.publicKey}
              autoSize={{ minRows: 2 }}
              onChange={(e) => { ob.stream.reality.publicKey = e.target.value; refresh(); }}
            />
          </Form.Item>
          <Form.Item label="mldsa65 verify">
            <Input.TextArea
              value={ob.stream.reality.mldsa65Verify}
              autoSize={{ minRows: 2 }}
              onChange={(e) => { ob.stream.reality.mldsa65Verify = e.target.value; refresh(); }}
            />
          </Form.Item>
        </>
      )}
    </>
  );
}

function SockoptFields({ ob, refresh }: FieldProps) {
  return (
    <>
      <Form.Item label="Sockopts">
        <Switch checked={!!ob.stream.sockoptSwitch} onChange={(v) => { ob.stream.sockoptSwitch = v; refresh(); }} />
      </Form.Item>
      {ob.stream.sockoptSwitch && (
        <>
          <Form.Item label="Dialer proxy">
            <Input value={ob.stream.sockopt.dialerProxy || ''} onChange={(e) => { ob.stream.sockopt.dialerProxy = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="Address+Port strategy">
            <Select
              value={ob.stream.sockopt.addressPortStrategy}
              onChange={(v) => { ob.stream.sockopt.addressPortStrategy = v; refresh(); }}
              options={Object.values(Address_Port_Strategy).map((k) => ({ value: k as string, label: k as string }))}
            />
          </Form.Item>
          <Form.Item label="Keep alive interval">
            <InputNumber
              value={ob.stream.sockopt.tcpKeepAliveInterval}
              min={0}
              onChange={(v) => { ob.stream.sockopt.tcpKeepAliveInterval = Number(v) || 0; refresh(); }}
            />
          </Form.Item>
          <Form.Item label="TCP Fast Open">
            <Switch checked={!!ob.stream.sockopt.tcpFastOpen} onChange={(v) => { ob.stream.sockopt.tcpFastOpen = v; refresh(); }} />
          </Form.Item>
          <Form.Item label="Multipath TCP">
            <Switch checked={!!ob.stream.sockopt.tcpMptcp} onChange={(v) => { ob.stream.sockopt.tcpMptcp = v; refresh(); }} />
          </Form.Item>
          <Form.Item label="Penetrate">
            <Switch checked={!!ob.stream.sockopt.penetrate} onChange={(v) => { ob.stream.sockopt.penetrate = v; refresh(); }} />
          </Form.Item>
          <Form.Item label="Mark (fwmark)">
            <InputNumber
              value={ob.stream.sockopt.mark}
              min={0}
              onChange={(v) => { ob.stream.sockopt.mark = Number(v) || 0; refresh(); }}
            />
          </Form.Item>
          <Form.Item label="Interface">
            <Input value={ob.stream.sockopt.interfaceName} onChange={(e) => { ob.stream.sockopt.interfaceName = e.target.value; refresh(); }} />
          </Form.Item>
        </>
      )}
    </>
  );
}

function MuxFields({ ob, refresh, t }: TFieldProps) {
  return (
    <>
      <Form.Item label={t('pages.settings.mux')}>
        <Switch checked={!!ob.mux.enabled} onChange={(v) => { ob.mux.enabled = v; refresh(); }} />
      </Form.Item>
      {ob.mux.enabled && (
        <>
          <Form.Item label="Concurrency">
            <InputNumber
              value={ob.mux.concurrency}
              min={-1}
              max={1024}
              onChange={(v) => { ob.mux.concurrency = Number(v) || 0; refresh(); }}
            />
          </Form.Item>
          <Form.Item label="xudp concurrency">
            <InputNumber
              value={ob.mux.xudpConcurrency}
              min={-1}
              max={1024}
              onChange={(v) => { ob.mux.xudpConcurrency = Number(v) || 0; refresh(); }}
            />
          </Form.Item>
          <Form.Item label="xudp UDP 443">
            <Select
              value={ob.mux.xudpProxyUDP443}
              onChange={(v) => { ob.mux.xudpProxyUDP443 = v; refresh(); }}
              options={['reject', 'allow', 'skip'].map((x) => ({ value: x, label: x }))}
            />
          </Form.Item>
        </>
      )}
    </>
  );
}
