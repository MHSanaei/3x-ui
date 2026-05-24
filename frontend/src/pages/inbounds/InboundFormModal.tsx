/* eslint-disable @typescript-eslint/no-explicit-any */
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import dayjs, { type Dayjs } from 'dayjs';
import {
  Button,
  Card,
  Checkbox,
  Col,
  Divider,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Radio,
  Row,
  Select,
  Space,
  Switch,
  Tabs,
  Tooltip,
  Typography,
  message,
} from 'antd';
import {
  SyncOutlined,
  PlusOutlined,
  MinusOutlined,
  DeleteOutlined,
  CaretUpOutlined,
  CaretDownOutlined,
  SettingOutlined,
} from '@ant-design/icons';

import {
  HttpUtil,
  RandomUtil,
  NumberFormatter,
  SizeFormatter,
  Wireguard,
} from '@/utils';
import InputAddon from '@/components/InputAddon';
import { getRandomRealityTarget } from '@/models/reality-targets';
import {
  Inbound,
  Protocols,
  SSMethods,
  SNIFFING_OPTION,
  TLS_VERSION_OPTION,
  TLS_CIPHER_OPTION,
  UTLS_FINGERPRINT,
  ALPN_OPTION,
  USAGE_OPTION,
  DOMAIN_STRATEGY_OPTION,
  TCP_CONGESTION_OPTION,
  MODE_OPTION,
} from '@/models/inbound.js';
import { DBInbound } from '@/models/dbinbound.js';
import FinalMaskForm from '@/components/FinalMaskForm';
import DateTimePicker from '@/components/DateTimePicker';
import JsonEditor from '@/components/JsonEditor';
import type { NodeRecord } from '@/hooks/useNodes';
import './InboundFormModal.css';

const { TextArea } = Input;
const { Text, Paragraph } = Typography;

interface InboundFormModalProps {
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
  mode: 'add' | 'edit';
  dbInbound: any;
  dbInbounds: any[];
  availableNodes?: NodeRecord[];
}

const TRAFFIC_RESETS = ['never', 'hourly', 'daily', 'weekly', 'monthly'];
const PROTOCOLS = Object.values(Protocols) as string[];
const TLS_VERSIONS = Object.values(TLS_VERSION_OPTION) as string[];
const CIPHER_SUITES = Object.entries(TLS_CIPHER_OPTION) as [string, string][];
const FINGERPRINTS = Object.values(UTLS_FINGERPRINT) as string[];
const ALPNS = Object.values(ALPN_OPTION) as string[];
const USAGES = Object.values(USAGE_OPTION) as string[];
const DOMAIN_STRATEGIES = Object.values(DOMAIN_STRATEGY_OPTION) as string[];
const TCP_CONGESTIONS = Object.values(TCP_CONGESTION_OPTION) as string[];
const MODE_OPTIONS = Object.values(MODE_OPTION) as string[];

const NODE_ELIGIBLE_PROTOCOLS = new Set([
  Protocols.VLESS,
  Protocols.VMESS,
  Protocols.TROJAN,
  Protocols.SHADOWSOCKS,
  Protocols.HYSTERIA,
  Protocols.WIREGUARD,
]);

const FALLBACK_ELIGIBLE_TRANSPORTS = new Set(['tcp', 'ws', 'grpc', 'httpupgrade', 'xhttp']);

interface FallbackRow {
  rowKey: string;
  childId: number | null;
  name: string;
  alpn: string;
  path: string;
  xver: number;
}

