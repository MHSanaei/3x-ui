import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import {
  Form,
  Input,
  InputNumber,
  Modal,
  Radio,
  Select,
  Switch,
  Tabs,
  Tooltip,
  message,
} from 'antd';

import { HttpUtil, NumberFormatter, RandomUtil, SizeFormatter, Wireguard } from '@/utils';
import {
  rawInboundToFormValues,
  formValuesToWirePayload,
} from '@/lib/xray/inbound-form-adapter';
import { createDefaultInboundSettings } from '@/lib/xray/inbound-defaults';
import { composeInboundTag, isAutoInboundTag, type InboundTagInput } from '@/lib/xray/inbound-tag';
import {
  canEnableReality,
  canEnableStream,
  canEnableTls,
  isSS2022,
} from '@/lib/xray/protocol-capabilities';
import {
  InboundFormBaseSchema,
  InboundFormSchema,
  type InboundFormValues,
} from '@/schemas/forms/inbound-form';
import { antdRule } from '@/utils/zodForm';
import { Protocols } from '@/schemas/primitives';
import { SockoptStreamSettingsSchema } from '@/schemas/protocols/stream/sockopt';
import { HysteriaStreamSettingsSchema } from '@/schemas/protocols/stream/hysteria';
import { TlsStreamSettingsSchema } from '@/schemas/protocols/security/tls';
import { SniffingSchema } from '@/schemas/primitives/sniffing';
import { TcpStreamSettingsSchema } from '@/schemas/protocols/stream/tcp';
import { KcpStreamSettingsSchema } from '@/schemas/protocols/stream/kcp';
import { WsStreamSettingsSchema } from '@/schemas/protocols/stream/ws';
import { GrpcStreamSettingsSchema } from '@/schemas/protocols/stream/grpc';
import { HttpUpgradeStreamSettingsSchema } from '@/schemas/protocols/stream/httpupgrade';
import { XHttpStreamSettingsSchema } from '@/schemas/protocols/stream/xhttp';
import { DateTimePicker } from '@/components/form';
import { FinalMaskForm } from '@/lib/xray/forms/transport';
import './InboundFormModal.css';

import { AdvancedAllEditor, AdvancedSliceEditor } from './advanced-editors';
import { formatInboundIssue, formatInboundValidation } from './formatValidationError';
import {
  HttpFields,
  HysteriaFields,
  MixedFields,
  ShadowsocksFields,
  TunFields,
  TunnelFields,
  VlessFields,
  WireguardFields,
} from './protocols';
import {
  ExternalProxyForm,
  GrpcForm,
  HttpUpgradeForm,
  KcpForm,
  RawForm,
  SockoptForm,
  WsForm,
  XhttpForm,
} from './transport';
import { RealityForm, TlsForm } from './security';
import { useSecurityActions } from './useSecurityActions';
import { useInboundFallbacks } from './useInboundFallbacks';
import FallbacksCard from './FallbacksCard';
import SniffingTab from './SniffingTab';

import type { DBInbound } from '@/models/dbinbound';
import type { NodeRecord } from '@/api/queries/useNodesQuery';


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
    streamSettings: {
      network: 'tcp',
      security: 'none',
      tcpSettings: TcpStreamSettingsSchema.parse({ header: { type: 'none' } }),
    },
    sniffing: SniffingSchema.parse({}),
    port: RandomUtil.randomInteger(10000, 60000),
    listen: '',
    tag: '',
    enable: true,
    trafficReset: 'never',
  });
}

