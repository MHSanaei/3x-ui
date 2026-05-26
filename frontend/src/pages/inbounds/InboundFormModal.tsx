import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import {
  Button,
  Card,
  Checkbox,
  Divider,
  Empty,
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
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  DeleteOutlined,
  MinusOutlined,
  PlusOutlined,
  SyncOutlined,
} from '@ant-design/icons';

import { HttpUtil, NumberFormatter, RandomUtil, SizeFormatter, Wireguard } from '@/utils';
import {
  rawInboundToFormValues,
  formValuesToWirePayload,
  pruneEmpty,
  normalizeSniffing,
  normalizeClients,
  dropLegacyOptionalEmpties,
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
  type FallbackRow,
  type InboundFormValues,
} from '@/schemas/forms/inbound-form';
import { antdRule } from '@/utils/zodForm';
import {
  ALPN_OPTION,
  Address_Port_Strategy,
  DOMAIN_STRATEGY_OPTION,
  Protocols,
  SNIFFING_OPTION,
  TCP_CONGESTION_OPTION,
  TLS_CIPHER_OPTION,
  TLS_VERSION_OPTION,
  USAGE_OPTION,
  UTLS_FINGERPRINT,
} from '@/schemas/primitives';
import {
  HappyEyeballsSchema,
  SockoptStreamSettingsSchema,
} from '@/schemas/protocols/stream/sockopt';
import { HysteriaStreamSettingsSchema } from '@/schemas/protocols/stream/hysteria';
import { TlsStreamSettingsSchema } from '@/schemas/protocols/security/tls';
import { RealityStreamSettingsSchema } from '@/schemas/protocols/security/reality';
import { SniffingSchema } from '@/schemas/primitives/sniffing';
import { TcpStreamSettingsSchema } from '@/schemas/protocols/stream/tcp';
import { KcpStreamSettingsSchema } from '@/schemas/protocols/stream/kcp';
import { WsStreamSettingsSchema } from '@/schemas/protocols/stream/ws';
import { GrpcStreamSettingsSchema } from '@/schemas/protocols/stream/grpc';
import { HttpUpgradeStreamSettingsSchema } from '@/schemas/protocols/stream/httpupgrade';
import { XHttpStreamSettingsSchema } from '@/schemas/protocols/stream/xhttp';
import DateTimePicker from '@/components/DateTimePicker';
import FinalMaskForm from '@/components/FinalMaskForm';
import HeaderMapEditor from '@/components/HeaderMapEditor';
import InputAddon from '@/components/InputAddon';
import JsonEditor from '@/components/JsonEditor';
import './InboundFormModal.css';
import type { FormInstance } from 'antd';
import type { NamePath } from 'antd/es/form/interface';

const { TextArea } = Input;
import { coerceInboundJsonField, type DBInbound } from '@/models/dbinbound';
import type { NodeRecord } from '@/api/queries/useNodesQuery';

// Pattern A rewrite of InboundFormModal. Built as a sibling file so the
// build stays green while the rewrite progresses section by section.
// InboundsPage continues to render the old InboundFormModal.tsx until the
// atomic swap at the end (Core Decision 7).

const { Text } = Typography;