function deriveFallbackDefaults(childDb: any): Omit<FallbackRow, 'rowKey' | 'childId'> {
  const out = { name: '', alpn: '', path: '', xver: 0 };
  if (!childDb) return out;
  let stream: any;
  try {
    stream = childDb.toInbound()?.stream;
  } catch {
    return out;
  }
  if (!stream) return out;
  switch (stream.network) {
    case 'tcp': {
      const tcp = stream.tcp;
      if (tcp?.type === 'http') {
        const p = tcp?.request?.path;
        if (Array.isArray(p) && p.length) out.path = p[0];
      }
      if (tcp?.acceptProxyProtocol) out.xver = 2;
      break;
    }
    case 'ws': {
      out.path = stream.ws?.path || '';
      if (stream.ws?.acceptProxyProtocol) out.xver = 2;
      break;
    }
    case 'grpc': {
      out.path = stream.grpc?.serviceName || '';
      out.alpn = 'h2';
      break;
    }
    case 'httpupgrade': {
      out.path = stream.httpupgrade?.path || '';
      if (stream.httpupgrade?.acceptProxyProtocol) out.xver = 2;
      break;
    }
    case 'xhttp': {
      out.path = stream.xhttp?.path || '';
      break;
    }
  }
  return out;
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
  const selectableNodes = useMemo(
    () => (availableNodes || []).filter((n: NodeRecord) => n.enable),
    [availableNodes],
  );

  const inboundRef = useRef<any>(null);
  const dbFormRef = useRef<any>(null);
  const fallbackKeyRef = useRef(0);
  const advancedTextRef = useRef({ stream: '', sniffing: '', settings: '' });

  const [, setTick] = useState(0);
  const refresh = useCallback(() => setTick((n) => n + 1), []);

  const [saving, setSaving] = useState(false);
  const [activeTabKey, setActiveTabKey] = useState('basic');
  const [advancedSectionKey, setAdvancedSectionKey] = useState('all');
  const [defaultCert, setDefaultCert] = useState('');
  const [defaultKey, setDefaultKey] = useState('');
  const [fallbacks, setFallbacks] = useState<FallbackRow[]>([]);
  const [fallbackEditing, setFallbackEditing] = useState<Set<string>>(new Set());

  const isVlessLike = inboundRef.current?.protocol === Protocols.VLESS;
  const isFallbackHost = useMemo(() => {
    const ib = inboundRef.current;
    if (!ib) return false;
    if (ib.protocol !== Protocols.VLESS && ib.protocol !== Protocols.TROJAN) return false;
    if (ib.stream?.network !== 'tcp') return false;
    const sec = ib.stream?.security;
    return sec === 'tls' || sec === 'reality';
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [inboundRef.current?.protocol, inboundRef.current?.stream?.network, inboundRef.current?.stream?.security]);

  const canEnableStream = inboundRef.current?.canEnableStream?.() === true;
  const canEnableTls = inboundRef.current?.canEnableTls?.() === true;
  const canEnableReality = inboundRef.current?.canEnableReality?.() === true;
  const isNodeEligible = NODE_ELIGIBLE_PROTOCOLS.has(inboundRef.current?.protocol);

  const hasProtocolTabContent = useMemo(() => {
    const ib = inboundRef.current;
    if (!ib) return false;
    if (ib.protocol === Protocols.VLESS) return true;
    if (isFallbackHost) return true;
    switch (ib.protocol) {
      case Protocols.SHADOWSOCKS:
      case Protocols.HTTP:
      case Protocols.MIXED:
      case Protocols.TUNNEL:
      case Protocols.TUN:
      case Protocols.WIREGUARD:
        return true;
      default:
        return false;
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [inboundRef.current?.protocol, isFallbackHost]);

  const externalProxyOn = Array.isArray(inboundRef.current?.stream?.externalProxy)
    && inboundRef.current.stream.externalProxy.length > 0;

  const stampAdvancedTextFor = useCallback((slice: 'stream' | 'sniffing' | 'settings') => {
    const ib = inboundRef.current;
    if (!ib) return;
    if (slice === 'stream' && !ib.canEnableStream?.()) {
      advancedTextRef.current.stream = '{}';
      return;
    }
    const obj = ib[slice];
    if (!obj) return;
    try {
      advancedTextRef.current[slice] = JSON.stringify(JSON.parse(obj.toString()), null, 2);
    } catch {
      /* keep prior */
    }
  }, []);

  const primeAdvancedJson = useCallback(() => {
    (['stream', 'sniffing', 'settings'] as const).forEach(stampAdvancedTextFor);
  }, [stampAdvancedTextFor]);

  const loadFallbacks = useCallback(async (masterId: number | null) => {
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
      (msg.obj as { childId: number; name?: string; alpn?: string; path?: string; xver?: number }[]).map((r) => ({
        rowKey: `fb-${++fallbackKeyRef.current}`,
        childId: r.childId,
        name: r.name || '',
        alpn: r.alpn || '',
        path: r.path || '',
        xver: r.xver || 0,
      })),
    );
  }, []);

  const fetchDefaultCertSettings = useCallback(async () => {
    try {
      const msg = await HttpUtil.post('/panel/setting/defaultSettings');
      if (msg?.success && msg.obj) {
        const obj = msg.obj as { defaultCert?: string; defaultKey?: string };
        setDefaultCert(obj.defaultCert || '');
        setDefaultKey(obj.defaultKey || '');
      }
    } catch {
      /* non-fatal */
    }
  }, []);

  useEffect(() => {
    if (!open) return;
    setFallbackEditing(new Set());
    if (mode === 'edit' && dbInbound) {
      const parsed = (Inbound as any).fromJson(dbInbound.toInbound().toJson());
      inboundRef.current = parsed;
      dbFormRef.current = new (DBInbound as any)(dbInbound);
      primeAdvancedJson();
      if (dbInbound.protocol === Protocols.VLESS || dbInbound.protocol === Protocols.TROJAN) {
        loadFallbacks(dbInbound.id);
      } else {
        setFallbacks([]);
      }
    } else {
      const ib = new (Inbound as any)();
      ib.protocol = Protocols.VLESS;
      ib.settings = (Inbound as any).Settings.getSettings(Protocols.VLESS);
      ib.port = RandomUtil.randomInteger(10000, 60000);
      inboundRef.current = ib;
      const form = new (DBInbound as any)();
      form.enable = true;
      form.remark = '';
      form.total = 0;
      form.expiryTime = 0;
      form.trafficReset = 'never';
      dbFormRef.current = form;
      primeAdvancedJson();
      setFallbacks([]);
    }
    setActiveTabKey('basic');
    setAdvancedSectionKey('all');
    fetchDefaultCertSettings();
    refresh();
  }, [open, mode, dbInbound, primeAdvancedJson, loadFallbacks, fetchDefaultCertSettings, refresh]);

  const setExternalProxy = useCallback((on: boolean) => {
    const ib = inboundRef.current;
    if (!ib?.stream) return;
    if (on) {
      ib.stream.externalProxy = [{
        forceTls: 'same',
        dest: window.location.hostname,
        port: ib.port,
        remark: '',
      }];
    } else {
      ib.stream.externalProxy = [];
    }
    refresh();
  }, [refresh]);

  const onProtocolChange = useCallback((next: string) => {
    const ib = inboundRef.current;
    if (mode === 'edit' || !ib) return;
    ib.protocol = next;
    ib.settings = (Inbound as any).Settings.getSettings(next);
    if (!NODE_ELIGIBLE_PROTOCOLS.has(next) && dbFormRef.current) {
      dbFormRef.current.nodeId = null;
    }
    primeAdvancedJson();
    refresh();
  }, [mode, primeAdvancedJson, refresh]);

  const onNetworkChange = useCallback((next: string) => {
    const ib = inboundRef.current;
    if (!ib?.stream) return;
    ib.stream.network = next;
    if (!ib.canEnableTls()) ib.stream.security = 'none';
    if (!ib.canEnableReality()) ib.reality = false;
    if (
      ib.protocol === Protocols.VLESS
      && !ib.canEnableTlsFlow()
      && Array.isArray(ib.settings.vlesses)
    ) {
      ib.settings.vlesses.forEach((c: any) => { c.flow = ''; });
    }
    if (next !== 'kcp' && ib.stream.finalmask) {
      ib.stream.finalmask.udp = [];
    }
    stampAdvancedTextFor('stream');
    refresh();
  }, [stampAdvancedTextFor, refresh]);

  const setSecurity = useCallback((v: string) => {
    const ib = inboundRef.current;
    if (ib?.stream) {
      ib.stream.security = v;
      refresh();
    }
  }, [refresh]);

  const addFallback = useCallback((childId: number | null = null) => {
    const row: FallbackRow = {
      rowKey: `fb-${++fallbackKeyRef.current}`,
      childId: childId || null,
      name: '',
      alpn: '',
      path: '',
      xver: 0,
    };
    if (childId) {
      const child = (dbInbounds || []).find((ib: any) => ib.id === childId);
      Object.assign(row, deriveFallbackDefaults(child));
    }
    setFallbacks((prev) => [...prev, row]);
  }, [dbInbounds]);

  const removeFallback = useCallback((idx: number) => {
    setFallbacks((prev) => prev.filter((_, i) => i !== idx));
  }, []);

  const moveFallback = useCallback((idx: number, dir: number) => {
    setFallbacks((prev) => {
      const arr = [...prev];
      const j = idx + dir;
      if (j < 0 || j >= arr.length) return prev;
      [arr[idx], arr[j]] = [arr[j], arr[idx]];
      return arr;
    });
  }, []);

  const onFallbackChildPicked = useCallback((rowKey: string, childId: number) => {
    setFallbacks((prev) => prev.map((row) => {
      if (row.rowKey !== rowKey) return row;
      const child = (dbInbounds || []).find((ib: any) => ib.id === childId);
      const defaults = deriveFallbackDefaults(child);
      return { ...row, childId, ...defaults };
    }));
  }, [dbInbounds]);

  const updateFallback = useCallback((rowKey: string, patch: Partial<FallbackRow>) => {
    setFallbacks((prev) => prev.map((row) => (row.rowKey === rowKey ? { ...row, ...patch } : row)));
  }, []);

  const rederiveFallback = useCallback((rowKey: string) => {
    setFallbacks((prev) => prev.map((row) => {
      if (row.rowKey !== rowKey || !row.childId) return row;
      const child = (dbInbounds || []).find((ib: any) => ib.id === row.childId);
      const defaults = deriveFallbackDefaults(child);
      return { ...row, ...defaults };
    }));
    messageApi.success(t('pages.inbounds.fallbacks.rederived') || 'Re-filled from child');
  }, [dbInbounds, t, messageApi]);

  const quickAddAllFallbacks = useCallback(() => {
    const masterId = dbInbound?.id;
    const list = dbInbounds || [];
    setFallbacks((prev) => {
      const existing = new Set(prev.map((r) => r.childId).filter(Boolean));
      const next = [...prev];
      let added = 0;
      for (const ib of list) {
        if (ib.id === masterId) continue;
        if (existing.has(ib.id)) continue;
        let stream: any;
        try { stream = ib.toInbound()?.stream; } catch { continue; }
        if (!stream || !FALLBACK_ELIGIBLE_TRANSPORTS.has(stream.network)) continue;
        const row: FallbackRow = {
          rowKey: `fb-${++fallbackKeyRef.current}`,
          childId: ib.id,
          ...deriveFallbackDefaults(ib),
        };
        next.push(row);
        added += 1;
      }
      if (added > 0) {
        messageApi.success(t('pages.inbounds.fallbacks.quickAdded', { n: added }) || `Added ${added} fallback(s)`);
      } else {
        messageApi.info(t('pages.inbounds.fallbacks.quickAddedNone') || 'No new eligible inbounds to add');
      }
      return next;
    });
  }, [dbInbound, dbInbounds, t, messageApi]);

  const fallbackChildOptions = useMemo(() => {
    const list = dbInbounds || [];
    const masterId = dbInbound?.id;
    return list
      .filter((ib: any) => ib.id !== masterId)
      .map((ib: any) => ({
        label: `${ib.remark || `#${ib.id}`} · ${ib.protocol}:${ib.port}`,
        value: ib.id,
      }));
  }, [dbInbounds, dbInbound]);

  const toggleFallbackEdit = useCallback((rowKey: string) => {
    setFallbackEditing((prev) => {
      const next = new Set(prev);
      if (next.has(rowKey)) next.delete(rowKey); else next.add(rowKey);
      return next;
    });
  }, []);

  const describeFallback = useCallback((record: FallbackRow) => {
    const parts: string[] = [];
    if (record.name) parts.push(`SNI=${record.name}`);
    if (record.alpn) parts.push(`ALPN=${record.alpn}`);
    if (record.path) parts.push(`path=${record.path}`);
    const condition = parts.length
      ? `${t('pages.inbounds.fallbacks.routesWhen') || 'Routes when'} ${parts.join(' · ')}`
      : (t('pages.inbounds.fallbacks.defaultCatchAll') || 'Default — catches anything else');
    const proxyTag = record.xver === 2 ? ' · PROXY v2' : record.xver === 1 ? ' · PROXY v1' : '';
    return { condition, proxyTag };
  }, [t]);

  const withSaving = useCallback(async <T,>(fn: () => Promise<T>): Promise<T> => {
    setSaving(true);
    try { return await fn(); } finally { setSaving(false); }
  }, []);

  const randomSSPassword = useCallback((target: any) => {
    if (target) {
      target.password = (RandomUtil as any).randomShadowsocksPassword(inboundRef.current.settings.method);
      refresh();
    }
  }, [refresh]);

  const regenWgKeypair = useCallback((target: any) => {
    const kp = (Wireguard as any).generateKeypair();
    target.publicKey = kp.publicKey;
    target.privateKey = kp.privateKey;
    refresh();
  }, [refresh]);

  const regenInboundWg = useCallback(() => {
    const kp = (Wireguard as any).generateKeypair();
    inboundRef.current.settings.pubKey = kp.publicKey;
    inboundRef.current.settings.secretKey = kp.privateKey;
    refresh();
  }, [refresh]);

  const genRealityKeypair = useCallback(async () => {
    await withSaving(async () => {
      const msg = await HttpUtil.get('/panel/api/server/getNewX25519Cert');
      if (msg?.success) {
        const obj = msg.obj as { privateKey: string; publicKey: string };
        inboundRef.current.stream.reality.privateKey = obj.privateKey;
        inboundRef.current.stream.reality.settings.publicKey = obj.publicKey;
        refresh();
      }
    });
  }, [withSaving, refresh]);

  const clearRealityKeypair = useCallback(() => {
    if (!inboundRef.current?.stream?.reality) return;
    inboundRef.current.stream.reality.privateKey = '';
    inboundRef.current.stream.reality.settings.publicKey = '';
    refresh();
  }, [refresh]);

  const genMldsa65 = useCallback(async () => {
    await withSaving(async () => {
      const msg = await HttpUtil.get('/panel/api/server/getNewmldsa65');
      if (msg?.success) {
        const obj = msg.obj as { seed: string; verify: string };
        inboundRef.current.stream.reality.mldsa65Seed = obj.seed;
        inboundRef.current.stream.reality.settings.mldsa65Verify = obj.verify;
        refresh();
      }
    });
  }, [withSaving, refresh]);

  const clearMldsa65 = useCallback(() => {
    if (!inboundRef.current?.stream?.reality) return;
    inboundRef.current.stream.reality.mldsa65Seed = '';
    inboundRef.current.stream.reality.settings.mldsa65Verify = '';
    refresh();
  }, [refresh]);

  const randomizeRealityTarget = useCallback(() => {
    if (!inboundRef.current?.stream?.reality) return;
    const target = getRandomRealityTarget() as { target: string; sni: string };
    inboundRef.current.stream.reality.target = target.target;
    inboundRef.current.stream.reality.serverNames = target.sni;
    refresh();
  }, [refresh]);

  const randomizeShortIds = useCallback(() => {
    if (!inboundRef.current?.stream?.reality) return;
    inboundRef.current.stream.reality.shortIds = (RandomUtil as any).randomShortIds();
    refresh();
  }, [refresh]);

  const getNewEchCert = useCallback(async () => {
    if (!inboundRef.current?.stream?.tls) return;
    await withSaving(async () => {
      const msg = await HttpUtil.post('/panel/api/server/getNewEchCert', {
        sni: inboundRef.current.stream.tls.sni,
      });
      if (msg?.success) {
        const obj = msg.obj as { echServerKeys: string; echConfigList: string };
        inboundRef.current.stream.tls.echServerKeys = obj.echServerKeys;
        inboundRef.current.stream.tls.settings.echConfigList = obj.echConfigList;
        refresh();
      }
    });
  }, [withSaving, refresh]);

  const clearEchCert = useCallback(() => {
    if (!inboundRef.current?.stream?.tls) return;
    inboundRef.current.stream.tls.echServerKeys = '';
    inboundRef.current.stream.tls.settings.echConfigList = '';
    refresh();
  }, [refresh]);

  const setDefaultCertData = useCallback((idx: number) => {
    if (!inboundRef.current?.stream?.tls?.certs?.[idx]) return;
    inboundRef.current.stream.tls.certs[idx].certFile = defaultCert;
    inboundRef.current.stream.tls.certs[idx].keyFile = defaultKey;
    refresh();
  }, [defaultCert, defaultKey, refresh]);

  const matchesVlessAuth = useCallback((block: any, authId: string) => {
    if (block?.id === authId) return true;
    const label = (block?.label || '').toLowerCase().replace(/[-_\s]/g, '');
    if (authId === 'mlkem768') return label.includes('mlkem768');
    if (authId === 'x25519') return label.includes('x25519');
    return false;
  }, []);

  const getNewVlessEnc = useCallback(async (authId: string) => {
    if (!authId || !inboundRef.current?.settings) return;
    await withSaving(async () => {
      const msg = await HttpUtil.get('/panel/api/server/getNewVlessEnc');
      if (!msg?.success) return;
      const obj = msg.obj as { auths?: { decryption: string; encryption: string; label?: string; id?: string }[] };
      const block = (obj.auths || []).find((a) => matchesVlessAuth(a, authId));
      if (!block) return;
      inboundRef.current.settings.decryption = block.decryption;
      inboundRef.current.settings.encryption = block.encryption;
      refresh();
    });
  }, [withSaving, refresh, matchesVlessAuth]);

  const clearVlessEnc = useCallback(() => {
    if (!inboundRef.current?.settings) return;
    inboundRef.current.settings.decryption = 'none';
    inboundRef.current.settings.encryption = 'none';
    refresh();
  }, [refresh]);

  const selectedVlessAuth = useMemo(() => {
    const encryption = inboundRef.current?.settings?.encryption;
    if (!encryption || encryption === 'none') return 'None';
    const parts = encryption.split('.').filter(Boolean);
    const authKey = parts[parts.length - 1] || '';
    if (!authKey) return t('pages.inbounds.vlessAuthCustom');
    return authKey.length > 300
      ? t('pages.inbounds.vlessAuthMlkem768')
      : t('pages.inbounds.vlessAuthX25519');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [inboundRef.current?.settings?.encryption, t]);

  const onSSMethodChange = useCallback(() => {
    const ib = inboundRef.current;
    ib.settings.password = (RandomUtil as any).randomShadowsocksPassword(ib.settings.method);
    if (ib.isSSMultiUser) {
      ib.settings.shadowsockses.forEach((c: any) => {
        c.method = ib.isSS2022 ? '' : ib.settings.method;
        c.password = (RandomUtil as any).randomShadowsocksPassword(ib.settings.method);
      });
    } else {
      ib.settings.shadowsockses = [];
    }
    refresh();
  }, [refresh]);

  const parseAdvancedSliceOrFallback = (rawText: string, fallback: unknown) => {
    if (!rawText?.trim()) return fallback;
    return JSON.parse(rawText);
  };

  const settingsFallback = () => inboundRef.current?.settings?.toJson?.() || {};
  const sniffingFallback = () => inboundRef.current?.sniffing?.toJson?.() || {};
  const streamFallback = () => inboundRef.current?.stream?.toJson?.() || {};

  const parseAdvancedSliceWithLabel = useCallback((rawText: string, fallback: unknown, label: string) => {
    try {
      return parseAdvancedSliceOrFallback(rawText, fallback);
    } catch (e) {
      messageApi.error(`${label} JSON invalid: ${(e as Error).message}`);
      throw e;
    }
  }, [messageApi]);

  const compactAdvancedJson = useCallback((raw: string, fallback: string, label: string) => {
    try {
      return JSON.stringify(JSON.parse(raw || fallback));
    } catch (e) {
      messageApi.error(`${label} JSON invalid: ${(e as Error).message}`);
      throw e;
    }
  }, [messageApi]);

  const applyAdvancedJsonToBasic = useCallback((): boolean => {
    const ib = inboundRef.current;
    if (!ib) return true;
    let settings: unknown;
    let streamSettings: unknown;
    let sniffing: unknown;
    try {
      settings = parseAdvancedSliceWithLabel(advancedTextRef.current.settings, settingsFallback(), t('pages.inbounds.advanced.settings'));
      streamSettings = parseAdvancedSliceWithLabel(advancedTextRef.current.stream, streamFallback(), t('pages.inbounds.advanced.stream'));
      sniffing = parseAdvancedSliceWithLabel(advancedTextRef.current.sniffing, sniffingFallback(), t('pages.inbounds.advanced.sniffing'));
    } catch {
      return false;
    }
    try {
      inboundRef.current = (Inbound as any).fromJson({
        port: ib.port,
        listen: ib.listen,
        protocol: ib.protocol,
        settings,
        streamSettings,
        tag: ib.tag,
        sniffing,
        clientStats: ib.clientStats,
      });
      refresh();
    } catch (e) {
      messageApi.error(`${t('pages.inbounds.advanced.jsonErrorPrefix')}: ${(e as Error).message}`);
      return false;
    }
    return true;
  }, [t, refresh, parseAdvancedSliceWithLabel, messageApi]);

  const handleTabChange = (next: string) => {
    if (document.activeElement instanceof HTMLElement) {
      document.activeElement.blur();
    }
    if (activeTabKey === 'advanced' && next !== 'advanced') {
      if (!applyAdvancedJsonToBasic()) return;
    }
    setActiveTabKey(next);
  };

  const unwrapWrappedObject = (parsed: unknown, key: string): unknown => {
    if (
      parsed
      && typeof parsed === 'object'
      && !Array.isArray(parsed)
      && (parsed as Record<string, unknown>)[key] !== undefined
    ) {
      return (parsed as Record<string, unknown>)[key];
    }
    return parsed;
  };

  const wrappedConfigValue = (key: string, slice: 'stream' | 'sniffing' | 'settings'): string => {
    const ib = inboundRef.current;
    if (!ib) return '';
    try {
      const fb = slice === 'settings' ? settingsFallback() : slice === 'sniffing' ? sniffingFallback() : streamFallback();
      const value = parseAdvancedSliceOrFallback(advancedTextRef.current[slice], fb);
      return JSON.stringify({ [key]: value }, null, 2);
    } catch {
      return '';
    }
  };

  const setWrappedConfigValue = (key: string, slice: 'stream' | 'sniffing' | 'settings', label: string, next: string) => {
    let parsed: unknown;
    try {
      parsed = JSON.parse(next);
    } catch (e) {
      messageApi.error(`${label} JSON invalid: ${(e as Error).message}`);
      return;
    }
    const unwrapped = unwrapWrappedObject(parsed, key);
    if (!unwrapped || typeof unwrapped !== 'object' || Array.isArray(unwrapped)) {
      messageApi.error(`${label} JSON must be an object or { ${key}: { ... } }.`);
      return;
    }
    try {
      advancedTextRef.current[slice] = JSON.stringify(unwrapped, null, 2);
      refresh();
    } catch (e) {
      messageApi.error(`${label} JSON invalid: ${(e as Error).message}`);
    }
  };

  const advancedAllValue = (() => {
    const ib = inboundRef.current;
    if (!ib) return '';
    try {
      const result: Record<string, unknown> = {
        listen: ib.listen,
        port: ib.port,
        protocol: ib.protocol,
        settings: parseAdvancedSliceOrFallback(advancedTextRef.current.settings, settingsFallback()),
        sniffing: parseAdvancedSliceOrFallback(advancedTextRef.current.sniffing, sniffingFallback()),
        tag: ib.tag,
      };
      if (canEnableStream) {
        result.streamSettings = parseAdvancedSliceOrFallback(advancedTextRef.current.stream, streamFallback());
      }
      return JSON.stringify(result, null, 2);
    } catch {
      return '';
    }
  })();

  const setAdvancedAllValue = (next: string) => {
    let parsed: any;
    try {
      parsed = JSON.parse(next);
    } catch (e) {
      messageApi.error(`All JSON invalid: ${(e as Error).message}`);
      return;
    }
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      messageApi.error('All JSON must be an inbound object.');
      return;
    }
    const ib = inboundRef.current;
    try {
      if (typeof parsed.listen === 'string') ib.listen = parsed.listen;
      if (parsed.port !== undefined) {
        const port = Number(parsed.port);
        if (Number.isFinite(port)) ib.port = port;
      }
      if (typeof parsed.protocol === 'string' && PROTOCOLS.includes(parsed.protocol)) {
        ib.protocol = parsed.protocol;
      }
      if (typeof parsed.tag === 'string') ib.tag = parsed.tag;

      const existingSettings = parseAdvancedSliceOrFallback(advancedTextRef.current.settings, settingsFallback());
      advancedTextRef.current.settings = JSON.stringify(parsed.settings ?? existingSettings, null, 2);
      advancedTextRef.current.sniffing = JSON.stringify(parsed.sniffing ?? sniffingFallback(), null, 2);
      advancedTextRef.current.stream = canEnableStream
        ? JSON.stringify(parsed.streamSettings ?? streamFallback(), null, 2)
        : '{}';
      refresh();
    } catch (e) {
      messageApi.error(`All JSON invalid: ${(e as Error).message}`);
    }
  };

  const saveFallbacks = useCallback(async (masterId: number) => {
    if (!masterId) return true;
    const payload = {
      fallbacks: fallbacks
        .filter((c) => c.childId)
        .map((c, i) => ({
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
  }, [fallbacks]);

  const submit = useCallback(async () => {
    const ib = inboundRef.current;
    const form = dbFormRef.current;
    if (!ib || !form) return;
    setSaving(true);
    try {
      let streamSettings: string;
      let sniffing: string;
      let settings: string;
      try {
        streamSettings = canEnableStream
          ? compactAdvancedJson(advancedTextRef.current.stream, '', t('pages.inbounds.advanced.stream'))
          : (ib.stream?.sockopt
            ? JSON.stringify({ sockopt: ib.stream.sockopt.toJson() })
            : '');
        sniffing = compactAdvancedJson(advancedTextRef.current.sniffing, ib.sniffing.toString(), t('pages.inbounds.advanced.sniffing'));
        settings = compactAdvancedJson(advancedTextRef.current.settings, ib.settings.toString(), t('pages.inbounds.advanced.settings'));
      } catch { return; }

      const payload: any = {
        up: form.up || 0,
        down: form.down || 0,
        total: form.total,
        remark: form.remark,
        enable: form.enable,
        expiryTime: form.expiryTime,
        trafficReset: form.trafficReset,
        lastTrafficResetTime: form.lastTrafficResetTime || 0,
        listen: ib.listen,
        port: ib.port,
        protocol: ib.protocol,
        settings,
        streamSettings,
        sniffing,
      };
      if (form.nodeId != null) payload.nodeId = form.nodeId;

      const url = mode === 'edit'
        ? `/panel/api/inbounds/update/${dbInbound.id}`
        : '/panel/api/inbounds/add';
      const msg = await HttpUtil.post(url, payload);
      if (msg?.success) {
        if (isFallbackHost) {
          const masterId = mode === 'edit'
            ? dbInbound.id
            : ((msg.obj as any)?.id || (msg.obj as any)?.Id);
          if (masterId) await saveFallbacks(masterId);
        }
        onSaved();
        onClose();
      }
    } finally {
      setSaving(false);
    }
  }, [canEnableStream, compactAdvancedJson, t, mode, dbInbound, isFallbackHost, saveFallbacks, onSaved, onClose]);

  const protocolSnapshot = inboundRef.current?.protocol;
  const streamSnapshot = JSON.stringify(inboundRef.current?.stream?.toJson?.() || {});
  const sniffingSnapshot = JSON.stringify(inboundRef.current?.sniffing?.toJson?.() || {});
  const settingsSnapshot = JSON.stringify(inboundRef.current?.settings?.toJson?.() || {});

  useEffect(() => {
    if (!inboundRef.current) return;
    (['stream', 'sniffing', 'settings'] as const).forEach(stampAdvancedTextFor);
  }, [protocolSnapshot, streamSnapshot, sniffingSnapshot, settingsSnapshot, stampAdvancedTextFor]);

  const title = mode === 'edit' ? t('pages.inbounds.modifyInbound') : t('pages.inbounds.addInbound');
  const okText = mode === 'edit' ? t('pages.clients.submitEdit') : t('create');

  const ib = inboundRef.current;
  const form = dbFormRef.current;
  if (!ib || !form) {
    return <Modal open={open} onCancel={onClose} title={title} footer={null} width={780} />;
  }

  const totalGB = form.total ? Math.round((form.total / SizeFormatter.ONE_GB) * 100) / 100 : 0;
  const expiryDate: Dayjs | null = form.expiryTime > 0 ? dayjs(form.expiryTime) : null;

  const renderBasicsTab = () => (
    <Form colon={false} labelCol={{ sm: { span: 8 } }} wrapperCol={{ sm: { span: 14 } }}>
      <Form.Item label={t('enable')}>
        <Switch checked={!!form.enable} onChange={(v) => { form.enable = v; refresh(); }} />
      </Form.Item>
      <Form.Item label={t('pages.inbounds.remark')}>
        <Input value={form.remark} onChange={(e) => { form.remark = e.target.value; refresh(); }} />
      </Form.Item>
      {selectableNodes.length > 0 && isNodeEligible && (
        <Form.Item label={t('pages.inbounds.deployTo')}>
          <Select
            value={form.nodeId ?? ''}
            disabled={mode === 'edit'}
            placeholder={t('pages.inbounds.localPanel')}
            allowClear
            onChange={(v) => { form.nodeId = v === '' || v == null ? null : v; refresh(); }}
          >
            <Select.Option value="">{t('pages.inbounds.localPanel')}</Select.Option>
            {selectableNodes.map((n: NodeRecord) => (
              <Select.Option key={n.id} value={n.id} disabled={n.status === 'offline'}>
                {n.name}{n.status === 'offline' ? ' (offline)' : ''}
              </Select.Option>
            ))}
          </Select>
        </Form.Item>
      )}
      <Form.Item label={t('pages.inbounds.protocol')}>
        <Select
          value={ib.protocol}
          disabled={mode === 'edit'}
          onChange={onProtocolChange}
        >
          {PROTOCOLS.map((p) => <Select.Option key={p} value={p}>{p}</Select.Option>)}
        </Select>
      </Form.Item>
      <Form.Item label={t('pages.inbounds.address')}>
        <Input
          value={ib.listen}
          placeholder={t('pages.inbounds.monitorDesc')}
          onChange={(e) => { ib.listen = e.target.value; refresh(); }}
        />
      </Form.Item>
      <Form.Item label={t('pages.inbounds.port')}>
        <InputNumber
          value={ib.port}
          min={1}
          max={65535}
          onChange={(v) => { ib.port = Number(v) || 0; refresh(); }}
        />
      </Form.Item>
      <Form.Item label={<Tooltip title={t('pages.inbounds.meansNoLimit')}>{t('pages.inbounds.totalFlow')}</Tooltip>}>
        <InputNumber
          value={totalGB}
          min={0}
          step={1}
          onChange={(v) => {
            form.total = NumberFormatter.toFixed((Number(v) || 0) * SizeFormatter.ONE_GB, 0);
            refresh();
          }}
        />
      </Form.Item>
      <Form.Item label={t('pages.inbounds.periodicTrafficResetTitle')}>
        <Select value={form.trafficReset} onChange={(v) => { form.trafficReset = v; refresh(); }}>
          {TRAFFIC_RESETS.map((r) => (
            <Select.Option key={r} value={r}>{t(`pages.inbounds.periodicTrafficReset.${r}`)}</Select.Option>
          ))}
        </Select>
      </Form.Item>
      <Form.Item label={<Tooltip title={t('pages.inbounds.leaveBlankToNeverExpire')}>{t('pages.inbounds.expireDate')}</Tooltip>}>
        <DateTimePicker
          value={expiryDate}
          onChange={(d) => { form.expiryTime = d ? d.valueOf() : 0; refresh(); }}
        />
      </Form.Item>
    </Form>
  );

  const renderFallbacksCard = () => (
    <Card size="small" className="mt-12" title={t('pages.inbounds.fallbacks.title') || 'Fallbacks'}>
      <Paragraph type="secondary" style={{ marginBottom: 12 }}>
        {t('pages.inbounds.fallbacks.help') || 'When a connection on this inbound does not match any client, route it to another inbound. Pick a child below and the routing fields (SNI / ALPN / path / xver) auto-fill from its transport — most setups need no further tweaking. Each child should listen on 127.0.0.1 with security=none.'}
      </Paragraph>
      {fallbacks.length === 0 && (
        <Empty description={t('pages.inbounds.fallbacks.empty') || 'No fallbacks yet'} styles={{ image: { height: 40 } }} style={{ margin: '8px 0 12px' }} />
      )}
      {fallbacks.map((record, index) => (
        <div key={record.rowKey} style={{ border: '1px solid var(--app-border-tertiary)', borderRadius: 6, padding: '10px 12px', marginBottom: 8 }}>
          <Row gutter={8} align="middle" wrap={false}>
            <Col flex="none">
              <Space orientation="vertical" size={2}>
                <Button size="small" type="text" disabled={index === 0} onClick={() => moveFallback(index, -1)}>
                  <CaretUpOutlined />
                </Button>
                <Button size="small" type="text" disabled={index === fallbacks.length - 1} onClick={() => moveFallback(index, 1)}>
                  <CaretDownOutlined />
                </Button>
              </Space>
            </Col>
            <Col flex="auto">
              <Select
                value={record.childId}
                options={fallbackChildOptions}
                showSearch
                placeholder={t('pages.inbounds.fallbacks.pickInbound') || 'Pick an inbound'}
                filterOption={(input, option) => ((option?.label as string) || '').toLowerCase().includes(input.toLowerCase())}
                style={{ width: '100%' }}
                onChange={(v) => onFallbackChildPicked(record.rowKey, v)}
              />
              <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 4 }}>
                {describeFallback(record).condition}{describeFallback(record).proxyTag}
              </Text>
            </Col>
            <Col flex="none">
              <Space size={4}>
                <Tooltip title={t('pages.inbounds.fallbacks.rederive') || 'Re-fill from child'}>
                  <Button size="small" type="text" disabled={!record.childId} onClick={() => rederiveFallback(record.rowKey)}>
                    <SyncOutlined />
                  </Button>
                </Tooltip>
                <Tooltip title={fallbackEditing.has(record.rowKey)
                  ? (t('pages.inbounds.fallbacks.hideAdvanced') || 'Hide advanced')
                  : (t('pages.inbounds.fallbacks.editAdvanced') || 'Edit routing fields')}>
                  <Button size="small" type="text" onClick={() => toggleFallbackEdit(record.rowKey)}>
                    <SettingOutlined />
                  </Button>
                </Tooltip>
                <Button size="small" type="text" danger onClick={() => removeFallback(index)}>
                  <DeleteOutlined />
                </Button>
              </Space>
            </Col>
          </Row>
          {fallbackEditing.has(record.rowKey) && (
            <Row gutter={8} style={{ marginTop: 8 }}>
              <Col xs={24} md={8}>
                <Space.Compact block>
                  <InputAddon>SNI</InputAddon>
                  <Input placeholder={t('pages.inbounds.fallbacks.matchAny') || 'any'}
                    value={record.name} onChange={(e) => updateFallback(record.rowKey, { name: e.target.value })} />
                </Space.Compact>
              </Col>
              <Col xs={24} md={5}>
                <Space.Compact block>
                  <InputAddon>ALPN</InputAddon>
                  <Input placeholder={t('pages.inbounds.fallbacks.matchAny') || 'any'}
                    value={record.alpn} onChange={(e) => updateFallback(record.rowKey, { alpn: e.target.value })} />
                </Space.Compact>
              </Col>
              <Col xs={24} md={7}>
                <Space.Compact block>
                  <InputAddon>Path</InputAddon>
                  <Input placeholder="/" value={record.path}
                    onChange={(e) => updateFallback(record.rowKey, { path: e.target.value })} />
                </Space.Compact>
              </Col>
              <Col xs={24} md={4}>
                <Space.Compact block>
                  <InputAddon>xver</InputAddon>
                  <InputNumber min={0} max={2} style={{ width: '100%' }}
                    value={record.xver}
                    onChange={(v) => updateFallback(record.rowKey, { xver: Number(v) || 0 })} />
                </Space.Compact>
              </Col>
            </Row>
          )}
        </div>
      ))}
      <Space size={8} style={{ marginTop: 4 }} wrap>
        <Button size="small" onClick={() => addFallback()}>
          <PlusOutlined /> {t('pages.inbounds.fallbacks.add') || 'Add fallback'}
        </Button>
        <Button size="small" type="primary" ghost onClick={quickAddAllFallbacks}>
          {t('pages.inbounds.fallbacks.quickAddAll') || 'Quick add all eligible'}
        </Button>
      </Space>
    </Card>
  );

  const renderProtocolTab = () => (
    <>
      {isVlessLike && (
        <Form colon={false} labelCol={{ sm: { span: 8 } }} wrapperCol={{ sm: { span: 14 } }} className="mt-12">
          <Form.Item label={t('pages.inbounds.decryption')}>
            <Input value={ib.settings.decryption} onChange={(e) => { ib.settings.decryption = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label={t('pages.inbounds.encryption')}>
            <Input value={ib.settings.encryption} onChange={(e) => { ib.settings.encryption = e.target.value; refresh(); }} />
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
        </Form>
      )}

      {isFallbackHost && renderFallbacksCard()}

      {ib.protocol === Protocols.SHADOWSOCKS && (
        <Form colon={false} labelCol={{ sm: { span: 8 } }} wrapperCol={{ sm: { span: 14 } }} className="mt-12">
          <Form.Item label="Encryption method">
            <Select value={ib.settings.method} onChange={(v) => { ib.settings.method = v; onSSMethodChange(); }}>
              {Object.entries(SSMethods).map(([k, m]) => (
                <Select.Option key={k} value={m as string}>{k}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          {ib.isSS2022 && (
            <Form.Item label={<>Password <SyncOutlined className="random-icon" onClick={() => randomSSPassword(ib.settings)} /></>}>
              <Input value={ib.settings.password} onChange={(e) => { ib.settings.password = e.target.value; refresh(); }} />
            </Form.Item>
          )}
          <Form.Item label="Network">
            <Select value={ib.settings.network} style={{ width: 120 }} onChange={(v) => { ib.settings.network = v; refresh(); }}>
              <Select.Option value="tcp,udp">TCP, UDP</Select.Option>
              <Select.Option value="tcp">TCP</Select.Option>
              <Select.Option value="udp">UDP</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item label="ivCheck">
            <Switch checked={!!ib.settings.ivCheck} onChange={(v) => { ib.settings.ivCheck = v; refresh(); }} />
          </Form.Item>
        </Form>
      )}

      {(ib.protocol === Protocols.HTTP || ib.protocol === Protocols.MIXED) && (
        <Form colon={false} labelCol={{ sm: { span: 8 } }} wrapperCol={{ sm: { span: 14 } }} className="mt-12">
          <Form.Item label="Accounts">
            <Button size="small" onClick={() => {
              const Account = ib.protocol === Protocols.HTTP
                ? (Inbound as any).HttpSettings.HttpAccount
                : (Inbound as any).MixedSettings.SocksAccount;
              ib.settings.addAccount(new Account());
              refresh();
            }}>
              <PlusOutlined /> Add
            </Button>
          </Form.Item>
          <Form.Item wrapperCol={{ span: 24 }}>
            {(ib.settings.accounts || []).map((account: any, idx: number) => (
              <Space.Compact key={idx} className="mb-8" block>
                <InputAddon>{String(idx + 1)}</InputAddon>
                <Input value={account.user} placeholder="Username"
                  onChange={(e) => { account.user = e.target.value; refresh(); }} />
                <Input value={account.pass} placeholder="Password"
                  onChange={(e) => { account.pass = e.target.value; refresh(); }} />
                <Button onClick={() => { ib.settings.delAccount(idx); refresh(); }}>
                  <MinusOutlined />
                </Button>
              </Space.Compact>
            ))}
          </Form.Item>
          {ib.protocol === Protocols.HTTP && (
            <Form.Item label="Allow transparent">
              <Switch checked={!!ib.settings.allowTransparent} onChange={(v) => { ib.settings.allowTransparent = v; refresh(); }} />
            </Form.Item>
          )}
          {ib.protocol === Protocols.MIXED && (
            <>
              <Form.Item label="Auth">
                <Select value={ib.settings.auth} onChange={(v) => { ib.settings.auth = v; refresh(); }}>
                  <Select.Option value="noauth">noauth</Select.Option>
                  <Select.Option value="password">password</Select.Option>
                </Select>
              </Form.Item>
              <Form.Item label="UDP">
                <Switch checked={!!ib.settings.udp} onChange={(v) => { ib.settings.udp = v; refresh(); }} />
              </Form.Item>
              {ib.settings.udp && (
                <Form.Item label="UDP IP">
                  <Input value={ib.settings.ip} onChange={(e) => { ib.settings.ip = e.target.value; refresh(); }} />
                </Form.Item>
              )}
            </>
          )}
        </Form>
      )}

      {ib.protocol === Protocols.TUNNEL && (
        <Form colon={false} labelCol={{ sm: { span: 8 } }} wrapperCol={{ sm: { span: 14 } }} className="mt-12">
          <Form.Item label="Rewrite address">
            <Input value={ib.settings.rewriteAddress} onChange={(e) => { ib.settings.rewriteAddress = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="Rewrite port">
            <InputNumber value={ib.settings.rewritePort} min={0} max={65535}
              onChange={(v) => { ib.settings.rewritePort = Number(v) || 0; refresh(); }} />
          </Form.Item>
          <Form.Item label="Allowed network">
            <Select value={ib.settings.allowedNetwork} onChange={(v) => { ib.settings.allowedNetwork = v; refresh(); }}>
              <Select.Option value="tcp,udp">TCP, UDP</Select.Option>
              <Select.Option value="tcp">TCP</Select.Option>
              <Select.Option value="udp">UDP</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item label="Port map">
            <Button size="small" onClick={() => { ib.settings.addPortMap('', ''); refresh(); }}>
              <PlusOutlined />
            </Button>
          </Form.Item>
          {(ib.settings.portMap || []).length > 0 && (
            <Form.Item wrapperCol={{ span: 24 }}>
              {(ib.settings.portMap as { name: string; value: string }[]).map((pm, idx) => (
                <Space.Compact key={`pm-${idx}`} className="mb-8" block>
                  <InputAddon>{String(idx + 1)}</InputAddon>
                  <Input value={pm.name} placeholder="5555"
                    onChange={(e) => { pm.name = e.target.value; refresh(); }} />
                  <Input value={pm.value} placeholder="1.1.1.1:7777"
                    onChange={(e) => { pm.value = e.target.value; refresh(); }} />
                  <Button onClick={() => { ib.settings.removePortMap(idx); refresh(); }}>
                    <MinusOutlined />
                  </Button>
                </Space.Compact>
              ))}
            </Form.Item>
          )}
          <Form.Item label="Follow redirect">
            <Switch checked={!!ib.settings.followRedirect} onChange={(v) => { ib.settings.followRedirect = v; refresh(); }} />
          </Form.Item>
        </Form>
      )}

      {ib.protocol === Protocols.TUN && (
        <Form colon={false} labelCol={{ sm: { span: 8 } }} wrapperCol={{ sm: { span: 14 } }} className="mt-12">
          <Form.Item label="Interface name">
            <Input value={ib.settings.name} placeholder="xray0"
              onChange={(e) => { ib.settings.name = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="MTU">
            <InputNumber value={ib.settings.mtu} min={0}
              onChange={(v) => { ib.settings.mtu = Number(v) || 0; refresh(); }} />
          </Form.Item>
          <Form.Item label="Gateway">
            <Button size="small" onClick={() => { ib.settings.gateway.push(''); refresh(); }}>
              <PlusOutlined />
            </Button>
            {(ib.settings.gateway || []).map((_ip: string, j: number) => (
              <Space.Compact key={`tun-gw-${j}`} block className="mt-4">
                <Input
                  placeholder={j === 0 ? '10.0.0.1/16' : 'fc00::1/64'}
                  value={ib.settings.gateway[j]}
                  onChange={(e) => { ib.settings.gateway[j] = e.target.value; refresh(); }} />
                <Button size="small" onClick={() => { ib.settings.gateway.splice(j, 1); refresh(); }}>
                  <MinusOutlined />
                </Button>
              </Space.Compact>
            ))}
          </Form.Item>
          <Form.Item label="DNS">
            <Button size="small" onClick={() => { ib.settings.dns.push(''); refresh(); }}>
              <PlusOutlined />
            </Button>
            {(ib.settings.dns || []).map((_ip: string, j: number) => (
              <Space.Compact key={`tun-dns-${j}`} block className="mt-4">
                <Input
                  placeholder={j === 0 ? '1.1.1.1' : '8.8.8.8'}
                  value={ib.settings.dns[j]}
                  onChange={(e) => { ib.settings.dns[j] = e.target.value; refresh(); }} />
                <Button size="small" onClick={() => { ib.settings.dns.splice(j, 1); refresh(); }}>
                  <MinusOutlined />
                </Button>
              </Space.Compact>
            ))}
          </Form.Item>
          <Form.Item label="User level">
            <InputNumber value={ib.settings.userLevel} min={0}
              onChange={(v) => { ib.settings.userLevel = Number(v) || 0; refresh(); }} />
          </Form.Item>
          <Form.Item label={<Tooltip title="Windows-only. CIDRs added to the system routing table automatically so matching traffic goes through TUN.">Auto system routes</Tooltip>}>
            <Button size="small" onClick={() => { ib.settings.autoSystemRoutingTable.push(''); refresh(); }}>
              <PlusOutlined />
            </Button>
            {(ib.settings.autoSystemRoutingTable || []).map((_ip: string, j: number) => (
              <Space.Compact key={`tun-rt-${j}`} block className="mt-4">
                <Input
                  placeholder={j === 0 ? '0.0.0.0/0' : '::/0'}
                  value={ib.settings.autoSystemRoutingTable[j]}
                  onChange={(e) => { ib.settings.autoSystemRoutingTable[j] = e.target.value; refresh(); }} />
                <Button size="small" onClick={() => { ib.settings.autoSystemRoutingTable.splice(j, 1); refresh(); }}>
                  <MinusOutlined />
                </Button>
              </Space.Compact>
            ))}
          </Form.Item>
          <Form.Item label={<Tooltip title="Physical interface for outbound traffic. Use 'auto' to detect; auto-enabled when Auto system routes is set.">Auto outbounds interface</Tooltip>}>
            <Input value={ib.settings.autoOutboundsInterface} placeholder="auto"
              onChange={(e) => { ib.settings.autoOutboundsInterface = e.target.value; refresh(); }} />
          </Form.Item>
        </Form>
      )}

      {ib.protocol === Protocols.WIREGUARD && (
        <Form colon={false} labelCol={{ sm: { span: 8 } }} wrapperCol={{ sm: { span: 14 } }} className="mt-12">
          <Form.Item label={<>Secret key <SyncOutlined className="random-icon" onClick={regenInboundWg} /></>}>
            <Input value={ib.settings.secretKey}
              onChange={(e) => { ib.settings.secretKey = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="Public key">
            <Input value={ib.settings.pubKey} disabled />
          </Form.Item>
          <Form.Item label="MTU">
            <InputNumber value={ib.settings.mtu}
              onChange={(v) => { ib.settings.mtu = Number(v) || 0; refresh(); }} />
          </Form.Item>
          <Form.Item label="No-kernel TUN">
            <Switch checked={!!ib.settings.noKernelTun}
              onChange={(v) => { ib.settings.noKernelTun = v; refresh(); }} />
          </Form.Item>
          <Form.Item label="Peers">
            <Button size="small" onClick={() => { ib.settings.addPeer(); refresh(); }}>
              <PlusOutlined /> Add peer
            </Button>
          </Form.Item>
          {(ib.settings.peers || []).map((peer: any, idx: number) => (
            <div key={idx} className="wg-peer">
              <Divider style={{ margin: '8px 0' }}>
                Peer {idx + 1}
                {ib.settings.peers.length > 1 && (
                  <DeleteOutlined className="danger-icon" onClick={() => { ib.settings.delPeer(idx); refresh(); }} />
                )}
              </Divider>
              <Form.Item label={<>Secret key <SyncOutlined className="random-icon" onClick={() => regenWgKeypair(peer)} /></>}>
                <Input value={peer.privateKey} onChange={(e) => { peer.privateKey = e.target.value; refresh(); }} />
              </Form.Item>
              <Form.Item label="Public key">
                <Input value={peer.publicKey} onChange={(e) => { peer.publicKey = e.target.value; refresh(); }} />
              </Form.Item>
              <Form.Item label="PSK">
                <Input value={peer.psk} onChange={(e) => { peer.psk = e.target.value; refresh(); }} />
              </Form.Item>
              <Form.Item label="Allowed IPs">
                <Button size="small" onClick={() => { peer.allowedIPs.push(''); refresh(); }}>
                  <PlusOutlined />
                </Button>
                {(peer.allowedIPs || []).map((_ip: string, j: number) => (
                  <Space.Compact key={j} block className="mt-4">
                    <Input
                      value={peer.allowedIPs[j]}
                      onChange={(e) => { peer.allowedIPs[j] = e.target.value; refresh(); }} />
                    {peer.allowedIPs.length > 1 && (
                      <Button size="small" onClick={() => { peer.allowedIPs.splice(j, 1); refresh(); }}>
                        <MinusOutlined />
                      </Button>
                    )}
                  </Space.Compact>
                ))}
              </Form.Item>
              <Form.Item label="Keep-alive">
                <InputNumber value={peer.keepAlive} min={0}
                  onChange={(v) => { peer.keepAlive = Number(v) || 0; refresh(); }} />
              </Form.Item>
            </div>
          ))}
        </Form>
      )}
    </>
  );

  const renderStreamTab = () => {
    const network = ib.stream?.network;
    return (
      <>
        <Form colon={false} labelCol={{ sm: { span: 8 } }} wrapperCol={{ sm: { span: 14 } }}>
          {ib.protocol !== Protocols.HYSTERIA && (
            <Form.Item label="Transmission">
              <Select value={network} style={{ width: '75%' }} onChange={onNetworkChange}>
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
              {canEnableTls && (
                <Form.Item label="Proxy Protocol">
                  <Switch checked={!!ib.stream.tcp.acceptProxyProtocol}
                    onChange={(v) => { ib.stream.tcp.acceptProxyProtocol = v; refresh(); }} />
                </Form.Item>
              )}
              <Form.Item label={`HTTP ${t('camouflage')}`}>
                <Switch checked={ib.stream.tcp.type === 'http'}
                  onChange={(v) => { ib.stream.tcp.type = v ? 'http' : 'none'; refresh(); }} />
              </Form.Item>
              {ib.stream.tcp.type === 'http' && (
                <>
                  <Divider style={{ margin: 0 }}>{t('pages.inbounds.stream.general.request')}</Divider>
                  <Form.Item label={t('pages.inbounds.stream.tcp.version')}>
                    <Input value={ib.stream.tcp.request.version}
                      onChange={(e) => { ib.stream.tcp.request.version = e.target.value; refresh(); }} />
                  </Form.Item>
                  <Form.Item label={t('pages.inbounds.stream.tcp.method')}>
                    <Input value={ib.stream.tcp.request.method}
                      onChange={(e) => { ib.stream.tcp.request.method = e.target.value; refresh(); }} />
                  </Form.Item>
                  <Form.Item label={<>{t('pages.inbounds.stream.tcp.path')} <Button size="small" style={{ marginLeft: 6 }} onClick={() => { ib.stream.tcp.request.addPath('/'); refresh(); }}><PlusOutlined /></Button></>}>
                    {(ib.stream.tcp.request.path || []).map((_p: string, idx: number) => (
                      <Space.Compact key={`tcp-path-${idx}`} block className="mb-4">
                        <Input
                          value={ib.stream.tcp.request.path[idx]}
                          onChange={(e) => { ib.stream.tcp.request.path[idx] = e.target.value; refresh(); }} />
                        {ib.stream.tcp.request.path.length > 1 && (
                          <Button size="small" onClick={() => { ib.stream.tcp.request.removePath(idx); refresh(); }}>
                            <MinusOutlined />
                          </Button>
                        )}
                      </Space.Compact>
                    ))}
                  </Form.Item>
                  <Form.Item label={t('pages.inbounds.stream.tcp.requestHeader')}>
                    <Button size="small" onClick={() => { ib.stream.tcp.request.addHeader('Host', ''); refresh(); }}>
                      <PlusOutlined />
                    </Button>
                  </Form.Item>
                  {(ib.stream.tcp.request.headers || []).length > 0 && (
                    <Form.Item wrapperCol={{ span: 24 }}>
                      {(ib.stream.tcp.request.headers as { name: string; value: string }[]).map((h, idx) => (
                        <Space.Compact key={`tcp-rh-${idx}`} className="mb-8" block>
                          <InputAddon>{String(idx + 1)}</InputAddon>
                          <Input value={h.name}
                            placeholder={t('pages.inbounds.stream.general.name')}
                            onChange={(e) => { h.name = e.target.value; refresh(); }} />
                          <Input value={h.value}
                            placeholder={t('pages.inbounds.stream.general.value')}
                            onChange={(e) => { h.value = e.target.value; refresh(); }} />
                          <Button onClick={() => { ib.stream.tcp.request.removeHeader(idx); refresh(); }}>
                            <MinusOutlined />
                          </Button>
                        </Space.Compact>
                      ))}
                    </Form.Item>
                  )}
                  <Divider style={{ margin: 0 }}>{t('pages.inbounds.stream.general.response')}</Divider>
                  <Form.Item label={t('pages.inbounds.stream.tcp.version')}>
                    <Input value={ib.stream.tcp.response.version}
                      onChange={(e) => { ib.stream.tcp.response.version = e.target.value; refresh(); }} />
                  </Form.Item>
                  <Form.Item label={t('pages.inbounds.stream.tcp.status')}>
                    <Input value={ib.stream.tcp.response.status}
                      onChange={(e) => { ib.stream.tcp.response.status = e.target.value; refresh(); }} />
                  </Form.Item>
                  <Form.Item label={t('pages.inbounds.stream.tcp.statusDescription')}>
                    <Input value={ib.stream.tcp.response.reason}
                      onChange={(e) => { ib.stream.tcp.response.reason = e.target.value; refresh(); }} />
                  </Form.Item>
                  <Form.Item label={t('pages.inbounds.stream.tcp.responseHeader')}>
                    <Button size="small" onClick={() => { ib.stream.tcp.response.addHeader('Content-Type', 'application/octet-stream'); refresh(); }}>
                      <PlusOutlined />
                    </Button>
                  </Form.Item>
                  {(ib.stream.tcp.response.headers || []).length > 0 && (
                    <Form.Item wrapperCol={{ span: 24 }}>
                      {(ib.stream.tcp.response.headers as { name: string; value: string }[]).map((h, idx) => (
                        <Space.Compact key={`tcp-rsh-${idx}`} className="mb-8" block>
                          <InputAddon>{String(idx + 1)}</InputAddon>
                          <Input value={h.name}
                            placeholder={t('pages.inbounds.stream.general.name')}
                            onChange={(e) => { h.name = e.target.value; refresh(); }} />
                          <Input value={h.value}
                            placeholder={t('pages.inbounds.stream.general.value')}
                            onChange={(e) => { h.value = e.target.value; refresh(); }} />
                          <Button onClick={() => { ib.stream.tcp.response.removeHeader(idx); refresh(); }}>
                            <MinusOutlined />
                          </Button>
                        </Space.Compact>
                      ))}
                    </Form.Item>
                  )}
                </>
              )}
            </>
          )}

          {network === 'kcp' && (
            <>
              <Form.Item label="MTU"><InputNumber value={ib.stream.kcp.mtu} min={576} max={1460} onChange={(v) => { ib.stream.kcp.mtu = Number(v) || 0; refresh(); }} /></Form.Item>
              <Form.Item label="TTI (ms)"><InputNumber value={ib.stream.kcp.tti} min={10} max={100} onChange={(v) => { ib.stream.kcp.tti = Number(v) || 0; refresh(); }} /></Form.Item>
              <Form.Item label="Uplink (MB/s)"><InputNumber value={ib.stream.kcp.upCap} min={0} onChange={(v) => { ib.stream.kcp.upCap = Number(v) || 0; refresh(); }} /></Form.Item>
              <Form.Item label="Downlink (MB/s)"><InputNumber value={ib.stream.kcp.downCap} min={0} onChange={(v) => { ib.stream.kcp.downCap = Number(v) || 0; refresh(); }} /></Form.Item>
              <Form.Item label="CWND Multiplier"><InputNumber value={ib.stream.kcp.cwndMultiplier} min={1} onChange={(v) => { ib.stream.kcp.cwndMultiplier = Number(v) || 0; refresh(); }} /></Form.Item>
              <Form.Item label="Max Sending Window"><InputNumber value={ib.stream.kcp.maxSendingWindow} min={0} onChange={(v) => { ib.stream.kcp.maxSendingWindow = Number(v) || 0; refresh(); }} /></Form.Item>
            </>
          )}

          {network === 'ws' && (
            <>
              <Form.Item label="Proxy Protocol"><Switch checked={!!ib.stream.ws.acceptProxyProtocol} onChange={(v) => { ib.stream.ws.acceptProxyProtocol = v; refresh(); }} /></Form.Item>
              <Form.Item label={t('host')}><Input value={ib.stream.ws.host} onChange={(e) => { ib.stream.ws.host = e.target.value; refresh(); }} /></Form.Item>
              <Form.Item label={t('path')}><Input value={ib.stream.ws.path} onChange={(e) => { ib.stream.ws.path = e.target.value; refresh(); }} /></Form.Item>
              <Form.Item label="Heartbeat Period"><InputNumber value={ib.stream.ws.heartbeatPeriod} min={0} onChange={(v) => { ib.stream.ws.heartbeatPeriod = Number(v) || 0; refresh(); }} /></Form.Item>
              <Form.Item label={t('pages.inbounds.stream.tcp.requestHeader')}>
                <Button size="small" onClick={() => { ib.stream.ws.addHeader('', ''); refresh(); }}><PlusOutlined /></Button>
              </Form.Item>
              {(ib.stream.ws.headers || []).length > 0 && (
                <Form.Item wrapperCol={{ span: 24 }}>
                  {(ib.stream.ws.headers as { name: string; value: string }[]).map((h, idx) => (
                    <Space.Compact key={`ws-h-${idx}`} className="mb-8" block>
                      <InputAddon>{String(idx + 1)}</InputAddon>
                      <Input value={h.name}
                        placeholder={t('pages.inbounds.stream.general.name')}
                        onChange={(e) => { h.name = e.target.value; refresh(); }} />
                      <Input value={h.value}
                        placeholder={t('pages.inbounds.stream.general.value')}
                        onChange={(e) => { h.value = e.target.value; refresh(); }} />
                      <Button onClick={() => { ib.stream.ws.removeHeader(idx); refresh(); }}>
                        <MinusOutlined />
                      </Button>
                    </Space.Compact>
                  ))}
                </Form.Item>
              )}
            </>
          )}

          {network === 'grpc' && (
            <>
              <Form.Item label="Service Name"><Input value={ib.stream.grpc.serviceName} onChange={(e) => { ib.stream.grpc.serviceName = e.target.value; refresh(); }} /></Form.Item>
              <Form.Item label="Authority"><Input value={ib.stream.grpc.authority} onChange={(e) => { ib.stream.grpc.authority = e.target.value; refresh(); }} /></Form.Item>
              <Form.Item label="Multi Mode"><Switch checked={!!ib.stream.grpc.multiMode} onChange={(v) => { ib.stream.grpc.multiMode = v; refresh(); }} /></Form.Item>
            </>
          )}

          {network === 'httpupgrade' && (
            <>
              <Form.Item label="Proxy Protocol"><Switch checked={!!ib.stream.httpupgrade.acceptProxyProtocol} onChange={(v) => { ib.stream.httpupgrade.acceptProxyProtocol = v; refresh(); }} /></Form.Item>
              <Form.Item label={t('host')}><Input value={ib.stream.httpupgrade.host} onChange={(e) => { ib.stream.httpupgrade.host = e.target.value; refresh(); }} /></Form.Item>
              <Form.Item label={t('path')}><Input value={ib.stream.httpupgrade.path} onChange={(e) => { ib.stream.httpupgrade.path = e.target.value; refresh(); }} /></Form.Item>
              <Form.Item label={t('pages.inbounds.stream.tcp.requestHeader')}>
                <Button size="small" onClick={() => { ib.stream.httpupgrade.addHeader('', ''); refresh(); }}><PlusOutlined /></Button>
              </Form.Item>
              {(ib.stream.httpupgrade.headers || []).length > 0 && (
                <Form.Item wrapperCol={{ span: 24 }}>
                  {(ib.stream.httpupgrade.headers as { name: string; value: string }[]).map((h, idx) => (
                    <Space.Compact key={`hu-h-${idx}`} className="mb-8" block>
                      <InputAddon>{String(idx + 1)}</InputAddon>
                      <Input value={h.name}
                        placeholder={t('pages.inbounds.stream.general.name')}
                        onChange={(e) => { h.name = e.target.value; refresh(); }} />
                      <Input value={h.value}
                        placeholder={t('pages.inbounds.stream.general.value')}
                        onChange={(e) => { h.value = e.target.value; refresh(); }} />
                      <Button onClick={() => { ib.stream.httpupgrade.removeHeader(idx); refresh(); }}>
                        <MinusOutlined />
                      </Button>
                    </Space.Compact>
                  ))}
                </Form.Item>
              )}
            </>
          )}

          {network === 'xhttp' && (
            <>
              <Form.Item label={t('host')}><Input value={ib.stream.xhttp.host} onChange={(e) => { ib.stream.xhttp.host = e.target.value; refresh(); }} /></Form.Item>
              <Form.Item label={t('path')}><Input value={ib.stream.xhttp.path} onChange={(e) => { ib.stream.xhttp.path = e.target.value; refresh(); }} /></Form.Item>
              <Form.Item label={t('pages.inbounds.stream.tcp.requestHeader')}>
                <Button size="small" onClick={() => { ib.stream.xhttp.addHeader('', ''); refresh(); }}><PlusOutlined /></Button>
              </Form.Item>
              {(ib.stream.xhttp.headers || []).length > 0 && (
                <Form.Item wrapperCol={{ span: 24 }}>
                  {(ib.stream.xhttp.headers as { name: string; value: string }[]).map((h, idx) => (
                    <Space.Compact key={`xh-h-${idx}`} className="mb-8" block>
                      <InputAddon>{String(idx + 1)}</InputAddon>
                      <Input value={h.name}
                        placeholder={t('pages.inbounds.stream.general.name')}
                        onChange={(e) => { h.name = e.target.value; refresh(); }} />
                      <Input value={h.value}
                        placeholder={t('pages.inbounds.stream.general.value')}
                        onChange={(e) => { h.value = e.target.value; refresh(); }} />
                      <Button onClick={() => { ib.stream.xhttp.removeHeader(idx); refresh(); }}>
                        <MinusOutlined />
                      </Button>
                    </Space.Compact>
                  ))}
                </Form.Item>
              )}
              <Form.Item label="Mode">
                <Select value={ib.stream.xhttp.mode} style={{ width: '50%' }} onChange={(v) => { ib.stream.xhttp.mode = v; refresh(); }}>
                  {MODE_OPTIONS.map((m) => <Select.Option key={m} value={m}>{m}</Select.Option>)}
                </Select>
              </Form.Item>
              {ib.stream.xhttp.mode === 'packet-up' && (
                <>
                  <Form.Item label="Max Buffered Upload"><InputNumber value={ib.stream.xhttp.scMaxBufferedPosts} onChange={(v) => { ib.stream.xhttp.scMaxBufferedPosts = Number(v) || 0; refresh(); }} /></Form.Item>
                  <Form.Item label="Max Upload Size (Byte)"><Input value={ib.stream.xhttp.scMaxEachPostBytes} onChange={(e) => { ib.stream.xhttp.scMaxEachPostBytes = e.target.value; refresh(); }} /></Form.Item>
                </>
              )}
              {ib.stream.xhttp.mode === 'stream-up' && (
                <Form.Item label="Stream-Up Server"><Input value={ib.stream.xhttp.scStreamUpServerSecs} onChange={(e) => { ib.stream.xhttp.scStreamUpServerSecs = e.target.value; refresh(); }} /></Form.Item>
              )}
              <Form.Item label="Server Max Header Bytes"><InputNumber value={ib.stream.xhttp.serverMaxHeaderBytes} min={0} placeholder="0 (default)" onChange={(v) => { ib.stream.xhttp.serverMaxHeaderBytes = Number(v) || 0; refresh(); }} /></Form.Item>
              <Form.Item label="Padding Bytes"><Input value={ib.stream.xhttp.xPaddingBytes} onChange={(e) => { ib.stream.xhttp.xPaddingBytes = e.target.value; refresh(); }} /></Form.Item>
              <Form.Item label="Padding Obfs Mode"><Switch checked={!!ib.stream.xhttp.xPaddingObfsMode} onChange={(v) => { ib.stream.xhttp.xPaddingObfsMode = v; refresh(); }} /></Form.Item>
              {ib.stream.xhttp.xPaddingObfsMode && (
                <>
                  <Form.Item label="Padding Key"><Input value={ib.stream.xhttp.xPaddingKey} placeholder="x_padding" onChange={(e) => { ib.stream.xhttp.xPaddingKey = e.target.value; refresh(); }} /></Form.Item>
                  <Form.Item label="Padding Header"><Input value={ib.stream.xhttp.xPaddingHeader} placeholder="X-Padding" onChange={(e) => { ib.stream.xhttp.xPaddingHeader = e.target.value; refresh(); }} /></Form.Item>
                  <Form.Item label="Padding Placement">
                    <Select value={ib.stream.xhttp.xPaddingPlacement} onChange={(v) => { ib.stream.xhttp.xPaddingPlacement = v; refresh(); }}>
                      <Select.Option value="">Default (queryInHeader)</Select.Option>
                      <Select.Option value="queryInHeader">queryInHeader</Select.Option>
                      <Select.Option value="header">header</Select.Option>
                      <Select.Option value="cookie">cookie</Select.Option>
                      <Select.Option value="query">query</Select.Option>
                    </Select>
                  </Form.Item>
                  <Form.Item label="Padding Method">
                    <Select value={ib.stream.xhttp.xPaddingMethod} onChange={(v) => { ib.stream.xhttp.xPaddingMethod = v; refresh(); }}>
                      <Select.Option value="">Default (repeat-x)</Select.Option>
                      <Select.Option value="repeat-x">repeat-x</Select.Option>
                      <Select.Option value="tokenish">tokenish</Select.Option>
                    </Select>
                  </Form.Item>
                </>
              )}
              <Form.Item label="Session Placement">
                <Select value={ib.stream.xhttp.sessionPlacement} onChange={(v) => { ib.stream.xhttp.sessionPlacement = v; refresh(); }}>
                  <Select.Option value="">Default (path)</Select.Option>
                  <Select.Option value="path">path</Select.Option>
                  <Select.Option value="header">header</Select.Option>
                  <Select.Option value="cookie">cookie</Select.Option>
                  <Select.Option value="query">query</Select.Option>
                </Select>
              </Form.Item>
              {ib.stream.xhttp.sessionPlacement && ib.stream.xhttp.sessionPlacement !== 'path' && (
                <Form.Item label="Session Key"><Input value={ib.stream.xhttp.sessionKey} placeholder="x_session" onChange={(e) => { ib.stream.xhttp.sessionKey = e.target.value; refresh(); }} /></Form.Item>
              )}
              <Form.Item label="Sequence Placement">
                <Select value={ib.stream.xhttp.seqPlacement} onChange={(v) => { ib.stream.xhttp.seqPlacement = v; refresh(); }}>
                  <Select.Option value="">Default (path)</Select.Option>
                  <Select.Option value="path">path</Select.Option>
                  <Select.Option value="header">header</Select.Option>
                  <Select.Option value="cookie">cookie</Select.Option>
                  <Select.Option value="query">query</Select.Option>
                </Select>
              </Form.Item>
              {ib.stream.xhttp.seqPlacement && ib.stream.xhttp.seqPlacement !== 'path' && (
                <Form.Item label="Sequence Key"><Input value={ib.stream.xhttp.seqKey} placeholder="x_seq" onChange={(e) => { ib.stream.xhttp.seqKey = e.target.value; refresh(); }} /></Form.Item>
              )}
              {ib.stream.xhttp.mode === 'packet-up' && (
                <Form.Item label="Uplink Data Placement">
                  <Select value={ib.stream.xhttp.uplinkDataPlacement} onChange={(v) => { ib.stream.xhttp.uplinkDataPlacement = v; refresh(); }}>
                    <Select.Option value="">Default (body)</Select.Option>
                    <Select.Option value="body">body</Select.Option>
                    <Select.Option value="header">header</Select.Option>
                    <Select.Option value="cookie">cookie</Select.Option>
                    <Select.Option value="query">query</Select.Option>
                  </Select>
                </Form.Item>
              )}
              {ib.stream.xhttp.mode === 'packet-up' && ib.stream.xhttp.uplinkDataPlacement && ib.stream.xhttp.uplinkDataPlacement !== 'body' && (
                <Form.Item label="Uplink Data Key"><Input value={ib.stream.xhttp.uplinkDataKey} placeholder="x_data" onChange={(e) => { ib.stream.xhttp.uplinkDataKey = e.target.value; refresh(); }} /></Form.Item>
              )}
              <Form.Item label="No SSE Header"><Switch checked={!!ib.stream.xhttp.noSSEHeader} onChange={(v) => { ib.stream.xhttp.noSSEHeader = v; refresh(); }} /></Form.Item>
            </>
          )}

          <Form.Item label="External Proxy">
            <Switch checked={externalProxyOn} onChange={setExternalProxy} />
            {externalProxyOn && (
              <Button size="small" type="primary" style={{ marginLeft: 10 }}
                onClick={() => { ib.stream.externalProxy.push({ forceTls: 'same', dest: '', port: 443, remark: '' }); refresh(); }}>
                <PlusOutlined />
              </Button>
            )}
          </Form.Item>
          {externalProxyOn && (
            <Form.Item wrapperCol={{ span: 24 }}>
              {(ib.stream.externalProxy as { forceTls: string; dest: string; port: number; remark: string }[]).map((row, idx) => (
                <Space.Compact key={`ep-${idx}`} style={{ margin: '8px 0' }} block>
                  <Tooltip title="Force TLS">
                    <Select value={row.forceTls} style={{ width: '20%' }} onChange={(v) => { row.forceTls = v; refresh(); }}>
                      <Select.Option value="same">{t('pages.inbounds.same')}</Select.Option>
                      <Select.Option value="none">{t('none')}</Select.Option>
                      <Select.Option value="tls">TLS</Select.Option>
                    </Select>
                  </Tooltip>
                  <Input style={{ width: '30%' }} value={row.dest} placeholder={t('host')}
                    onChange={(e) => { row.dest = e.target.value; refresh(); }} />
                  <Tooltip title={t('pages.inbounds.port')}>
                    <InputNumber value={row.port} style={{ width: '15%' }} min={1} max={65535}
                      onChange={(v) => { row.port = Number(v) || 0; refresh(); }} />
                  </Tooltip>
                  <Input style={{ width: '25%' }} value={row.remark} placeholder={t('pages.inbounds.remark')}
                    onChange={(e) => { row.remark = e.target.value; refresh(); }} />
                  <InputAddon onClick={() => { ib.stream.externalProxy.splice(idx, 1); refresh(); }}>
                    <MinusOutlined />
                  </InputAddon>
                </Space.Compact>
              ))}
            </Form.Item>
          )}

          <Form.Item label="Sockopt"><Switch checked={!!ib.stream.sockoptSwitch} onChange={(v) => { ib.stream.sockoptSwitch = v; refresh(); }} /></Form.Item>
          {ib.stream.sockoptSwitch && ib.stream.sockopt && (
            <>
              <Form.Item label="Route Mark"><InputNumber value={ib.stream.sockopt.mark} min={0} onChange={(v) => { ib.stream.sockopt.mark = Number(v) || 0; refresh(); }} /></Form.Item>
              <Form.Item label="TCP Keep Alive Interval"><InputNumber value={ib.stream.sockopt.tcpKeepAliveInterval} min={0} onChange={(v) => { ib.stream.sockopt.tcpKeepAliveInterval = Number(v) || 0; refresh(); }} /></Form.Item>
              <Form.Item label="TCP Keep Alive Idle"><InputNumber value={ib.stream.sockopt.tcpKeepAliveIdle} min={0} onChange={(v) => { ib.stream.sockopt.tcpKeepAliveIdle = Number(v) || 0; refresh(); }} /></Form.Item>
              <Form.Item label="TCP Max Seg"><InputNumber value={ib.stream.sockopt.tcpMaxSeg} min={0} onChange={(v) => { ib.stream.sockopt.tcpMaxSeg = Number(v) || 0; refresh(); }} /></Form.Item>
              <Form.Item label="TCP User Timeout"><InputNumber value={ib.stream.sockopt.tcpUserTimeout} min={0} onChange={(v) => { ib.stream.sockopt.tcpUserTimeout = Number(v) || 0; refresh(); }} /></Form.Item>
              <Form.Item label="TCP Window Clamp"><InputNumber value={ib.stream.sockopt.tcpWindowClamp} min={0} onChange={(v) => { ib.stream.sockopt.tcpWindowClamp = Number(v) || 0; refresh(); }} /></Form.Item>
              <Form.Item label="Proxy Protocol"><Switch checked={!!ib.stream.sockopt.acceptProxyProtocol} onChange={(v) => { ib.stream.sockopt.acceptProxyProtocol = v; refresh(); }} /></Form.Item>
              <Form.Item label="TCP Fast Open"><Switch checked={!!ib.stream.sockopt.tcpFastOpen} onChange={(v) => { ib.stream.sockopt.tcpFastOpen = v; refresh(); }} /></Form.Item>
              <Form.Item label="Multipath TCP"><Switch checked={!!ib.stream.sockopt.tcpMptcp} onChange={(v) => { ib.stream.sockopt.tcpMptcp = v; refresh(); }} /></Form.Item>
              <Form.Item label="Penetrate"><Switch checked={!!ib.stream.sockopt.penetrate} onChange={(v) => { ib.stream.sockopt.penetrate = v; refresh(); }} /></Form.Item>
              <Form.Item label="V6 Only"><Switch checked={!!ib.stream.sockopt.V6Only} onChange={(v) => { ib.stream.sockopt.V6Only = v; refresh(); }} /></Form.Item>
              <Form.Item label="Domain Strategy">
                <Select value={ib.stream.sockopt.domainStrategy} style={{ width: '50%' }} onChange={(v) => { ib.stream.sockopt.domainStrategy = v; refresh(); }}>
                  {DOMAIN_STRATEGIES.map((d) => <Select.Option key={d} value={d}>{d}</Select.Option>)}
                </Select>
              </Form.Item>
              <Form.Item label="TCP Congestion">
                <Select value={ib.stream.sockopt.tcpcongestion} style={{ width: '50%' }} onChange={(v) => { ib.stream.sockopt.tcpcongestion = v; refresh(); }}>
                  {TCP_CONGESTIONS.map((c) => <Select.Option key={c} value={c}>{c}</Select.Option>)}
                </Select>
              </Form.Item>
              <Form.Item label="TProxy">
                <Select value={ib.stream.sockopt.tproxy} style={{ width: '50%' }} onChange={(v) => { ib.stream.sockopt.tproxy = v; refresh(); }}>
                  <Select.Option value="off">Off</Select.Option>
                  <Select.Option value="redirect">Redirect</Select.Option>
                  <Select.Option value="tproxy">TProxy</Select.Option>
                </Select>
              </Form.Item>
              <Form.Item label="Dialer Proxy"><Input value={ib.stream.sockopt.dialerProxy} onChange={(e) => { ib.stream.sockopt.dialerProxy = e.target.value; refresh(); }} /></Form.Item>
              <Form.Item label="Interface Name"><Input value={ib.stream.sockopt.interfaceName} onChange={(e) => { ib.stream.sockopt.interfaceName = e.target.value; refresh(); }} /></Form.Item>
              <Form.Item label="Trusted X-Forwarded-For">
                <Select mode="tags" value={ib.stream.sockopt.trustedXForwardedFor} style={{ width: '100%' }}
                  tokenSeparators={[',']}
                  onChange={(v) => { ib.stream.sockopt.trustedXForwardedFor = v; refresh(); }}>
                  <Select.Option value="CF-Connecting-IP">CF-Connecting-IP</Select.Option>
                  <Select.Option value="X-Real-IP">X-Real-IP</Select.Option>
                  <Select.Option value="True-Client-IP">True-Client-IP</Select.Option>
                  <Select.Option value="X-Client-IP">X-Client-IP</Select.Option>
                </Select>
              </Form.Item>
            </>
          )}

          {ib.protocol === Protocols.HYSTERIA && (
            <>
              <Form.Item label={<Tooltip title="Hysteria protocol version. Currently must be 2.">Version</Tooltip>}>
                <InputNumber value={ib.stream.hysteria.version} min={2} max={2} onChange={(v) => { ib.stream.hysteria.version = Number(v) || 2; refresh(); }} />
              </Form.Item>
              <Form.Item label={<Tooltip title="Idle timeout (seconds) for a single QUIC native UDP connection.">UDP idle timeout</Tooltip>}>
                <InputNumber value={ib.stream.hysteria.udpIdleTimeout} min={0} onChange={(v) => { ib.stream.hysteria.udpIdleTimeout = Number(v) || 0; refresh(); }} />
              </Form.Item>
              <Form.Item label="Masquerade">
                <Switch checked={!!ib.stream.hysteria.masqueradeSwitch} onChange={(v) => { ib.stream.hysteria.masqueradeSwitch = v; refresh(); }} />
              </Form.Item>
              {ib.stream.hysteria.masqueradeSwitch && (
                <>
                  <Form.Item label="Type">
                    <Select value={ib.stream.hysteria.masquerade.type} style={{ width: '50%' }} onChange={(v) => { ib.stream.hysteria.masquerade.type = v; refresh(); }}>
                      <Select.Option value="proxy">Proxy</Select.Option>
                      <Select.Option value="file">File</Select.Option>
                      <Select.Option value="string">String</Select.Option>
                    </Select>
                  </Form.Item>
                  {ib.stream.hysteria.masquerade.type === 'proxy' && (
                    <>
                      <Form.Item label="URL"><Input value={ib.stream.hysteria.masquerade.url} placeholder="https://example.com" onChange={(e) => { ib.stream.hysteria.masquerade.url = e.target.value; refresh(); }} /></Form.Item>
                      <Form.Item label="Rewrite Host"><Switch checked={!!ib.stream.hysteria.masquerade.rewriteHost} onChange={(v) => { ib.stream.hysteria.masquerade.rewriteHost = v; refresh(); }} /></Form.Item>
                      <Form.Item label="Insecure"><Switch checked={!!ib.stream.hysteria.masquerade.insecure} onChange={(v) => { ib.stream.hysteria.masquerade.insecure = v; refresh(); }} /></Form.Item>
                    </>
                  )}
                  {ib.stream.hysteria.masquerade.type === 'file' && (
                    <Form.Item label="Directory"><Input value={ib.stream.hysteria.masquerade.dir} placeholder="/path/to/www" onChange={(e) => { ib.stream.hysteria.masquerade.dir = e.target.value; refresh(); }} /></Form.Item>
                  )}
                  {ib.stream.hysteria.masquerade.type === 'string' && (
                    <>
                      <Form.Item label="Content"><TextArea value={ib.stream.hysteria.masquerade.content} autoSize={{ minRows: 2, maxRows: 6 }} onChange={(e) => { ib.stream.hysteria.masquerade.content = e.target.value; refresh(); }} /></Form.Item>
                      <Form.Item label="Status Code"><InputNumber value={ib.stream.hysteria.masquerade.statusCode} min={100} max={599} placeholder="200" onChange={(v) => { ib.stream.hysteria.masquerade.statusCode = Number(v) || 0; refresh(); }} /></Form.Item>
                      <Form.Item label="Headers">
                        <Button size="small" onClick={() => { ib.stream.hysteria.masquerade.addHeader('', ''); refresh(); }}>
                          <PlusOutlined />
                        </Button>
                      </Form.Item>
                      {(ib.stream.hysteria.masquerade.headers || []).length > 0 && (
                        <Form.Item wrapperCol={{ span: 24 }}>
                          {(ib.stream.hysteria.masquerade.headers as { name: string; value: string }[]).map((h, idx) => (
                            <Space.Compact key={`mh-${idx}`} className="mb-8" block>
                              <InputAddon>{String(idx + 1)}</InputAddon>
                              <Input value={h.name} placeholder="Name"
                                onChange={(e) => { h.name = e.target.value; refresh(); }} />
                              <Input value={h.value} placeholder="Value"
                                onChange={(e) => { h.value = e.target.value; refresh(); }} />
                              <Button onClick={() => { ib.stream.hysteria.masquerade.removeHeader(idx); refresh(); }}>
                                <MinusOutlined />
                              </Button>
                            </Space.Compact>
                          ))}
                        </Form.Item>
                      )}
                    </>
                  )}
                </>
              )}
            </>
          )}
        </Form>

        <FinalMaskForm stream={ib.stream} protocol={ib.protocol} onChange={refresh} />
      </>
    );
  };

  const renderSecurityTab = () => (
    <Form colon={false} labelCol={{ sm: { span: 8 } }} wrapperCol={{ sm: { span: 14 } }}>
      <Form.Item label={t('pages.inbounds.securityTab')}>
        <Radio.Group value={ib.stream.security} buttonStyle="solid" disabled={!canEnableTls}
          onChange={(e) => setSecurity(e.target.value)}>
          <Radio.Button value="none">none</Radio.Button>
          <Radio.Button value="tls">tls</Radio.Button>
          {canEnableReality && <Radio.Button value="reality">reality</Radio.Button>}
        </Radio.Group>
      </Form.Item>

      {ib.stream.security === 'tls' && ib.stream.tls && (
        <>
          <Form.Item label="SNI"><Input value={ib.stream.tls.sni} placeholder="Server Name Indication" onChange={(e) => { ib.stream.tls.sni = e.target.value; refresh(); }} /></Form.Item>
          <Form.Item label="Cipher Suites">
            <Select value={ib.stream.tls.cipherSuites} onChange={(v) => { ib.stream.tls.cipherSuites = v; refresh(); }}>
              <Select.Option value="">Auto</Select.Option>
              {CIPHER_SUITES.map(([label, val]) => <Select.Option key={val} value={val}>{label}</Select.Option>)}
            </Select>
          </Form.Item>
          <Form.Item label="Min/Max Version">
            <Space.Compact block>
              <Select value={ib.stream.tls.minVersion} style={{ width: '50%' }} onChange={(v) => { ib.stream.tls.minVersion = v; refresh(); }}>
                {TLS_VERSIONS.map((v) => <Select.Option key={v} value={v}>{v}</Select.Option>)}
              </Select>
              <Select value={ib.stream.tls.maxVersion} style={{ width: '50%' }} onChange={(v) => { ib.stream.tls.maxVersion = v; refresh(); }}>
                {TLS_VERSIONS.map((v) => <Select.Option key={v} value={v}>{v}</Select.Option>)}
              </Select>
            </Space.Compact>
          </Form.Item>
          <Form.Item label="uTLS">
            <Select value={ib.stream.tls.settings.fingerprint} style={{ width: '100%' }} onChange={(v) => { ib.stream.tls.settings.fingerprint = v; refresh(); }}>
              <Select.Option value="">None</Select.Option>
              {FINGERPRINTS.map((fp) => <Select.Option key={fp} value={fp}>{fp}</Select.Option>)}
            </Select>
          </Form.Item>
          <Form.Item label="ALPN">
            <Select mode="multiple" value={ib.stream.tls.alpn} style={{ width: '100%' }} tokenSeparators={[',']}
              onChange={(v) => { ib.stream.tls.alpn = v; refresh(); }}>
              {ALPNS.map((a) => <Select.Option key={a} value={a}>{a}</Select.Option>)}
            </Select>
          </Form.Item>
          <Form.Item label="Reject Unknown SNI"><Switch checked={!!ib.stream.tls.rejectUnknownSni} onChange={(v) => { ib.stream.tls.rejectUnknownSni = v; refresh(); }} /></Form.Item>
          <Form.Item label="Disable System Root"><Switch checked={!!ib.stream.tls.disableSystemRoot} onChange={(v) => { ib.stream.tls.disableSystemRoot = v; refresh(); }} /></Form.Item>
          <Form.Item label="Session Resumption"><Switch checked={!!ib.stream.tls.enableSessionResumption} onChange={(v) => { ib.stream.tls.enableSessionResumption = v; refresh(); }} /></Form.Item>

          {(ib.stream.tls.certs || []).map((cert: any, idx: number) => (
            <div key={`cert-${idx}`}>
              <Form.Item label={t('certificate')}>
                <Radio.Group value={cert.useFile} buttonStyle="solid" onChange={(e) => { cert.useFile = e.target.value; refresh(); }}>
                  <Radio.Button value={true}>{t('pages.inbounds.certificatePath')}</Radio.Button>
                  <Radio.Button value={false}>{t('pages.inbounds.certificateContent')}</Radio.Button>
                </Radio.Group>
              </Form.Item>
              <Form.Item label=" ">
                <Space>
                  {idx === 0 && (
                    <Button type="primary" size="small" onClick={() => { ib.stream.tls.addCert(); refresh(); }}>
                      <PlusOutlined />
                    </Button>
                  )}
                  {ib.stream.tls.certs.length > 1 && (
                    <Button type="primary" size="small" onClick={() => { ib.stream.tls.removeCert(idx); refresh(); }}>
                      <MinusOutlined />
                    </Button>
                  )}
                </Space>
              </Form.Item>
              {cert.useFile ? (
                <>
                  <Form.Item label={t('pages.inbounds.publicKey')}>
                    <Input value={cert.certFile} onChange={(e) => { cert.certFile = e.target.value; refresh(); }} />
                  </Form.Item>
                  <Form.Item label={t('pages.inbounds.privatekey')}>
                    <Input value={cert.keyFile} onChange={(e) => { cert.keyFile = e.target.value; refresh(); }} />
                  </Form.Item>
                  <Form.Item label=" ">
                    <Button type="primary" disabled={!defaultCert && !defaultKey} onClick={() => setDefaultCertData(idx)}>
                      {t('pages.inbounds.setDefaultCert')}
                    </Button>
                  </Form.Item>
                </>
              ) : (
                <>
                  <Form.Item label={t('pages.inbounds.publicKey')}>
                    <TextArea value={cert.cert} autoSize={{ minRows: 3, maxRows: 8 }}
                      onChange={(e) => { cert.cert = e.target.value; refresh(); }} />
                  </Form.Item>
                  <Form.Item label={t('pages.inbounds.privatekey')}>
                    <TextArea value={cert.key} autoSize={{ minRows: 3, maxRows: 8 }}
                      onChange={(e) => { cert.key = e.target.value; refresh(); }} />
                  </Form.Item>
                </>
              )}
              <Form.Item label="One Time Loading"><Switch checked={!!cert.oneTimeLoading} onChange={(v) => { cert.oneTimeLoading = v; refresh(); }} /></Form.Item>
              <Form.Item label="Usage Option">
                <Select value={cert.usage} style={{ width: '50%' }} onChange={(v) => { cert.usage = v; refresh(); }}>
                  {USAGES.map((u) => <Select.Option key={u} value={u}>{u}</Select.Option>)}
                </Select>
              </Form.Item>
              {cert.usage === 'issue' && (
                <Form.Item label="Build Chain"><Switch checked={!!cert.buildChain} onChange={(v) => { cert.buildChain = v; refresh(); }} /></Form.Item>
              )}
            </div>
          ))}

          <Form.Item label="ECH key"><Input value={ib.stream.tls.echServerKeys} onChange={(e) => { ib.stream.tls.echServerKeys = e.target.value; refresh(); }} /></Form.Item>
          <Form.Item label="ECH config"><Input value={ib.stream.tls.settings.echConfigList} onChange={(e) => { ib.stream.tls.settings.echConfigList = e.target.value; refresh(); }} /></Form.Item>
          <Form.Item label=" ">
            <Space>
              <Button type="primary" loading={saving} onClick={getNewEchCert}>Get New ECH Cert</Button>
              <Button danger onClick={clearEchCert}>Clear</Button>
            </Space>
          </Form.Item>
        </>
      )}

      {ib.stream.security === 'reality' && ib.stream.reality && (
        <>
          <Form.Item label="Show"><Switch checked={!!ib.stream.reality.show} onChange={(v) => { ib.stream.reality.show = v; refresh(); }} /></Form.Item>
          <Form.Item label="Xver"><InputNumber value={ib.stream.reality.xver} min={0} onChange={(v) => { ib.stream.reality.xver = Number(v) || 0; refresh(); }} /></Form.Item>
          <Form.Item label="uTLS">
            <Select value={ib.stream.reality.settings.fingerprint} style={{ width: '100%' }} onChange={(v) => { ib.stream.reality.settings.fingerprint = v; refresh(); }}>
              {FINGERPRINTS.map((fp) => <Select.Option key={fp} value={fp}>{fp}</Select.Option>)}
            </Select>
          </Form.Item>
          <Form.Item label={<>Target <SyncOutlined className="random-icon" onClick={randomizeRealityTarget} /></>}>
            <Input value={ib.stream.reality.target} onChange={(e) => { ib.stream.reality.target = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label={<>SNI <SyncOutlined className="random-icon" onClick={randomizeRealityTarget} /></>}>
            <Input value={ib.stream.reality.serverNames} onChange={(e) => { ib.stream.reality.serverNames = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="Max Time Diff (ms)"><InputNumber value={ib.stream.reality.maxTimediff} min={0} onChange={(v) => { ib.stream.reality.maxTimediff = Number(v) || 0; refresh(); }} /></Form.Item>
          <Form.Item label="Min Client Ver"><Input value={ib.stream.reality.minClientVer} placeholder="25.9.11" onChange={(e) => { ib.stream.reality.minClientVer = e.target.value; refresh(); }} /></Form.Item>
          <Form.Item label="Max Client Ver"><Input value={ib.stream.reality.maxClientVer} placeholder="25.9.11" onChange={(e) => { ib.stream.reality.maxClientVer = e.target.value; refresh(); }} /></Form.Item>
          <Form.Item label={<>Short IDs <SyncOutlined className="random-icon" onClick={randomizeShortIds} /></>}>
            <TextArea value={ib.stream.reality.shortIds} autoSize={{ minRows: 1, maxRows: 4 }} onChange={(e) => { ib.stream.reality.shortIds = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="SpiderX"><Input value={ib.stream.reality.settings.spiderX} onChange={(e) => { ib.stream.reality.settings.spiderX = e.target.value; refresh(); }} /></Form.Item>
          <Form.Item label={t('pages.inbounds.publicKey')}>
            <TextArea value={ib.stream.reality.settings.publicKey} autoSize={{ minRows: 1, maxRows: 4 }}
              onChange={(e) => { ib.stream.reality.settings.publicKey = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label={t('pages.inbounds.privatekey')}>
            <TextArea value={ib.stream.reality.privateKey} autoSize={{ minRows: 1, maxRows: 4 }}
              onChange={(e) => { ib.stream.reality.privateKey = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label=" ">
            <Space>
              <Button type="primary" loading={saving} onClick={genRealityKeypair}>Get New Cert</Button>
              <Button danger onClick={clearRealityKeypair}>Clear</Button>
            </Space>
          </Form.Item>
          <Form.Item label="mldsa65 Seed">
            <TextArea value={ib.stream.reality.mldsa65Seed} autoSize={{ minRows: 2, maxRows: 6 }} onChange={(e) => { ib.stream.reality.mldsa65Seed = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label="mldsa65 Verify">
            <TextArea value={ib.stream.reality.settings.mldsa65Verify} autoSize={{ minRows: 2, maxRows: 6 }} onChange={(e) => { ib.stream.reality.settings.mldsa65Verify = e.target.value; refresh(); }} />
          </Form.Item>
          <Form.Item label=" ">
            <Space>
              <Button type="primary" loading={saving} onClick={genMldsa65}>Get New Seed</Button>
              <Button danger onClick={clearMldsa65}>Clear</Button>
            </Space>
          </Form.Item>
        </>
      )}
    </Form>
  );

  const renderSniffingTab = () => (
    <Form colon={false} labelCol={{ sm: { span: 8 } }} wrapperCol={{ sm: { span: 14 } }}>
      <Form.Item label={t('enable')}>
        <Switch checked={!!ib.sniffing.enabled} onChange={(v) => { ib.sniffing.enabled = v; refresh(); }} />
      </Form.Item>
      {ib.sniffing.enabled && (
        <>
          <Form.Item wrapperCol={{ span: 24 }}>
            <Checkbox.Group value={ib.sniffing.destOverride} onChange={(v) => { ib.sniffing.destOverride = v; refresh(); }}>
              {Object.entries(SNIFFING_OPTION).map(([key, value]) => (
                <Checkbox key={key} value={value}>{key}</Checkbox>
              ))}
            </Checkbox.Group>
          </Form.Item>
          <Form.Item label={t('pages.inbounds.sniffingMetadataOnly')}>
            <Switch checked={!!ib.sniffing.metadataOnly} onChange={(v) => { ib.sniffing.metadataOnly = v; refresh(); }} />
          </Form.Item>
          <Form.Item label={t('pages.inbounds.sniffingRouteOnly')}>
            <Switch checked={!!ib.sniffing.routeOnly} onChange={(v) => { ib.sniffing.routeOnly = v; refresh(); }} />
          </Form.Item>
          <Form.Item label={t('pages.inbounds.sniffingIpsExcluded')}>
            <Select mode="tags" value={ib.sniffing.ipsExcluded} tokenSeparators={[',']}
              placeholder="IP/CIDR/geoip:*/ext:*" style={{ width: '100%' }}
              onChange={(v) => { ib.sniffing.ipsExcluded = v; refresh(); }} />
          </Form.Item>
          <Form.Item label={t('pages.inbounds.sniffingDomainsExcluded')}>
            <Select mode="tags" value={ib.sniffing.domainsExcluded} tokenSeparators={[',']}
              placeholder="domain:*/ext:*" style={{ width: '100%' }}
              onChange={(v) => { ib.sniffing.domainsExcluded = v; refresh(); }} />
          </Form.Item>
        </>
      )}
    </Form>
  );

  const renderAdvancedTab = () => {
    const advancedTabItems = [
      {
        key: 'all',
        label: t('pages.inbounds.advanced.all'),
        children: (
          <>
            <div className="advanced-editor-meta">{t('pages.inbounds.advanced.allHelp')}</div>
            <JsonEditor value={advancedAllValue} onChange={setAdvancedAllValue} minHeight="340px" maxHeight="560px" />
          </>
        ),
      },
      {
        key: 'settings',
        label: t('pages.inbounds.advanced.settings'),
        children: (
          <>
            <div className="advanced-editor-meta">
              {t('pages.inbounds.advanced.settingsHelp')} <code>{'{ settings: { ... } }'}</code>.
            </div>
            <JsonEditor value={wrappedConfigValue('settings', 'settings')}
              onChange={(v) => setWrappedConfigValue('settings', 'settings', 'Settings', v)}
              minHeight="320px" maxHeight="540px" />
          </>
        ),
      },
      {
        key: 'sniffingSection',
        label: t('pages.inbounds.advanced.sniffing'),
        children: (
          <>
            <div className="advanced-editor-meta">
              {t('pages.inbounds.advanced.sniffingHelp')} <code>{'{ sniffing: { ... } }'}</code>.
            </div>
            <JsonEditor value={wrappedConfigValue('sniffing', 'sniffing')}
              onChange={(v) => setWrappedConfigValue('sniffing', 'sniffing', 'Sniffing', v)}
              minHeight="240px" maxHeight="420px" />
          </>
        ),
      },
    ];
    if (canEnableStream) {
      advancedTabItems.push({
        key: 'streamSection',
        label: t('pages.inbounds.advanced.stream'),
        children: (
          <>
            <div className="advanced-editor-meta">
              {t('pages.inbounds.advanced.streamHelp')} <code>{'{ streamSettings: { ... } }'}</code>.
            </div>
            <JsonEditor value={wrappedConfigValue('streamSettings', 'stream')}
              onChange={(v) => setWrappedConfigValue('streamSettings', 'stream', 'Stream', v)}
              minHeight="320px" maxHeight="540px" />
          </>
        ),
      });
    }

    return (
      <div className="advanced-shell">
        <div className="advanced-panel">
          <div className="advanced-panel__header">
            <div>
              <div className="advanced-panel__title">{t('pages.inbounds.advanced.title')}</div>
              <div className="advanced-panel__subtitle">{t('pages.inbounds.advanced.subtitle')}</div>
            </div>
          </div>
          <Tabs activeKey={advancedSectionKey} onChange={setAdvancedSectionKey} items={advancedTabItems} className="advanced-inner-tabs" />
        </div>
      </div>
    );
  };

  const tabItems = [
    { key: 'basic', label: t('pages.xray.basicTemplate'), children: renderBasicsTab() },
  ];
  if (hasProtocolTabContent) {
    tabItems.push({ key: 'protocol', label: t('pages.inbounds.protocol'), children: renderProtocolTab() });
  }
  if (canEnableStream) {
    tabItems.push({ key: 'stream', label: t('pages.inbounds.streamTab'), children: renderStreamTab() });
    tabItems.push({ key: 'security', label: t('pages.inbounds.securityTab'), children: renderSecurityTab() });
  }
  tabItems.push({ key: 'sniffing', label: t('pages.inbounds.sniffingTab'), children: renderSniffingTab() });
  tabItems.push({ key: 'advanced', label: t('pages.xray.advancedTemplate'), children: renderAdvancedTab() });

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
        <Tabs activeKey={activeTabKey} onChange={handleTabChange} items={tabItems} />
      </Modal>
    </>
  );
}