export default function InboundFormModal({
  open,
  onClose,
  onSaved,
  mode,
  dbInbound,
  dbInbounds,
  availableNodes,
}: InboundFormModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [form] = Form.useForm<InboundFormValues>();
  const [saving, setSaving] = useState(false);
  const {
    fallbacks,
    fallbackChildOptions,
    loadFallbacks,
    saveFallbacks,
    addFallback,
    updateFallback,
    removeFallback,
    moveFallback,
    addAllFallbacks,
  } = useInboundFallbacks(dbInbound, dbInbounds);

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

  const wPort = Form.useWatch('port', form);
  const wListen = (Form.useWatch('listen', form) ?? '') as string;
  const isUdsListen = wListen.startsWith('/');
  const wNodeId = Form.useWatch('nodeId', form) ?? null;
  const wTag = Form.useWatch('tag', form) ?? '';
  const wSsNetwork = Form.useWatch(['settings', 'network'], form);
  const wTunnelNetwork = Form.useWatch(['settings', 'allowedNetwork'], form);
  const autoTagRef = useRef(true);
  const lastWrittenTagRef = useRef('');
  const currentTagInput = (): InboundTagInput => ({
    port: typeof wPort === 'number' ? wPort : 0,
    nodeId: typeof wNodeId === 'number' ? wNodeId : null,
    protocol,
    streamSettings: { network },
    settings: { network: wSsNetwork, allowedNetwork: wTunnelNetwork, udp: mixedUdpOn },
  });
  const isFallbackHost =
    (protocol === Protocols.VLESS || protocol === Protocols.TROJAN)
    && network === 'tcp'
    && (security === 'tls' || security === 'reality');

  const {
    genRealityKeypair,
    clearRealityKeypair,
    genMldsa65,
    clearMldsa65,
    randomizeRealityTarget,
    randomizeShortIds,
    getNewEchCert,
    clearEchCert,
    generateRandomPinHash,
    setCertFromPanel,
    clearCertFiles,
    onSecurityChange,
  } = useSecurityActions({ form, setSaving, messageApi });

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
    const initialTag = (initial.tag ?? '') as string;
    autoTagRef.current = isAutoInboundTag(initialTag, {
      port: initial.port ?? 0,
      nodeId: initial.nodeId ?? null,
      protocol: initial.protocol,
      streamSettings: (initial.streamSettings ?? {}) as Record<string, unknown>,
      settings: (initial.settings ?? {}) as Record<string, unknown>,
    });
    lastWrittenTagRef.current = initialTag;
    if (
      mode === 'edit'
      && dbInbound
      && (dbInbound.protocol === Protocols.VLESS || dbInbound.protocol === Protocols.TROJAN)
    ) {
      loadFallbacks(dbInbound.id);
    } else {
      loadFallbacks(null);
    }

    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, mode, dbInbound, form]);

  useEffect(() => {
    if (!open) return;
    if (wTag === lastWrittenTagRef.current) return;
    autoTagRef.current = isAutoInboundTag(wTag, currentTagInput());
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, wTag]);

  useEffect(() => {
    if (!open || !autoTagRef.current) return;
    const next = composeInboundTag(currentTagInput());
    if (next !== (form.getFieldValue('tag') ?? '')) {
      lastWrittenTagRef.current = next;
      form.setFieldValue('tag', next);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, wPort, wNodeId, protocol, network, mixedUdpOn, wSsNetwork, wTunnelNetwork]);

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
      // Hysteria uses its dedicated transport — force the network branch
      // so the stream tab renders the hysteria sub-form, not the leftover
      // tcpSettings from the previous protocol. When leaving hysteria,
      // snap back to TCP so the standard network selector has a valid
      // starting point.
      if (next === Protocols.HYSTERIA) {
        const tls = TlsStreamSettingsSchema.parse({}) as Record<string, unknown>;
        tls.certificates = [{
          useFile: true,
          certificateFile: '',
          keyFile: '',
          certificate: [],
          key: [],
          oneTimeLoading: false,
          usage: 'encipherment',
          buildChain: false,
        }];
        form.setFieldValue('streamSettings', {
          network: 'hysteria',
          security: 'tls',
          hysteriaSettings: HysteriaStreamSettingsSchema.parse({}),
          tlsSettings: tls,
          // Hysteria2 needs an obfs wrapper on the FinalMask side; seed
          // it with salamander + a random password so the listener boots
          // with a usable default. Re-selecting Hysteria from another
          // protocol re-runs this and refreshes the password — that's
          // intentional, the form was already being reset.
          finalmask: {
            tcp: [],
            udp: [{
              type: 'salamander',
              settings: { password: RandomUtil.randomLowerAndNum(16) },
            }],
          },
        });
      } else {
        const current = form.getFieldValue('streamSettings') as { network?: string } | undefined;
        if (current?.network === 'hysteria') {
          form.setFieldValue('streamSettings', { network: 'tcp', security: 'none', tcpSettings: {} });
        }
      }
    }
  };

  const submit = async () => {
    try {
      await form.validateFields();
    } catch {
      return;
    }
    // Why getFieldsValue(true) instead of the validateFields return value:
    // rc-component/form's validateFields filters its output by REGISTERED
    // name paths. settings.clients and settings.fallbacks have no Form.Item
    // bound to them (clients are managed via the standalone Client modal,
    // not inside this inbound modal) — so validateFields would drop them
    // and the update wire payload would silently delete every client on
    // every save. getFieldsValue(true) returns the entire form store and
    // keeps those sub-trees intact.
    const values = form.getFieldsValue(true) as InboundFormValues;
    const parsed = InboundFormSchema.safeParse(values);
    if (!parsed.success) {
      const issues = parsed.error.issues;
      messageApi.error(formatInboundValidation(issues, values, t));
      console.error(
        '[InboundFormModal] schema validation failed:',
        issues.map((issue) => formatInboundIssue(issue, values, t)),
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
        if (isFallbackHost) {
          const obj = msg.obj as { id?: number; Id?: number } | null;
          const masterId = mode === 'edit'
            ? dbInbound!.id
            : (obj?.id ?? obj?.Id ?? 0);
          if (masterId) await saveFallbacks(masterId);
        }
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
      <Form.Item name="tag" hidden noStyle><Input /></Form.Item>
      <Form.Item name="up" hidden noStyle><InputNumber /></Form.Item>
      <Form.Item name="down" hidden noStyle><InputNumber /></Form.Item>
      <Form.Item name="total" hidden noStyle><InputNumber /></Form.Item>
      <Form.Item name="expiryTime" hidden noStyle><InputNumber /></Form.Item>
      <Form.Item name="lastTrafficResetTime" hidden noStyle><InputNumber /></Form.Item>
      <Form.Item name="clientStats" hidden noStyle><Input /></Form.Item>

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
            options={selectableNodes.map((n) => ({
              value: n.id,
              label: `${n.name}${n.status === 'offline' ? ' (offline)' : ''}`,
              disabled: n.status === 'offline',
            }))}
          />
        </Form.Item>
      )}

      <Form.Item name="protocol" label={t('pages.inbounds.protocol')}>
        <Select disabled={mode === 'edit'} options={PROTOCOL_OPTIONS} />
      </Form.Item>

      <Form.Item
        name="listen"
        label={t('pages.inbounds.address')}
        extra={t('pages.inbounds.form.listenHelp')}
      >
        <Input placeholder={t('pages.inbounds.monitorDesc')} />
      </Form.Item>

      <Form.Item
        name="port"
        label={t('pages.inbounds.port')}
        rules={[antdRule(InboundFormBaseSchema.shape.port, t)]}
      >
        <InputNumber min={isUdsListen ? 0 : 1} max={65535} />
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
        <Select
          options={TRAFFIC_RESETS.map((r) => ({
            value: r,
            label: t(`pages.inbounds.periodicTrafficReset.${r}`),
          }))}
        />
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

  const fallbacksCard = (
    <FallbacksCard
      fallbacks={fallbacks}
      fallbackChildOptions={fallbackChildOptions}
      addFallback={addFallback}
      updateFallback={updateFallback}
      removeFallback={removeFallback}
      moveFallback={moveFallback}
      addAllFallbacks={addAllFallbacks}
    />
  );

  const protocolTab = (
    <>
      {protocol === Protocols.WIREGUARD && <WireguardFields wgPubKey={wgPubKey} regenInboundWg={regenInboundWg} regenWgPeerKeypair={regenWgPeerKeypair} />}

      {protocol === Protocols.TUN && <TunFields />}

      {protocol === Protocols.TUNNEL && <TunnelFields />}

      {protocol === Protocols.HTTP && <HttpFields />}
      {protocol === Protocols.MIXED && <MixedFields mixedUdpOn={mixedUdpOn} />}

      {protocol === Protocols.SHADOWSOCKS && <ShadowsocksFields form={form} isSSWith2022={isSSWith2022} />}

      {protocol === Protocols.VLESS && <VlessFields saving={saving} selectedVlessAuth={selectedVlessAuth} network={network} security={security} getNewVlessEnc={getNewVlessEnc} clearVlessEnc={clearVlessEnc} />}

      {isFallbackHost && fallbacksCard}
    </>
  );

  // Switching `network` swaps which per-network key (tcpSettings,
  // wsSettings, grpcSettings, ...) appears on the wire. Clear the old
  // network's blob and seed the new one with the schema defaults so the
  // Form.Items inside it have valid initial values (KCP needs MTU=1350
  // etc., not empty strings).
  // Seed each network's settings blob with its Zod schema defaults so
  // every Form.Item inside the network sub-form has a defined starting
  // value. XHTTP in particular has ~20 fields (sessionPlacement,
  // seqPlacement, xPaddingMethod, uplinkHTTPMethod, ...) whose value
  // is the literal "" sentinel meaning "let xray-core pick its
  // default". Without seeding "", the Form.Item reads `undefined` and
  // the Select shows blank instead of the "Default (path)" option.
  const newStreamSlice = (n: string): Record<string, unknown> => {
    switch (n) {
      case 'tcp': return TcpStreamSettingsSchema.parse({ header: { type: 'none' } });
      case 'kcp': return KcpStreamSettingsSchema.parse({});
      case 'ws': return WsStreamSettingsSchema.parse({});
      case 'grpc': return GrpcStreamSettingsSchema.parse({});
      case 'httpupgrade': return HttpUpgradeStreamSettingsSchema.parse({});
      case 'xhttp': return XHttpStreamSettingsSchema.parse({});
      default: return {};
    }
  };
  const onNetworkChange = (next: string) => {
    const ALL = ['tcpSettings', 'kcpSettings', 'wsSettings', 'grpcSettings', 'httpupgradeSettings', 'xhttpSettings'];
    const current = (form.getFieldValue('streamSettings') as Record<string, unknown>) ?? {};
    const cleaned: Record<string, unknown> = { ...current, network: next };
    for (const k of ALL) {
      if (k !== `${next}Settings`) delete cleaned[k];
    }
    cleaned[`${next}Settings`] = newStreamSlice(next);
    // mKCP wants a UDP mask wrapper on the FinalMask side; seed it with
    // `mkcp-legacy` so the inbound boots with a sensible default
    // instead of unobfuscated mKCP traffic. The user can still edit or
    // clear the mask via the FinalMask section.
    if (next === 'kcp') {
      const fm = (cleaned.finalmask as Record<string, unknown> | undefined) ?? {};
      const udp = Array.isArray(fm.udp) ? (fm.udp as unknown[]) : [];
      const hasMkcp = udp.some((m) => {
        const entry = m as { type?: string };
        return entry?.type === 'mkcp-legacy';
      });
      if (!hasMkcp) {
        cleaned.finalmask = {
          ...fm,
          udp: [...udp, { type: 'mkcp-legacy', settings: { header: '', value: '' } }],
        };
      }
    }
    form.setFieldValue('streamSettings', cleaned);
  };

  const streamTab = (
    <>
      {protocol !== Protocols.HYSTERIA && (
        <Form.Item label={t('transmission')} name={['streamSettings', 'network']}>
          <Select
            style={{ width: '75%' }}
            onChange={onNetworkChange}
            options={[
              { value: 'tcp', label: 'RAW' },
              { value: 'kcp', label: 'mKCP' },
              { value: 'ws', label: 'WebSocket' },
              { value: 'grpc', label: 'gRPC' },
              { value: 'httpupgrade', label: 'HTTPUpgrade' },
              { value: 'xhttp', label: 'XHTTP' },
            ]}
          />
        </Form.Item>
      )}

      {/* Inbound Hysteria stream sub-form. The transport for hysteria
          isn't user-selectable (always 'hysteria'), so the network
          dropdown is hidden above. Fields here mirror the legacy
          HysteriaStreamSettings inbound class: version is locked to 2,
          auth + udpIdleTimeout are required, masquerade is an optional
          sub-object that lets xray-core disguise the listener as an
          HTTP server when probed. */}
      {protocol === Protocols.HYSTERIA && <HysteriaFields form={form} />}

      {network === 'tcp' && <RawForm />}

      {network === 'ws' && <WsForm />}

      {network === 'grpc' && <GrpcForm />}

      {network === 'xhttp' && <XhttpForm form={form} />}

      {network === 'httpupgrade' && <HttpUpgradeForm />}

      {network === 'kcp' && <KcpForm />}

      <ExternalProxyForm toggleExternalProxy={toggleExternalProxy} />

      <SockoptForm toggleSockopt={toggleSockopt} />

      <FinalMaskForm
        name={['streamSettings', 'finalmask']}
        network={network as string}
        protocol={protocol}
        form={form}
      />
    </>
  );

  const securityTab = (
    <>
      <Form.Item name={['streamSettings', 'security']} hidden noStyle>
        <Input />
      </Form.Item>
      <Form.Item label={t('pages.inbounds.securityTab')}>
        <Form.Item
          noStyle
          shouldUpdate={(prev, curr) =>
            prev.streamSettings?.security !== curr.streamSettings?.security
            || prev.streamSettings?.network !== curr.streamSettings?.network
            || prev.protocol !== curr.protocol
          }
        >
          {({ getFieldValue }) => {
            const sec = getFieldValue(['streamSettings', 'security']) ?? 'none';
            const net = getFieldValue(['streamSettings', 'network']) ?? '';
            const proto = getFieldValue('protocol') ?? '';
            const tlsOk = canEnableTls({ protocol: proto, streamSettings: { network: net, security: sec } });
            const realityOk = canEnableReality({ protocol: proto, streamSettings: { network: net, security: sec } });
            const tlsOnly = proto === Protocols.HYSTERIA;
            return (
              <Radio.Group
                value={sec}
                buttonStyle="solid"
                disabled={!tlsOk}
                onChange={(e) => onSecurityChange(e.target.value)}
              >
                {!tlsOnly && <Radio.Button value="none">{t('none')}</Radio.Button>}
                <Radio.Button value="tls">TLS</Radio.Button>
                {realityOk && <Radio.Button value="reality">Reality</Radio.Button>}
              </Radio.Group>
            );
          }}
        </Form.Item>
      </Form.Item>

      {security === 'tls' && (
        <TlsForm
          saving={saving}
          setCertFromPanel={setCertFromPanel}
          clearCertFiles={clearCertFiles}
          generateRandomPinHash={generateRandomPinHash}
          getNewEchCert={getNewEchCert}
          clearEchCert={clearEchCert}
        />
      )}

      {security === 'reality' && (
        <RealityForm
          saving={saving}
          randomizeRealityTarget={randomizeRealityTarget}
          randomizeShortIds={randomizeShortIds}
          genRealityKeypair={genRealityKeypair}
          clearRealityKeypair={clearRealityKeypair}
          genMldsa65={genMldsa65}
          clearMldsa65={clearMldsa65}
        />
      )}
    </>
  );

  const advancedTab = (
    <div className="advanced-shell">
      <div className="advanced-panel">
        <div className="advanced-panel__header">
          <div>
            <div className="advanced-panel__title">{t('pages.inbounds.advanced.title')}</div>
            <div className="advanced-panel__subtitle">{t('pages.inbounds.advanced.subtitle')}</div>
          </div>
        </div>
        <Tabs
          className="advanced-inner-tabs"
          items={[
            {
              key: 'all',
              label: t('pages.inbounds.advanced.all'),
              children: (
                <>
                  <div className="advanced-editor-meta">
                    {t('pages.inbounds.advanced.allHelp')}
                  </div>
                  <AdvancedAllEditor form={form} streamEnabled={streamEnabled} />
                </>
              ),
            },
            {
              key: 'settings',
              label: t('pages.inbounds.advanced.settings'),
              children: (
                <>
                  <div className="advanced-editor-meta">
                    {t('pages.inbounds.advanced.settingsHelp')}{' '}
                    <code>{'{ settings: { ... } }'}</code>.
                  </div>
                  <AdvancedSliceEditor
                    form={form}
                    path="settings"
                    wrapKey="settings"
                    minHeight="320px"
                    maxHeight="540px"
                  />
                </>
              ),
            },
            ...(streamEnabled
              ? [{
                key: 'stream',
                label: t('pages.inbounds.advanced.stream'),
                children: (
                  <>
                    <div className="advanced-editor-meta">
                      {t('pages.inbounds.advanced.streamHelp')}{' '}
                      <code>{'{ streamSettings: { ... } }'}</code>.
                    </div>
                    <AdvancedSliceEditor
                      form={form}
                      path="streamSettings"
                      wrapKey="streamSettings"
                      minHeight="320px"
                      maxHeight="540px"
                    />
                  </>
                ),
              }]
              : []),
            {
              key: 'sniffing',
              label: t('pages.inbounds.advanced.sniffing'),
              children: (
                <>
                  <div className="advanced-editor-meta">
                    {t('pages.inbounds.advanced.sniffingHelp')}{' '}
                    <code>{'{ sniffing: { ... } }'}</code>.
                  </div>
                  <AdvancedSliceEditor
                    form={form}
                    path="sniffing"
                    wrapKey="sniffing"
                    minHeight="240px"
                    maxHeight="420px"
                  />
                </>
              ),
            },
          ]}
        />
      </div>
    </div>
  );

  const sniffingTab = <SniffingTab sniffingEnabled={sniffingEnabled} />;

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
            // forceRender on every tab so all Form.Items register at modal
            // open, not lazily on first visit. Without it, AntD's items API
            // lazy-mounts inactive tabs — their fields don't register, so
            // Form.useWatch on a parent path (e.g. 'sniffing') returns the
            // partial-view {} until the user touches the tab and the
            // inner Form.Item for `sniffing.enabled` registers.
            { key: 'basic', label: t('pages.xray.basicTemplate'), children: basicTab, forceRender: true },
            ...(([
              Protocols.VLESS,
              Protocols.SHADOWSOCKS,
              Protocols.HTTP,
              Protocols.MIXED,
              Protocols.TUNNEL,
              Protocols.TUN,
              Protocols.WIREGUARD,
            ] as string[]).includes(protocol) || isFallbackHost
              ? [{ key: 'protocol', label: t('pages.inbounds.protocol'), children: protocolTab, forceRender: true }]
              : []),
            ...(streamEnabled
              ? [
                { key: 'stream', label: t('pages.inbounds.streamTab'), children: streamTab, forceRender: true },
                { key: 'security', label: t('pages.inbounds.securityTab'), children: securityTab, forceRender: true },
              ]
              : []),
            { key: 'sniffing', label: t('pages.inbounds.sniffingTab'), children: sniffingTab, forceRender: true },
            { key: 'advanced', label: t('pages.xray.advancedTemplate'), children: advancedTab, forceRender: true },
          ]} />
        </Form>
      </Modal>
    </>
  );
}