// Sub-editor for one slice of the form (settings, streamSettings, sniffing).
// Holds a local text buffer so the user can type freely; on every keystroke
// we try to JSON.parse and forward the result to form state. Invalid JSON
// is held in the buffer until the next valid moment — no panic on partial
// input. The buffer seeds once on mount; the modal's destroyOnHidden makes
// each open a fresh editor instance, so we don't need to re-sync on outer
// form changes.
function AdvancedSliceEditor({
  form,
  path,
  wrapKey,
  minHeight,
  maxHeight,
}: {
  form: FormInstance<InboundFormValues>;
  path: NamePath;
  // When set, the editor wraps the inner value with `{ [wrapKey]: ... }` so
  // the JSON the user sees matches the wire shape's slice envelope (e.g.
  // `{ "settings": { ... } }`). Edits unwrap the outer key before writing
  // back to the form. Mirrors the legacy modal's wrappedConfigValue.
  wrapKey?: string;
  minHeight?: string;
  maxHeight?: string;
}) {
  const serialize = (value: unknown): string => {
    const inner = value ?? {};
    return JSON.stringify(wrapKey ? { [wrapKey]: inner } : inner, null, 2);
  };

  // preserve: true so useWatch returns the full subtree from the form
  // store — without it, useWatch goes through getFieldsValue() which
  // filters out unregistered fields. Slices like `settings` would lose
  // their `clients` / `fallbacks` sub-trees because those aren't bound
  // to any Form.Item.
  const watched = Form.useWatch(path, { form, preserve: true });
  const lastEmitRef = useRef<string>('');
  const [text, setText] = useState(() => {
    const initial = serialize(form.getFieldValue(path));
    lastEmitRef.current = initial;
    return initial;
  });

  useEffect(() => {
    const formStr = serialize(watched);
    if (formStr === lastEmitRef.current) return;
    setText(formStr);
    lastEmitRef.current = formStr;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [watched, wrapKey]);

  return (
    <JsonEditor
      value={text}
      minHeight={minHeight}
      maxHeight={maxHeight}
      onChange={(next) => {
        setText(next);
        try {
          const parsed = JSON.parse(next);
          const toWrite = wrapKey && parsed && typeof parsed === 'object' && !Array.isArray(parsed)
            ? (parsed as Record<string, unknown>)[wrapKey] ?? {}
            : parsed;
          form.setFieldValue(path, toWrite);
          lastEmitRef.current = JSON.stringify(wrapKey ? { [wrapKey]: toWrite } : toWrite, null, 2);
        } catch {
          // invalid JSON; keep buffer, don't push to form
        }
      }}
    />
  );
}

// The "All" editor shows the full inbound JSON in one editor: top-level
// connection fields plus the three nested sub-objects (settings,
// streamSettings, sniffing). Edits round-trip back to the form's slices,
// mirroring the legacy modal's setAdvancedAllValue behavior. Reactivity
// works the same way as AdvancedSliceEditor: useWatch on the slices we
// care about, lastEmitRef as the "we wrote this" guard.
function AdvancedAllEditor({
  form,
  streamEnabled,
}: {
  form: FormInstance<InboundFormValues>;
  streamEnabled: boolean;
}) {
  // preserve: true — default useWatch returns only registered fields, so
  // sub-trees we never bound (settings.clients/fallbacks, sniffing
  // defaults, etc.) wouldn't show up. preserve switches the read to
  // getFieldsValue(true) which returns the full form store.
  const wListen = Form.useWatch('listen', { form, preserve: true });
  const wPort = Form.useWatch('port', { form, preserve: true });
  const wProtocol = Form.useWatch('protocol', { form, preserve: true });
  const wTag = Form.useWatch('tag', { form, preserve: true });
  const wSettings = Form.useWatch('settings', { form, preserve: true });
  const wSniffing = Form.useWatch('sniffing', { form, preserve: true });
  const wStream = Form.useWatch('streamSettings', { form, preserve: true });

  const serialize = () => {
    // Apply the same prune/normalize as the wire payload so the JSON
    // shown here is what the panel actually POSTs (no empty defaults,
    // disabled sniffing as { enabled: false }, finalmask dropped when
    // there are no masks).
    const settingsView = (pruneEmpty(wSettings ?? {}) ?? {}) as Record<string, unknown>;
    if (typeof wProtocol === 'string' && Array.isArray(settingsView.clients)) {
      settingsView.clients = normalizeClients(wProtocol, settingsView.clients);
    }
    const streamView = streamEnabled
      ? ((pruneEmpty(wStream ?? {}) ?? {}) as Record<string, unknown>)
      : undefined;
    dropLegacyOptionalEmpties(settingsView, streamView);
    const out: Record<string, unknown> = {
      listen: wListen ?? '',
      port: wPort ?? 0,
      protocol: wProtocol ?? '',
      tag: wTag ?? '',
      settings: settingsView,
      sniffing: normalizeSniffing(wSniffing as Parameters<typeof normalizeSniffing>[0]),
    };
    if (streamView) out.streamSettings = streamView;
    return JSON.stringify(out, null, 2);
  };

  const lastEmitRef = useRef<string>('');
  const [text, setText] = useState(() => {
    const initial = serialize();
    lastEmitRef.current = initial;
    return initial;
  });

  useEffect(() => {
    const formStr = serialize();
    if (formStr === lastEmitRef.current) return;
    setText(formStr);
    lastEmitRef.current = formStr;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [wListen, wPort, wProtocol, wTag, wSettings, wSniffing, wStream, streamEnabled]);

  return (
    <JsonEditor
      value={text}
      minHeight="340px"
      maxHeight="560px"
      onChange={(next) => {
        setText(next);
        let parsed: Record<string, unknown>;
        try {
          parsed = JSON.parse(next) as Record<string, unknown>;
        } catch {
          return;
        }
        if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return;
        if (typeof parsed.listen === 'string') form.setFieldValue('listen', parsed.listen);
        if (typeof parsed.port === 'number' && Number.isFinite(parsed.port)) {
          form.setFieldValue('port', parsed.port);
        }
        if (typeof parsed.protocol === 'string') form.setFieldValue('protocol', parsed.protocol);
        if (typeof parsed.tag === 'string') form.setFieldValue('tag', parsed.tag);
        if (parsed.settings && typeof parsed.settings === 'object') {
          form.setFieldValue('settings', parsed.settings);
        }
        if (parsed.sniffing && typeof parsed.sniffing === 'object') {
          form.setFieldValue('sniffing', parsed.sniffing);
        }
        if (streamEnabled && parsed.streamSettings && typeof parsed.streamSettings === 'object') {
          form.setFieldValue('streamSettings', parsed.streamSettings);
        }
        lastEmitRef.current = next;
      }}
    />
  );
}

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
  const fallbackKeyRef = useRef(0);
  const [fallbacks, setFallbacks] = useState<FallbackRow[]>([]);

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
  const isFallbackHost =
    (protocol === Protocols.VLESS || protocol === Protocols.TROJAN)
    && network === 'tcp'
    && (security === 'tls' || security === 'reality');

  const fallbackChildOptions = (dbInbounds || [])
    .filter((ib) => ib.id !== dbInbound?.id)
    .map((ib) => ({
      label: `${ib.remark || `#${ib.id}`} · ${ib.protocol}:${ib.port}`,
      value: ib.id,
    }));

  const loadFallbacks = async (masterId: number | null) => {
    if (!masterId) {
      setFallbacks([]);
      return;
    }
    const msg = await HttpUtil.get(`/panel/api/inbounds/${masterId}/fallbacks`);
    if (!msg?.success || !Array.isArray(msg.obj)) {
      setFallbacks([]);
      return;
    }
    setFallbacks(
      (msg.obj as { childId: number; name?: string; alpn?: string; path?: string; xver?: number }[])
        .map((r) => ({
          rowKey: `fb-${++fallbackKeyRef.current}`,
          childId: r.childId,
          name: r.name || '',
          alpn: r.alpn || '',
          path: r.path || '',
          xver: r.xver || 0,
        })),
    );
  };

  const saveFallbacks = async (masterId: number) => {
    if (!masterId) return true;
    const payload = {
      fallbacks: fallbacks.filter((c) => c.childId).map((c, i) => ({
        childId: c.childId,
        name: c.name,
        alpn: c.alpn,
        path: c.path,
        xver: Number(c.xver) || 0,
        sortOrder: i,
      })),
    };
    const msg = await HttpUtil.post(
      `/panel/api/inbounds/${masterId}/fallbacks`,
      payload,
      { headers: { 'Content-Type': 'application/json' } },
    );
    return !!msg?.success;
  };

  // Derive a fallback row's SNI / ALPN / Path / xver from a child
  // inbound's streamSettings — what the legacy panel auto-filled when an
  // operator wired a fallback target. SNI/ALPN come straight off the
  // child's TLS block; path depends on the child's transport (ws/grpc
  // /httpupgrade carry an explicit path; tcp/kcp/xhttp have no path of
  // their own). xver stays 0 unless the child explicitly opts in via
  // PROXY-protocol sockopt.
  const deriveFallbackDefaults = (childId: number): Partial<FallbackRow> => {
    const child = (dbInbounds || []).find((ib) => ib.id === childId);
    if (!child) return {};
    const stream = coerceInboundJsonField(child.streamSettings);
    const tls = (stream.tlsSettings as Record<string, unknown> | undefined) ?? {};
    const network = typeof stream.network === 'string' ? stream.network : '';
    const sni = typeof tls.serverName === 'string' ? tls.serverName : '';
    const alpnArr = Array.isArray(tls.alpn) ? tls.alpn : [];
    const alpn = alpnArr.filter((v) => typeof v === 'string').join(',');
    let path = '';
    if (network === 'ws') {
      const ws = (stream.wsSettings as Record<string, unknown> | undefined) ?? {};
      if (typeof ws.path === 'string') path = ws.path;
    } else if (network === 'grpc') {
      const grpc = (stream.grpcSettings as Record<string, unknown> | undefined) ?? {};
      if (typeof grpc.serviceName === 'string') path = grpc.serviceName;
    } else if (network === 'httpupgrade') {
      const hu = (stream.httpupgradeSettings as Record<string, unknown> | undefined) ?? {};
      if (typeof hu.path === 'string') path = hu.path;
    } else if (network === 'xhttp') {
      const xh = (stream.xhttpSettings as Record<string, unknown> | undefined) ?? {};
      if (typeof xh.path === 'string') path = xh.path;
    }
    return { name: sni, alpn, path, xver: 0 };
  };

  const addFallback = () => {
    setFallbacks((prev) => [...prev, {
      rowKey: `fb-${++fallbackKeyRef.current}`,
      childId: null,
      name: '',
      alpn: '',
      path: '',
      xver: 0,
    }]);
  };

  const updateFallback = (rowKey: string, patch: Partial<FallbackRow>) => {
    setFallbacks((prev) => prev.map((r) => {
      if (r.rowKey !== rowKey) return r;
      // When the picker selects a new child inbound and the row hasn't
      // been hand-edited yet (sni/alpn/path all blank, xver = 0), pull
      // the SNI/ALPN/Path defaults off that child. Operators who
      // intentionally typed values keep them — we only fill the empties.
      if (typeof patch.childId === 'number' && patch.childId !== r.childId) {
        const isPristine = !r.name && !r.alpn && !r.path && r.xver === 0;
        if (isPristine) return { ...r, ...patch, ...deriveFallbackDefaults(patch.childId) };
      }
      return { ...r, ...patch };
    }));
  };

  const removeFallback = (idx: number) => {
    setFallbacks((prev) => prev.filter((_, i) => i !== idx));
  };

  // Move a fallback row up/down by swapping adjacent indices. The order
  // is persisted via the fallback row's sortOrder (rebuilt by index on
  // save), so reordering survives reloads.
  const moveFallback = (idx: number, direction: -1 | 1) => {
    setFallbacks((prev) => {
      const target = idx + direction;
      if (target < 0 || target >= prev.length) return prev;
      const next = prev.slice();
      [next[idx], next[target]] = [next[target], next[idx]];
      return next;
    });
  };

  // One-shot: add a fresh fallback row for every eligible inbound (i.e.
  // every option in fallbackChildOptions) that is not already wired up.
  // Convenient for operators who want catch-all routing to every host
  // they manage on the panel.
  const addAllFallbacks = () => {
    setFallbacks((prev) => {
      const alreadyHave = new Set(prev.map((r) => r.childId));
      const additions = fallbackChildOptions
        .filter((opt) => !alreadyHave.has(opt.value))
        .map<FallbackRow>((opt) => {
          const derived = deriveFallbackDefaults(opt.value);
          return {
            rowKey: `fb-${++fallbackKeyRef.current}`,
            childId: opt.value,
            name: derived.name ?? '',
            alpn: derived.alpn ?? '',
            path: derived.path ?? '',
            xver: derived.xver ?? 0,
          };
        });
      if (additions.length === 0) return prev;
      return [...prev, ...additions];
    });
  };

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

  const onSecurityChange = async (next: string) => {
    const current = (form.getFieldValue('streamSettings') as Record<string, unknown>) ?? {};
    const cleaned: Record<string, unknown> = { ...current, security: next };
    delete cleaned.tlsSettings;
    delete cleaned.realitySettings;
    if (next === 'tls') {
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
      cleaned.tlsSettings = tls;
    }
    if (next === 'reality') {
      const reality = RealityStreamSettingsSchema.parse({}) as Record<string, unknown>;
      const tgt = getRandomRealityTarget() as { target: string; sni: string };
      reality.target = tgt.target;
      reality.serverNames = tgt.sni.split(',').map((s) => s.trim()).filter(Boolean);
      reality.shortIds = RandomUtil.randomShortIds().split(',').map((s) => s.trim()).filter(Boolean);
      cleaned.realitySettings = reality;
    }
    form.setFieldValue('streamSettings', cleaned);
    if (next === 'reality') {
      try {
        const msg = await HttpUtil.get('/panel/api/server/getNewX25519Cert');
        if (msg?.success) {
          const obj = msg.obj as { privateKey: string; publicKey: string };
          form.setFieldValue(['streamSettings', 'realitySettings', 'privateKey'], obj.privateKey);
          form.setFieldValue(['streamSettings', 'realitySettings', 'settings', 'publicKey'], obj.publicKey);
        }
      } catch {
        // best-effort: leave keypair fields empty if server call fails
      }
    }
  };
  const xhttpMode = Form.useWatch(['streamSettings', 'xhttpSettings', 'mode'], form);
  const xhttpObfsMode = Form.useWatch(['streamSettings', 'xhttpSettings', 'xPaddingObfsMode'], form) ?? false;
  const xhttpSessionPlacement = Form.useWatch(['streamSettings', 'xhttpSettings', 'sessionPlacement'], form);
  const xhttpSeqPlacement = Form.useWatch(['streamSettings', 'xhttpSettings', 'seqPlacement'], form);
  const xhttpUplinkPlacement = Form.useWatch(['streamSettings', 'xhttpSettings', 'uplinkDataPlacement'], form);

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
    if (
      mode === 'edit'
      && dbInbound
      && (dbInbound.protocol === Protocols.VLESS || dbInbound.protocol === Protocols.TROJAN)
    ) {
      loadFallbacks(dbInbound.id);
    } else {
      setFallbacks([]);
    }

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
      const issue = parsed.error.issues[0];
      const path = Array.isArray(issue?.path) && issue.path.length > 0
        ? issue.path.join('.')
        : '';
      const baseMsg = issue?.message ?? 'somethingWentWrong';
      const display = path ? `${path}: ${baseMsg}` : baseMsg;
      messageApi.error(t(baseMsg, { defaultValue: display }));
      console.error('[InboundFormModal] schema validation failed', {
        path: issue?.path,
        message: issue?.message,
        values,
      });
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
            options={[
              { value: null, label: t('pages.inbounds.localPanel') },
              ...selectableNodes.map((n) => ({
                value: n.id,
                label: `${n.name}${n.status === 'offline' ? ' (offline)' : ''}`,
                disabled: n.status === 'offline',
              })),
            ]}
          />
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
    <Card size="small" className="mt-12" title={t('pages.inbounds.fallbacks.title') || 'Fallbacks'}>
      {fallbacks.length === 0 && (
        <Empty
          description={t('pages.inbounds.fallbacks.empty') || 'No fallbacks yet'}
          styles={{ image: { height: 40 } }}
          style={{ margin: '8px 0 12px' }}
        />
      )}
      {fallbacks.map((record, idx) => (
        <div
          key={record.rowKey}
          style={{ border: '1px solid var(--app-border-tertiary)', borderRadius: 6, padding: '10px 12px', marginBottom: 8 }}
        >
          <Space.Compact block style={{ marginBottom: 6 }}>
            <Select
              value={record.childId}
              options={fallbackChildOptions}
              placeholder={t('pages.inbounds.fallbacks.pickInbound') || 'Pick an inbound'}
              showSearch={{
                filterOption: (input, option) =>
                  ((option?.label as string) || '').toLowerCase().includes(input.toLowerCase()),
              }}
              style={{ width: '100%' }}
              onChange={(v) => updateFallback(record.rowKey, { childId: v })}
            />
            <Button
              disabled={idx === 0}
              onClick={() => moveFallback(idx, -1)}
              title="Move up"
            >
              <ArrowUpOutlined />
            </Button>
            <Button
              disabled={idx === fallbacks.length - 1}
              onClick={() => moveFallback(idx, 1)}
              title="Move down"
            >
              <ArrowDownOutlined />
            </Button>
            <Button danger onClick={() => removeFallback(idx)}>
              <DeleteOutlined />
            </Button>
          </Space.Compact>
          <Space.Compact block>
            <InputAddon>SNI</InputAddon>
            <Input
              placeholder={t('pages.inbounds.fallbacks.matchAny') || 'any'}
              value={record.name}
              onChange={(e) => updateFallback(record.rowKey, { name: e.target.value })}
            />
            <InputAddon>ALPN</InputAddon>
            <Input
              placeholder={t('pages.inbounds.fallbacks.matchAny') || 'any'}
              value={record.alpn}
              onChange={(e) => updateFallback(record.rowKey, { alpn: e.target.value })}
            />
            <InputAddon>Path</InputAddon>
            <Input
              placeholder="/"
              value={record.path}
              onChange={(e) => updateFallback(record.rowKey, { path: e.target.value })}
            />
            <InputAddon>xver</InputAddon>
            <InputNumber
              min={0}
              max={2}
              value={record.xver}
              onChange={(v) => updateFallback(record.rowKey, { xver: Number(v) || 0 })}
            />
          </Space.Compact>
        </div>
      ))}
      <Space>
        <Button size="small" onClick={addFallback}>
          <PlusOutlined /> {t('pages.inbounds.fallbacks.add') || 'Add fallback'}
        </Button>
        <Button
          size="small"
          onClick={addAllFallbacks}
          disabled={fallbackChildOptions.length === 0
            || fallbacks.length >= fallbackChildOptions.length}
          title="Add a fallback row for every eligible inbound not yet wired up"
        >
          Add all
        </Button>
      </Space>
    </Card>
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
                    onClick={() => {
                      const kp = Wireguard.generateKeypair();
                      add({
                        privateKey: kp.privateKey,
                        publicKey: kp.publicKey,
                        allowedIPs: ['10.0.0.2/32'],
                        keepAlive: 0,
                      });
                    }}
                  >
                    <PlusOutlined /> Add peer
                  </Button>
                </Form.Item>
                {fields.map((field, idx) => (
                  <div key={field.key} className="wg-peer">
                    <Divider titlePlacement="center">
                      <Space>
                        <span>Peer {idx + 1}</span>
                        {fields.length > 1 && (
                          <Button
                            size="small"
                            danger
                            icon={<MinusOutlined />}
                            onClick={() => remove(field.name)}
                          />
                        )}
                      </Space>
                    </Divider>
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
            <Select
              options={[
                { value: 'tcp,udp', label: 'TCP, UDP' },
                { value: 'tcp', label: 'TCP' },
                { value: 'udp', label: 'UDP' },
              ]}
            />
          </Form.Item>
          <Form.Item label="Port map" name={['settings', 'portMap']}>
            <HeaderMapEditor mode="v1" />
          </Form.Item>
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
                  <Button
                    size="small"
                    onClick={() => add({
                      user: RandomUtil.randomLowerAndNum(8),
                      pass: RandomUtil.randomLowerAndNum(12),
                    })}
                  >
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
                <Select
                  options={[
                    { value: 'noauth', label: 'noauth' },
                    { value: 'password', label: 'password' },
                  ]}
                />
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
              options={SSMethodSchema.options.map((m) => ({ value: m, label: m }))}
            />
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
            <Select
              style={{ width: 120 }}
              options={[
                { value: 'tcp,udp', label: 'TCP, UDP' },
                { value: 'tcp', label: 'TCP' },
                { value: 'udp', label: 'UDP' },
              ]}
            />
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
      case 'tcp':         return TcpStreamSettingsSchema.parse({ header: { type: 'none' } });
      case 'kcp':         return KcpStreamSettingsSchema.parse({});
      case 'ws':          return WsStreamSettingsSchema.parse({});
      case 'grpc':        return GrpcStreamSettingsSchema.parse({});
      case 'httpupgrade': return HttpUpgradeStreamSettingsSchema.parse({});
      case 'xhttp':       return XHttpStreamSettingsSchema.parse({});
      default:            return {};
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
    form.setFieldValue('streamSettings', cleaned);
  };

  const streamTab = (
    <>
      {protocol !== Protocols.HYSTERIA && (
        <Form.Item label="Transmission" name={['streamSettings', 'network']}>
          <Select
            style={{ width: '75%' }}
            onChange={onNetworkChange}
            options={[
              { value: 'tcp', label: 'TCP (RAW)' },
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
      {protocol === Protocols.HYSTERIA && (
        <>
          <Form.Item
            label="Version"
            name={['streamSettings', 'hysteriaSettings', 'version']}
          >
            <InputNumber min={2} max={2} disabled />
          </Form.Item>
          <Form.Item
            label="UDP idle timeout (s)"
            name={['streamSettings', 'hysteriaSettings', 'udpIdleTimeout']}
          >
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>

          <Form.Item label="Masquerade">
            <Form.Item shouldUpdate noStyle>
              {() => {
                const m = form.getFieldValue([
                  'streamSettings', 'hysteriaSettings', 'masquerade',
                ]);
                return (
                  <Switch
                    checked={!!m}
                    onChange={(checked) =>
                      form.setFieldValue(
                        ['streamSettings', 'hysteriaSettings', 'masquerade'],
                        checked
                          ? {
                              type: '', dir: '', url: '',
                              rewriteHost: false, insecure: false,
                              content: '', headers: {}, statusCode: 0,
                            }
                          : undefined,
                      )
                    }
                  />
                );
              }}
            </Form.Item>
          </Form.Item>
          <Form.Item shouldUpdate noStyle>
            {() => {
              const m = form.getFieldValue([
                'streamSettings', 'hysteriaSettings', 'masquerade',
              ]) as { type?: string } | undefined;
              if (!m) return null;
              return (
                <>
                  <Form.Item
                    label="Type"
                    name={['streamSettings', 'hysteriaSettings', 'masquerade', 'type']}
                  >
                    <Select
                      options={[
                        { value: '', label: 'default (404 page)' },
                        { value: 'proxy', label: 'proxy (reverse proxy)' },
                        { value: 'file', label: 'file (serve directory)' },
                        { value: 'string', label: 'string (fixed body)' },
                      ]}
                    />
                  </Form.Item>
                  {m.type === 'proxy' && (
                    <>
                      <Form.Item
                        label="Upstream URL"
                        name={['streamSettings', 'hysteriaSettings', 'masquerade', 'url']}
                      >
                        <Input placeholder="https://www.example.com" />
                      </Form.Item>
                      <Form.Item
                        label="Rewrite Host"
                        name={['streamSettings', 'hysteriaSettings', 'masquerade', 'rewriteHost']}
                        valuePropName="checked"
                      >
                        <Switch />
                      </Form.Item>
                      <Form.Item
                        label="Skip TLS verify"
                        name={['streamSettings', 'hysteriaSettings', 'masquerade', 'insecure']}
                        valuePropName="checked"
                      >
                        <Switch />
                      </Form.Item>
                    </>
                  )}
                  {m.type === 'file' && (
                    <Form.Item
                      label="Directory"
                      name={['streamSettings', 'hysteriaSettings', 'masquerade', 'dir']}
                    >
                      <Input placeholder="/var/www/html" />
                    </Form.Item>
                  )}
                  {m.type === 'string' && (
                    <>
                      <Form.Item
                        label="Status code"
                        name={['streamSettings', 'hysteriaSettings', 'masquerade', 'statusCode']}
                      >
                        <InputNumber min={0} max={599} style={{ width: '100%' }} />
                      </Form.Item>
                      <Form.Item
                        label="Body"
                        name={['streamSettings', 'hysteriaSettings', 'masquerade', 'content']}
                      >
                        <Input.TextArea autoSize={{ minRows: 3 }} />
                      </Form.Item>
                      <Form.Item
                        label="Headers"
                        name={['streamSettings', 'hysteriaSettings', 'masquerade', 'headers']}
                      >
                        <HeaderMapEditor mode="v1" />
                      </Form.Item>
                    </>
                  )}
                </>
              );
            }}
          </Form.Item>
        </>
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
                        v
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
                      );
                    }}
                  />
                );
              }}
            </Form.Item>
          </Form.Item>
          {/* Per Xray docs (transports/raw.html#httpheaderobject), the
              `request` object is honored only by outbound proxies; the
              inbound listener reads `response`. Showing Host / Path /
              Method / Version / request-headers on the inbound side was
              a regression from this modal's earlier iteration — those
              inputs wrote to the wire but xray-core ignored them. The
              inbound modal now only exposes the response side. */}
          <Form.Item
            noStyle
            shouldUpdate={(prev, curr) =>
              prev.streamSettings?.tcpSettings?.header?.type
              !== curr.streamSettings?.tcpSettings?.header?.type
            }
          >
            {({ getFieldValue }) => {
              const headerType = getFieldValue(
                ['streamSettings', 'tcpSettings', 'header', 'type'],
              ) as string | undefined;
              if (headerType !== 'http') return null;
              return (
                <>
                  <Form.Item
                    label="Response version"
                    name={[
                      'streamSettings', 'tcpSettings', 'header',
                      'response', 'version',
                    ]}
                  >
                    <Input placeholder="1.1" />
                  </Form.Item>
                  <Form.Item
                    label="Response status"
                    name={[
                      'streamSettings', 'tcpSettings', 'header',
                      'response', 'status',
                    ]}
                  >
                    <Input placeholder="200" />
                  </Form.Item>
                  <Form.Item
                    label="Response reason"
                    name={[
                      'streamSettings', 'tcpSettings', 'header',
                      'response', 'reason',
                    ]}
                  >
                    <Input placeholder="OK" />
                  </Form.Item>
                  <Form.Item
                    label="Response headers"
                    name={[
                      'streamSettings', 'tcpSettings', 'header',
                      'response', 'headers',
                    ]}
                  >
                    <HeaderMapEditor mode="v2" />
                  </Form.Item>
                </>
              );
            }}
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
          <Form.Item
            label="Headers"
            name={['streamSettings', 'wsSettings', 'headers']}
          >
            <HeaderMapEditor mode="v1" />
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
            <Select
              style={{ width: '50%' }}
              options={(['auto', 'packet-up', 'stream-up', 'stream-one'] as const).map((m) => ({
                value: m,
                label: m,
              }))}
            />
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
            <Select
              options={[
                { value: '', label: 'Default (POST)' },
                { value: 'POST', label: 'POST' },
                { value: 'PUT', label: 'PUT' },
                {
                  value: 'GET',
                  label: 'GET (packet-up only)',
                  disabled: xhttpMode !== 'packet-up',
                },
              ]}
            />
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
                name={['streamSettings', 'xhttpSettings', 'xPaddingMethod']}
                label="Padding Method"
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
          )}
          <Form.Item
            name={['streamSettings', 'xhttpSettings', 'sessionPlacement']}
            label="Session Placement"
          >
            <Select
              options={[
                { value: '', label: 'Default (path)' },
                { value: 'path', label: 'path' },
                { value: 'header', label: 'header' },
                { value: 'cookie', label: 'cookie' },
                { value: 'query', label: 'query' },
              ]}
            />
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
            <Select
              options={[
                { value: '', label: 'Default (path)' },
                { value: 'path', label: 'path' },
                { value: 'header', label: 'header' },
                { value: 'cookie', label: 'cookie' },
                { value: 'query', label: 'query' },
              ]}
            />
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
          <Form.Item
            label="Headers"
            name={['streamSettings', 'httpupgradeSettings', 'headers']}
          >
            <HeaderMapEditor mode="v1" />
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
          <Form.Item name={['streamSettings', 'kcpSettings', 'uplinkCapacity']} label="Uplink (MB/s)">
            <InputNumber min={0} />
          </Form.Item>
          <Form.Item name={['streamSettings', 'kcpSettings', 'downlinkCapacity']} label="Downlink (MB/s)">
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

      <Form.Item
        noStyle
        shouldUpdate={(prev, curr) => {
          const a = (prev.streamSettings as { externalProxy?: unknown[] } | undefined)?.externalProxy;
          const b = (curr.streamSettings as { externalProxy?: unknown[] } | undefined)?.externalProxy;
          return (Array.isArray(a) ? a.length : 0) !== (Array.isArray(b) ? b.length : 0);
        }}
      >
        {({ getFieldValue }) => {
          const arr = getFieldValue(['streamSettings', 'externalProxy']);
          const on = Array.isArray(arr) && arr.length > 0;
          return (
            <>
              <Form.Item label="External Proxy">
                <Switch checked={on} onChange={toggleExternalProxy} />
              </Form.Item>
              {on && (
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
                                <Select
                                  style={{ width: '20%' }}
                                  options={[
                                    { value: 'same', label: t('pages.inbounds.same') },
                                    { value: 'none', label: t('none') },
                                    { value: 'tls', label: 'TLS' },
                                  ]}
                                />
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
                                      <Select
                                        style={{ width: '30%' }}
                                        placeholder="Fingerprint"
                                        options={[
                                          { value: '', label: 'Default' },
                                          ...Object.values(UTLS_FINGERPRINT).map((fp) => ({
                                            value: fp,
                                            label: fp,
                                          })),
                                        ]}
                                      />
                                    </Form.Item>
                                    <Form.Item name={[field.name, 'alpn']} noStyle>
                                      <Select
                                        mode="multiple"
                                        style={{ width: '40%' }}
                                        placeholder="ALPN"
                                        options={Object.values(ALPN_OPTION).map((a) => ({
                                          value: a,
                                          label: a,
                                        }))}
                                      />
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
            </>
          );
        }}
      </Form.Item>

      <Form.Item
        noStyle
        shouldUpdate={(prev, curr) => {
          const a = (prev.streamSettings as { sockopt?: object } | undefined)?.sockopt;
          const b = (curr.streamSettings as { sockopt?: object } | undefined)?.sockopt;
          return !!a !== !!b;
        }}
      >
        {({ getFieldValue }) => {
          const sock = getFieldValue(['streamSettings', 'sockopt']);
          const on = !!sock && typeof sock === 'object' && Object.keys(sock).length > 0;
          return (
            <>
              <Form.Item label="Sockopt">
                <Switch checked={on} onChange={toggleSockopt} />
              </Form.Item>
              {on && (
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
            <Select
              style={{ width: '50%' }}
              options={Object.values(DOMAIN_STRATEGY_OPTION).map((d) => ({ value: d, label: d }))}
            />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'sockopt', 'tcpcongestion']}
            label="TCP Congestion"
          >
            <Select
              style={{ width: '50%' }}
              options={Object.values(TCP_CONGESTION_OPTION).map((c) => ({ value: c, label: c }))}
            />
          </Form.Item>
          <Form.Item name={['streamSettings', 'sockopt', 'tproxy']} label="TProxy">
            <Select
              style={{ width: '50%' }}
              options={[
                { value: 'off', label: 'Off' },
                { value: 'redirect', label: 'Redirect' },
                { value: 'tproxy', label: 'TProxy' },
              ]}
            />
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
            <Select
              mode="tags"
              style={{ width: '100%' }}
              tokenSeparators={[',']}
              options={[
                { value: 'CF-Connecting-IP', label: 'CF-Connecting-IP' },
                { value: 'X-Real-IP', label: 'X-Real-IP' },
                { value: 'True-Client-IP', label: 'True-Client-IP' },
                { value: 'X-Client-IP', label: 'X-Client-IP' },
              ]}
            />
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'sockopt', 'addressPortStrategy']}
            label="Address+port strategy"
          >
            <Select
              style={{ width: '50%' }}
              options={Object.values(Address_Port_Strategy).map((v) => ({ value: v, label: v }))}
            />
          </Form.Item>
          <Form.Item shouldUpdate noStyle>
            {({ getFieldValue, setFieldValue }) => {
              const he = getFieldValue(['streamSettings', 'sockopt', 'happyEyeballs']);
              const hasHe = he != null;
              return (
                <>
                  <Form.Item label="Happy Eyeballs">
                    <Switch
                      checked={hasHe}
                      onChange={(v) => {
                        setFieldValue(
                          ['streamSettings', 'sockopt', 'happyEyeballs'],
                          v ? HappyEyeballsSchema.parse({}) : undefined,
                        );
                      }}
                    />
                  </Form.Item>
                  {hasHe && (
                    <>
                      <Form.Item
                        name={['streamSettings', 'sockopt', 'happyEyeballs', 'tryDelayMs']}
                        label="Try delay (ms)"
                      >
                        <InputNumber min={0} placeholder="0 disabled — 250 recommended" />
                      </Form.Item>
                      <Form.Item
                        name={['streamSettings', 'sockopt', 'happyEyeballs', 'prioritizeIPv6']}
                        label="Prioritize IPv6"
                        valuePropName="checked"
                      >
                        <Switch />
                      </Form.Item>
                      <Form.Item
                        name={['streamSettings', 'sockopt', 'happyEyeballs', 'interleave']}
                        label="Interleave"
                      >
                        <InputNumber min={1} />
                      </Form.Item>
                      <Form.Item
                        name={['streamSettings', 'sockopt', 'happyEyeballs', 'maxConcurrentTry']}
                        label="Max concurrent try"
                      >
                        <InputNumber min={0} />
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
                <Form.Item label="Custom sockopt">
                  <Button
                    type="dashed"
                    size="small"
                    onClick={() => add({ type: 'int', level: '6', opt: '', value: '' })}
                  >
                    + Add custom option
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
                      <Input placeholder="opt" style={{ width: 120 }} />
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
                {!tlsOnly && <Radio.Button value="none">none</Radio.Button>}
                <Radio.Button value="tls">tls</Radio.Button>
                {realityOk && <Radio.Button value="reality">reality</Radio.Button>}
              </Radio.Group>
            );
          }}
        </Form.Item>
      </Form.Item>

      <Form.Item
        noStyle
        shouldUpdate={(prev, curr) =>
          prev.streamSettings?.security !== curr.streamSettings?.security
        }
      >
        {({ getFieldValue }) => {
          const sec = getFieldValue(['streamSettings', 'security']);
          if (sec !== 'tls') return null;
          return (
            <>
              <Form.Item name={['streamSettings', 'tlsSettings', 'serverName']} label="SNI">
            <Input placeholder="Server Name Indication" />
          </Form.Item>
          <Form.Item name={['streamSettings', 'tlsSettings', 'cipherSuites']} label="Cipher Suites">
            <Select
              options={[
                { value: '', label: 'Auto' },
                ...Object.entries(TLS_CIPHER_OPTION).map(([k, v]) => ({ value: v, label: k })),
              ]}
            />
          </Form.Item>
          <Form.Item label="Min/Max Version">
            <Space.Compact block>
              <Form.Item name={['streamSettings', 'tlsSettings', 'minVersion']} noStyle>
                <Select
                  style={{ width: '50%' }}
                  options={Object.values(TLS_VERSION_OPTION).map((v) => ({ value: v, label: v }))}
                />
              </Form.Item>
              <Form.Item name={['streamSettings', 'tlsSettings', 'maxVersion']} noStyle>
                <Select
                  style={{ width: '50%' }}
                  options={Object.values(TLS_VERSION_OPTION).map((v) => ({ value: v, label: v }))}
                />
              </Form.Item>
            </Space.Compact>
          </Form.Item>
          <Form.Item
            name={['streamSettings', 'tlsSettings', 'settings', 'fingerprint']}
            label="uTLS"
          >
            <Select
              options={[
                { value: '', label: 'None' },
                ...Object.values(UTLS_FINGERPRINT).map((fp) => ({ value: fp, label: fp })),
              ]}
            />
          </Form.Item>
          <Form.Item name={['streamSettings', 'tlsSettings', 'alpn']} label="ALPN">
            <Select
              mode="multiple"
              tokenSeparators={[',']}
              style={{ width: '100%' }}
              options={Object.values(ALPN_OPTION).map((a) => ({ value: a, label: a }))}
            />
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
                      <Select
                        style={{ width: '50%' }}
                        options={Object.values(USAGE_OPTION).map((u) => ({ value: u, label: u }))}
                      />
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
          );
        }}
      </Form.Item>

      <Form.Item
        noStyle
        shouldUpdate={(prev, curr) =>
          prev.streamSettings?.security !== curr.streamSettings?.security
        }
      >
        {({ getFieldValue }) => {
          const sec = getFieldValue(['streamSettings', 'security']);
          if (sec !== 'reality') return null;
          return (
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
            <Select
              options={Object.values(UTLS_FINGERPRINT).map((fp) => ({ value: fp, label: fp }))}
            />
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
          );
        }}
      </Form.Item>
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
