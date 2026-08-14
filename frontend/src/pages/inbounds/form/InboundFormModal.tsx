import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { QuestionCircleOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import {
  Alert,
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
import { Controller, FormProvider, useForm, useWatch } from 'react-hook-form';

import { HttpUtil, NumberFormatter, RandomUtil, SizeFormatter, Wireguard } from '@/utils';
import type { QuicCaptureResult, RealityScanResult } from '@/generated/types';
import {
  rawInboundToFormValues,
  formValuesToWirePayload,
} from '@/lib/xray/inbound-form-adapter';
import { createDefaultInboundSettings } from '@/lib/xray/inbound-defaults';
import { composeInboundTag, isAutoInboundTag, type InboundTagInput } from '@/lib/xray/inbound-tag';
import {
  canEnableReality,
  canEnableSniffing,
  canEnableStream,
  canEnableTls,
  isSS2022,
} from '@/lib/xray/protocol-capabilities';
import {
  InboundDbFieldsSchema,
  InboundFormBaseSchema,
  InboundFormSchema,
  type InboundFormValues,
} from '@/schemas/forms/inbound-form';
import { FormField, rhfZodValidate } from '@/components/form/rhf';
import { Protocols } from '@/schemas/primitives';
import { AWG_VERSION_3, effectiveAwgVersion } from '@/schemas/protocols/inbound/amneziawg';
import { SockoptStreamSettingsSchema } from '@/schemas/protocols/stream/sockopt';
import { HysteriaStreamSettingsSchema } from '@/schemas/protocols/stream/hysteria';
import { createHysteriaTlsSettingsWithDefaultCert } from '@/lib/xray/inbound-tls-defaults';
import { CPS_SLOTS, type CpsSlot, genI1, isQuicI1Supported, type I1ProfileChoice } from '@/lib/xray/i1Generators';
import { VLESS_AUTH_LABEL_KEYS, vlessEncryptionAuthKind } from '@/lib/xray/vless-encryption';
import { SniffingSchema } from '@/schemas/primitives/sniffing';
import { TcpStreamSettingsSchema } from '@/schemas/protocols/stream/tcp';
import { KcpStreamSettingsSchema } from '@/schemas/protocols/stream/kcp';
import { WsStreamSettingsSchema } from '@/schemas/protocols/stream/ws';
import { GrpcStreamSettingsSchema } from '@/schemas/protocols/stream/grpc';
import { HttpUpgradeStreamSettingsSchema } from '@/schemas/protocols/stream/httpupgrade';
import { XHttpStreamSettingsSchema } from '@/schemas/protocols/stream/xhttp';
import { DateTimePicker } from '@/components/form';
import { FinalMaskField } from '@/lib/xray/forms/fields';
import './InboundFormModal.css';

import { AdvancedAllEditor, AdvancedSliceEditor } from './advanced-editors';
import { formatInboundIssue, formatInboundValidation } from './formatValidationError';
import {
  AmneziawgFields,
  HttpFields,
  HysteriaFields,
  MixedFields,
  MtprotoFields,
  ShadowsocksFields,
  TunFields,
  TunnelFields,
  VlessFields,
  WireguardFields,
} from './protocols';
import {
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


/* Render a field label with a hover tooltip icon instead of an `extra` help line below. */
const labelWithHint = (label: string, hint: string) => (
  <span>
    {label}
    <Tooltip title={hint}>
      <QuestionCircleOutlined style={{ marginInlineStart: 4, color: 'rgba(128,128,128,0.65)' }} />
    </Tooltip>
  </span>
);

const PROTOCOL_OPTIONS = Object.values(Protocols).map((p) => ({ value: p, label: p }));
// The 5 AWG3 device-timer field names, gated by amgVersion the same way as
// headerProtectionKey/contentPaddingAddition (see onAwgVersionChange).
const AWG_TIMER_FIELDS = [
  'rekeyAfterTime', 'rekeyTimeout', 'rejectAfterTime', 'keepaliveTimeout', 'maxHandshakeAttempts',
] as const;
const TRAFFIC_RESETS = ['never', 'hourly', 'daily', 'weekly', 'monthly'] as const;
const SHARE_ADDR_STRATEGIES = ['node', 'listen', 'custom'] as const;
const SHARE_ADDR_HOSTNAME_RE = /^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?)*$/;
const NODE_ELIGIBLE_PROTOCOLS = new Set<string>([
  Protocols.VLESS,
  Protocols.VMESS,
  Protocols.TROJAN,
  Protocols.SHADOWSOCKS,
  Protocols.HYSTERIA,
  Protocols.WIREGUARD,
]);

function isValidShareAddrInput(value: string): boolean {
  const v = value.trim();
  if (v.length === 0) return true;
  if (v.includes('://') || v.startsWith('//') || /[/?#@]/.test(v)) return false;
  if (v.startsWith('[')) {
    if (!v.endsWith(']')) return false;
    try {
      new URL(`http://${v}`);
      return true;
    } catch {
      return false;
    }
  }
  if (v.includes(':')) {
    try {
      new URL(`http://[${v}]`);
      return true;
    } catch {
      return false;
    }
  }
  return SHARE_ADDR_HOSTNAME_RE.test(v);
}

interface InboundFormModalProps {
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
  mode: 'add' | 'edit';
  dbInbound: DBInbound | null;
  dbInbounds: DBInbound[];
  availableNodes?: NodeRecord[];
  availableNodesFetched?: boolean;
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

/*
 * Switching `network` swaps which per-network key (tcpSettings, wsSettings,
 * grpcSettings, ...) appears on the wire. Seed each network's blob with its
 * Zod schema defaults so every field inside the network sub-form has a
 * defined starting value (KCP needs MTU=1350 etc., XHTTP needs the ""
 * sentinels so the "Default" option shows instead of blank).
 */
function newStreamSlice(n: string): Record<string, unknown> {
  switch (n) {
    case 'tcp': return TcpStreamSettingsSchema.parse({ header: { type: 'none' } });
    case 'kcp': return KcpStreamSettingsSchema.parse({});
    case 'ws': return WsStreamSettingsSchema.parse({});
    case 'grpc': return GrpcStreamSettingsSchema.parse({});
    case 'httpupgrade': return HttpUpgradeStreamSettingsSchema.parse({});
    case 'xhttp': return XHttpStreamSettingsSchema.parse({});
    default: return {};
  }
}

export default function InboundFormModal({
  open,
  onClose,
  onSaved,
  mode,
  dbInbound,
  dbInbounds,
  availableNodes,
  availableNodesFetched = true,
}: InboundFormModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const methods = useForm<InboundFormValues>({ defaultValues: buildAddModeValues() });
  const setV = methods.setValue as unknown as (name: string, value: unknown) => void;
  const getV = methods.getValues as unknown as (name?: string) => unknown;
  const control = methods.control;
  const [saving, setSaving] = useState(false);
  const [scanning, setScanning] = useState(false);
  const [scanResult, setScanResult] = useState<RealityScanResult | null>(null);
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
  const protocol = (useWatch({ control, name: 'protocol' }) ?? '') as string;
  const isNodeEligible = NODE_ELIGIBLE_PROTOCOLS.has(protocol);
  /*
   * The `node` share-address strategy only means something when the inbound can
   * actually live on a node — otherwise the node address it would resolve to is
   * always empty. Offer it only then; `listen`/`custom` work for local inbounds.
   */
  const nodeShareOptionAvailable = selectableNodes.length > 0 && isNodeEligible;
  const vlessEncryption = useWatch({ control, name: 'settings.encryption' }) ?? '';
  const ssMethod = useWatch({ control, name: 'settings.method' });
  const isSSWith2022 = isSS2022({
    protocol,
    settings: typeof ssMethod === 'string' ? { method: ssMethod } : {},
  });
  const mixedUdpOn = (useWatch({ control, name: 'settings.udp' }) ?? false) as boolean;
  const network = (useWatch({ control, name: 'streamSettings.network' }) ?? '') as string;
  const security = (useWatch({ control, name: 'streamSettings.security' }) ?? 'none') as string;
  const streamEnabled = canEnableStream({ protocol });
  const sniffingSupported = canEnableSniffing({ protocol });
  /*
   * Wireguard (always a UDP listener) and Tunnel (dokodemo-door) expose no
   * user-selectable transport — their stream tab is just sockopt, which is all
   * Tunnel's TProxy/redirect mode needs (sockopt.tproxy). Hysteria carries its
   * own dedicated transport form. For all of these the RAW/mKCP/WS/... network
   * picker and the per-network sub-forms are hidden.
   */
  const hasSelectableTransport =
    protocol !== Protocols.HYSTERIA
    && protocol !== Protocols.WIREGUARD
    && protocol !== Protocols.TUNNEL;

  const wPort = useWatch({ control, name: 'port' });
  const wListen = (useWatch({ control, name: 'listen' }) ?? '') as string;
  const isUdsListen = wListen.startsWith('/') || wListen.startsWith('@');
  const wNodeId = useWatch({ control, name: 'nodeId' }) ?? null;
  const shareAddrStrategy = useWatch({ control, name: 'shareAddrStrategy' }) ?? 'node';
  const wTag = (useWatch({ control, name: 'tag' }) ?? '') as string;
  const wSsNetwork = useWatch({ control, name: 'settings.network' });
  const wTunnelNetwork = useWatch({ control, name: 'settings.allowedNetwork' });
  const wTotal = (useWatch({ control, name: 'total' }) as number | undefined) ?? 0;
  const wExpiry = (useWatch({ control, name: 'expiryTime' }) as number | undefined) ?? 0;
  const trafficReset = useWatch({ control, name: 'trafficReset' }) ?? 'never';
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
    scanRealityTarget,
    scanRealityCandidates,
    applyRealityScanResult,
    randomizeShortIds,
    randomizeSpiderX,
    getNewEchCert,
    clearEchCert,
    pinFromCert,
    pinFromRemote,
    setCertFromPanel,
    clearCertFiles,
    onSecurityChange,
  } = useSecurityActions({ methods, setSaving, messageApi, nodeId: typeof wNodeId === 'number' ? wNodeId : null, setScanResult, setScanning });


  const toggleSockopt = (on: boolean) => {
    if (on) {
      setV('streamSettings.sockopt', SockoptStreamSettingsSchema.parse({}));
    } else {
      setV('streamSettings.sockopt', undefined);
    }
  };
  const wgSecretKey = useWatch({ control, name: 'settings.secretKey' });
  const wgPubKey = typeof wgSecretKey === 'string' && wgSecretKey.length > 0
    ? Wireguard.generateKeypair(wgSecretKey).publicKey
    : '';

  const regenInboundWg = () => {
    const kp = Wireguard.generateKeypair();
    setV('settings.secretKey', kp.privateKey);
  };

  // AmneziaWG uses the same Curve25519 keys as WireGuard, just nested under
  // settings.server instead of flat on settings — see amneziawg.ts. Unlike
  // WireGuard's Xray-native inbound (which re-derives its public key at
  // runtime and never stores one), AmneziaWG's server.publicKey is a real,
  // persisted field the Go backend reads directly, so it must be kept in
  // sync even when the user free-types a new private key instead of using
  // the regenerate button.
  const awgPrivateKey = useWatch({ control, name: 'settings.server.privateKey' });
  const awgPubKey = typeof awgPrivateKey === 'string' && awgPrivateKey.length > 0
    ? Wireguard.generateKeypair(awgPrivateKey).publicKey
    : '';

  useEffect(() => {
    if (protocol === Protocols.AMNEZIAWG) {
      setV('settings.server.publicKey', awgPubKey);
    }
    /* eslint-disable-next-line react-hooks/exhaustive-deps */
  }, [awgPubKey, protocol]);

  const regenInboundAwg = () => {
    const kp = Wireguard.generateKeypair();
    setV('settings.server.privateKey', kp.privateKey);
    setV('settings.server.publicKey', kp.publicKey);
  };

  // AmneziaWG 3.0's HeaderProtectionKey is a symmetric ChaCha20 key, not a
  // Curve25519 keypair member -- generatePresharedKey (32 unclamped random
  // bytes) is the correct primitive, matching how a WireGuard PSK is
  // generated, not generateKeypair/generatePrivateKey's X25519 clamping.
  // Deliberately its own action, separate from regenInboundAwgObfuscation:
  // turning on header protection is a compatibility-breaking decision an
  // admin should make explicitly, not something a bulk obfuscation reroll
  // silently does.
  const regenInboundAwgHeaderProtectionKey = () => {
    setV('settings.server.headerProtectionKey', Wireguard.keyToBase64(Wireguard.generatePresharedKey()));
    // The backend rejects a save otherwise (amneziawg.ValidateAwgVersion) --
    // raising the version ceiling here means clicking this one button is
    // enough to turn header protection on, instead of also sending the admin
    // hunting for a separate version dropdown.
    setV('settings.server.awgVersion', AWG_VERSION_3);
    // The backend rejects a save otherwise (amneziawg.ValidateHeaderProtection,
    // mirroring amneziawg-go's own IpcSet requirement) -- raising S1-S4 here
    // means clicking this one button is enough to turn header protection on,
    // instead of also sending the admin hunting for which S value is too low.
    // Only raises fields that are actually below the minimum; never lowers one.
    for (const field of ['s1', 's2', 's3', 's4']) {
      const current = getV(`settings.server.${field}`);
      if (typeof current === 'number' && current < 12) {
        setV(`settings.server.${field}`, 12);
      }
    }
  };

  // Mirrors the two regen handlers above in the other direction: dialing the
  // version ceiling back below 3 while headerProtectionKey/contentPaddingAddition
  // are still set would otherwise surface amneziawg.ValidateAwgVersion's error
  // at save time with no obvious way to fix it from the form -- clearing both
  // AWG3-only fields the moment the admin picks a lower version keeps the two
  // in sync automatically, the same "turning off this tier removes its
  // settings" direction the regen handlers already apply when turning it on.
  const onAwgVersionChange = (version: string) => {
    if (version !== AWG_VERSION_3) {
      setV('settings.server.headerProtectionKey', '');
      setV('settings.server.contentPaddingAddition', '');
      for (const field of AWG_TIMER_FIELDS) {
        setV(`settings.server.${field}`, '');
      }
      setV('settings.server.randomTrailers', false);
      setV('settings.server.disableCookies', false);
    }
  };

  // Suggests a modest "low-high" ContentPaddingAddition range -- there's no
  // backend generator to mirror (unlike H1-H4's generateHRanges), so this is
  // a standalone, deliberately conservative choice: a few tens of bytes of
  // extra per-packet padding is enough to blur packet-size fingerprinting
  // without adding much real bandwidth overhead, which a much larger range
  // (this field's grammar allows up to uint16 max) would.
  const regenInboundAwgContentPaddingAddition = () => {
    const randInt = (min: number, max: number) => min + Math.floor(Math.random() * (max - min + 1));
    const lo = randInt(0, 30);
    const hi = lo + randInt(20, 100);
    setV('settings.server.contentPaddingAddition', `${lo}-${hi}`);
    // Same reasoning as regenInboundAwgHeaderProtectionKey: contentPaddingAddition
    // is also AWG3-only (amneziawg.ValidateAwgVersion), so this button alone
    // should be enough to make the save succeed.
    setV('settings.server.awgVersion', AWG_VERSION_3);
  };

  // Fills one AWG3 device-timer field with a random "low-high" window
  // jittered around amneziawg-go's own real protocol default (see
  // amneziawg.tsx's own hint copy for each field's exact default), and
  // raises awgVersion to "3" -- same "one click is enough" reasoning as
  // regenInboundAwgHeaderProtectionKey above, since all 5 of these are
  // gated by the identical amneziawg.ValidateAwgVersion check.
  const regenAwgTimerField = (field: string, center: number, spread: number) => {
    const randInt = (min: number, max: number) => min + Math.floor(Math.random() * (max - min + 1));
    const lo = Math.max(1, center - randInt(1, spread));
    const hi = lo + randInt(Math.max(1, Math.floor(spread / 2)), spread * 2);
    setV(`settings.server.${field}`, `${lo}-${hi}`);
    setV('settings.server.awgVersion', AWG_VERSION_3);
  };
  const regenInboundAwgRekeyAfterTime = () => regenAwgTimerField('rekeyAfterTime', 120, 20);
  const regenInboundAwgRekeyTimeout = () => regenAwgTimerField('rekeyTimeout', 5, 3);
  const regenInboundAwgRejectAfterTime = () => regenAwgTimerField('rejectAfterTime', 180, 20);
  const regenInboundAwgKeepaliveTimeout = () => regenAwgTimerField('keepaliveTimeout', 10, 6);
  const regenInboundAwgMaxHandshakeAttempts = () => regenAwgTimerField('maxHandshakeAttempts', 18, 6);

  // Which mimicry profile the next generate click uses, one choice per CPS
  // slot (i1-i5) -- UI-only state, never persisted (the backend only ever
  // sees the resulting CPS chain, not which profile produced it, exactly
  // like the reference implementation this was ported from).
  const [cpsProfiles, setCpsProfiles] = useState<Record<CpsSlot, I1ProfileChoice>>({
    i1: 'random', i2: 'random', i3: 'random', i4: 'random', i5: 'random',
  });
  const onCpsProfileChange = (slot: CpsSlot, profile: I1ProfileChoice) => {
    setCpsProfiles((prev) => ({ ...prev, [slot]: profile }));
  };
  const onRegenCps = async (slot: CpsSlot) => {
    const profile = cpsProfiles[slot];
    // 'quic' builds a real AES-GCM/HKDF-sealed packet via crypto.subtle,
    // which needs a secure context (HTTPS or localhost). A 'random' pick
    // that lands on quic degrades silently (genI1 excludes quic from its
    // own pool when unsupported) -- but an admin who explicitly chose
    // quic should get a clear reason, not a button that quietly no-ops.
    if (profile === 'quic' && !isQuicI1Supported()) {
      messageApi.error(t('pages.xray.amneziawg.i1QuicUnsupported'));
      return;
    }
    const result = await genI1(profile);
    if (result) setV(`settings.server.${slot}`, result.chain);
  };

  // Sends a real QUICv1 Initial ClientHello to host:443 from the *panel
  // server* (not the browser -- raw UDP isn't available to page JS) and
  // turns the real server's reply into a CPS chain, applied directly to the
  // chosen slot. Doesn't touch cpsProfiles: capture is a separate action
  // from the per-slot mimicry-profile regenerate, not a new profile choice.
  const [quicCapturing, setQuicCapturing] = useState(false);
  const onQuicCapture = async (host: string, slot: CpsSlot) => {
    const trimmed = host.trim();
    if (!trimmed) {
      messageApi.warning(t('pages.xray.amneziawg.quicCaptureHostRequired'));
      return;
    }
    setQuicCapturing(true);
    try {
      const msg = await HttpUtil.post<QuicCaptureResult>(
        '/panel/api/server/awgQuicCapture',
        { host: trimmed },
        { silent: true },
      );
      if (!msg?.success || !msg.obj) {
        messageApi.error(msg?.msg || t('pages.xray.amneziawg.awgQuicCaptureError'));
        return;
      }
      setV(`settings.server.${slot}`, msg.obj.chain);
      messageApi.success(t('pages.xray.amneziawg.quicCaptureSuccess', { slot: slot.toUpperCase() }));
    } finally {
      setQuicCapturing(false);
    }
  };

  // Runs the same capture as onQuicCapture once per CPS slot, sequentially
  // (gentler on the target host than 5 parallel UDP probes, and each
  // handshake is independently random anyway -- no benefit to racing them).
  // Slots that succeed are written immediately, so a later failure doesn't
  // discard earlier progress.
  const onQuicCaptureAll = async (host: string) => {
    const trimmed = host.trim();
    if (!trimmed) {
      messageApi.warning(t('pages.xray.amneziawg.quicCaptureHostRequired'));
      return;
    }
    setQuicCapturing(true);
    try {
      let successCount = 0;
      for (const slot of CPS_SLOTS) {
        const msg = await HttpUtil.post<QuicCaptureResult>(
          '/panel/api/server/awgQuicCapture',
          { host: trimmed },
          { silent: true },
        );
        if (msg?.success && msg.obj) {
          setV(`settings.server.${slot}`, msg.obj.chain);
          successCount++;
        }
      }
      if (successCount === CPS_SLOTS.length) {
        messageApi.success(t('pages.xray.amneziawg.quicCaptureAllSuccess'));
      } else if (successCount > 0) {
        messageApi.warning(
          t('pages.xray.amneziawg.quicCaptureAllPartial', { success: successCount, total: CPS_SLOTS.length }),
        );
      } else {
        messageApi.error(t('pages.xray.amneziawg.awgQuicCaptureError'));
      }
    } finally {
      setQuicCapturing(false);
    }
  };

  // Randomizes the AmneziaWG 2.0 obfuscation set client-side, mirroring the
  // ranges/constraints of the Go backend's amneziawg.GenerateObfuscation20
  // "default" preset (internal/amneziawg/params.go) closely enough for a form
  // suggestion — exact parity isn't required since the user can still edit
  // any field afterward, and a fresh, non-crypto-grade random value here is
  // no weaker than what the backend would have generated on first save.
  const regenInboundAwgObfuscation = () => {
    const randInt = (min: number, max: number) => min + Math.floor(Math.random() * (max - min + 1));

    const jc = randInt(3, 6);
    const jmin = randInt(40, 89);
    const jmax = jmin + randInt(50, 250);

    const s1 = randInt(15, 150);
    let s2 = randInt(15, 150);
    while (s1 + 56 === s2) {
      s2 = randInt(15, 150);
    }
    const s3 = randInt(8, 55);
    const s4 = randInt(4, 27);

    // Four non-overlapping "low-high" ranges for H1-H4: split the space into
    // four bands and take a random sub-range from each (>= 1000 wide, low
    // bound >= 5 since 1-4 are reserved for vanilla WireGuard message types).
    const hMax = 2147483647;
    const hMinWidth = 1000;
    const lo = 5;
    const bandSize = Math.floor((hMax - lo + 1) / 4);
    const hRanges = Array.from({ length: 4 }, (_, i) => {
      const bandLo = lo + i * bandSize;
      const bandHi = bandLo + bandSize - 1;
      const start = randInt(bandLo, bandHi - hMinWidth - 1);
      const end = randInt(start + hMinWidth, bandHi - 1);
      return `${start}-${end}`;
    });

    setV('settings.server.jc', jc);
    setV('settings.server.jmin', jmin);
    setV('settings.server.jmax', jmax);
    setV('settings.server.s1', s1);
    setV('settings.server.s2', s2);
    setV('settings.server.s3', s3);
    setV('settings.server.s4', s4);
    setV('settings.server.h1', hRanges[0]);
    setV('settings.server.h2', hRanges[1]);
    setV('settings.server.h3', hRanges[2]);
    setV('settings.server.h4', hRanges[3]);
    for (const slot of CPS_SLOTS) {
      setV(`settings.server.${slot}`, `<r ${randInt(32, 256)}>`);
    }
  };

  const matchesVlessAuth = (
    block: { id?: string; label?: string } | undefined | null,
    authId: string,
  ) => {
    if (block?.id === authId) return true;
    const label = (block?.label || '').toLowerCase().replace(/[-_\s]/g, '');
    if (authId === 'mlkem768') return label.includes('mlkem768') && !label.includes('xorpub') && !label.includes('random');
    if (authId === 'x25519') return label.includes('x25519') && !label.includes('xorpub') && !label.includes('random');
    if (authId === 'mlkem768_xorpub') return label.includes('mlkem768') && label.includes('xorpub');
    if (authId === 'mlkem768_random') return label.includes('mlkem768') && label.includes('random');
    if (authId === 'x25519_xorpub') return label.includes('x25519') && label.includes('xorpub');
    if (authId === 'x25519_random') return label.includes('x25519') && label.includes('random');
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
      setV('settings.decryption', block.decryption);
      setV('settings.encryption', block.encryption);
    } finally {
      setSaving(false);
    }
  };

  const clearVlessEnc = () => {
    setV('settings.decryption', 'none');
    setV('settings.encryption', 'none');
  };

  const vlessAuthKind = vlessEncryptionAuthKind(
    typeof vlessEncryption === 'string' ? vlessEncryption : '',
  );
  const selectedVlessAuth = (() => {
    const enc = typeof vlessEncryption === 'string' ? vlessEncryption : '';
    if (!enc || enc === 'none') return 'None';
    if (!vlessAuthKind) return t('pages.inbounds.vlessAuthCustom');
    return t(VLESS_AUTH_LABEL_KEYS[vlessAuthKind]);
  })();

  useEffect(() => {
    if (!open) return;
    const initial = mode === 'edit' && dbInbound
      ? rawInboundToFormValues(dbInbound)
      : buildAddModeValues();
    if (initial.protocol === Protocols.AMNEZIAWG) {
      // A record saved before awgVersion existed can already have
      // headerProtectionKey/contentPaddingAddition set with no awgVersion at
      // all in its stored JSON (rawInboundToFormValues casts the settings
      // blob verbatim, no schema-default pass on read) -- resolve the
      // effective version here so the Select shows what's actually in
      // effect, instead of defaulting to "2" and having the very next
      // unrelated save rejected by the backend's own consistency check.
      const server = (initial.settings as Record<string, unknown> | undefined)?.server as
        | Record<string, unknown>
        | undefined;
      if (server) {
        server.awgVersion = effectiveAwgVersion(
          server.awgVersion as string | undefined,
          server.headerProtectionKey as string | undefined,
          server.contentPaddingAddition as string | undefined,
          ...AWG_TIMER_FIELDS.map((f) => server[f] as string | undefined),
        );
      }
    }
    methods.reset(initial);
    setScanResult(null);
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

    /* eslint-disable-next-line react-hooks/exhaustive-deps */
  }, [open, mode, dbInbound, methods]);

  useEffect(() => {
    if (!open) return;
    if (wTag === lastWrittenTagRef.current) return;
    autoTagRef.current = isAutoInboundTag(wTag, currentTagInput());
    /* eslint-disable-next-line react-hooks/exhaustive-deps */
  }, [open, wTag]);

  useEffect(() => {
    if (!open || !autoTagRef.current) return;
    const next = composeInboundTag(currentTagInput());
    if (next !== ((getV('tag') as string | undefined) ?? '')) {
      lastWrittenTagRef.current = next;
      setV('tag', next);
    }
    /* eslint-disable-next-line react-hooks/exhaustive-deps */
  }, [open, wPort, wNodeId, protocol, network, mixedUdpOn, wSsNetwork, wTunnelNetwork]);

  /*
   * Keep the strategy value inside the visible option set: when `node` isn't
   * offered (no node, or a protocol that can't deploy to one) fall back to
   * `listen`, which yields the same link for a local inbound. Mirrors how the
   * protocol reset drops a nodeId that no longer applies.
   * Only downgrade once the inputs this decision depends on are settled, so a
   * persisted `node` strategy is never clobbered by transient mount state (#5375).
   */
  useEffect(() => {
    if (!open) return;
    if (!availableNodesFetched || !protocol) return;
    const current = getV('shareAddrStrategy') as InboundFormValues['shareAddrStrategy'] | undefined;
    if (!nodeShareOptionAvailable && (current ?? 'node') === 'node') {
      setV('shareAddrStrategy', 'listen');
    }
    /* eslint-disable-next-line react-hooks/exhaustive-deps */
  }, [open, availableNodesFetched, protocol, nodeShareOptionAvailable, shareAddrStrategy]);

  /*
   * Protocol picker reset cascades through the form — clearing the settings DU
   * branch and dropping a nodeId that no longer applies. Only a real user
   * change (type === 'change') triggers it; programmatic setValue (advanced
   * JSON edits, open reset) must not, matching the legacy onValuesChange.
   */
  useEffect(() => {
    if (mode === 'edit') return;
    /* eslint-disable-next-line react-hooks/incompatible-library */
    const sub = methods.watch((_value, { name, type }) => {
      if (name !== 'protocol' || type !== 'change') return;
      const next = getV('protocol') as string;
      const settings = createDefaultInboundSettings(next) ?? undefined;
      setV('settings', settings);
      if (!NODE_ELIGIBLE_PROTOCOLS.has(next)) {
        setV('nodeId', null);
      }
      if (next === Protocols.HYSTERIA) {
        setV('streamSettings', {
          network: 'hysteria',
          security: 'tls',
          hysteriaSettings: HysteriaStreamSettingsSchema.parse({}),
          tlsSettings: createHysteriaTlsSettingsWithDefaultCert(),
          finalmask: {
            tcp: [],
            udp: [{
              type: 'salamander',
              settings: { password: RandomUtil.randomLowerAndNum(16) },
            }],
          },
        });
      } else if (next === Protocols.WIREGUARD || next === Protocols.TUNNEL) {
        setV('streamSettings', { security: 'none' });
      } else {
        const current = getV('streamSettings') as { network?: string } | undefined;
        if (current?.network === 'hysteria' || !current?.network) {
          setV('streamSettings', { network: 'tcp', security: 'none', tcpSettings: {} });
        }
      }
    });
    return () => sub.unsubscribe();
    /* eslint-disable-next-line react-hooks/exhaustive-deps */
  }, [mode, methods]);

  const submit = async () => {
    if (!(await methods.trigger())) return;
    /*
     * getValues() returns the entire form store, including settings.clients and
     * settings.fallbacks which have no bound field (clients are managed via the
     * standalone Client modal, not this inbound modal). With shouldUnregister
     * false those pass-through sub-trees survive from the reset object, so the
     * update wire payload never silently drops every client on save.
     */
    const values = methods.getValues() as InboundFormValues;
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
      <FormField name="enable" label={t('enable')} valueProp="checked">
        <Switch />
      </FormField>

      <FormField name="remark" label={t('pages.inbounds.remark')}>
        <Input />
      </FormField>

      {selectableNodes.length > 0 && isNodeEligible && (
        <FormField name="nodeId" label={t('pages.inbounds.deployTo')}>
          <Select
            showSearch
            disabled={mode === 'edit'}
            placeholder={t('pages.inbounds.localPanel')}
            allowClear
            options={selectableNodes.map((n) => ({
              value: n.id,
              label: `${n.name}${n.status === 'offline' ? ' (offline)' : ''}`,
              disabled: n.status === 'offline',
            }))}
          />
        </FormField>
      )}

      <FormField name="protocol" label={t('pages.inbounds.protocol')}>
        <Select id="protocol" disabled={mode === 'edit'} options={PROTOCOL_OPTIONS} />
      </FormField>

      <FormField
        name="listen"
        label={labelWithHint(t('pages.inbounds.address'), t('pages.inbounds.form.listenHelp'))}
      >
        <Input placeholder={t('pages.inbounds.monitorDesc')} />
      </FormField>

      <FormField
        name="shareAddrStrategy"
        label={labelWithHint(t('pages.inbounds.form.shareAddrStrategy'), t('pages.inbounds.form.shareAddrStrategyHelp'))}
      >
        <Select
          options={SHARE_ADDR_STRATEGIES
            .filter((strategy) => strategy !== 'node' || nodeShareOptionAvailable)
            .map((strategy) => ({
              value: strategy,
              label: t(`pages.inbounds.form.shareAddrStrategyOptions.${strategy}`),
            }))}
        />
      </FormField>

      {shareAddrStrategy === 'custom' && (
        <FormField
          name="shareAddr"
          label={labelWithHint(t('pages.inbounds.form.shareAddr'), t('pages.inbounds.form.shareAddrHelp'))}
          rules={{
            validate: (value) =>
              isValidShareAddrInput(String(value ?? '')) || t('pages.inbounds.form.shareAddrHelp'),
          }}
        >
          <Input placeholder="edge.example.com" />
        </FormField>
      )}

      <FormField
        name="subSortIndex"
        label={labelWithHint(t('pages.inbounds.form.subSortIndex'), t('pages.inbounds.form.subSortIndexHelp'))}
      >
        <InputNumber min={1} />
      </FormField>

      <FormField
        name="port"
        label={t('pages.inbounds.port')}
        rules={{ validate: rhfZodValidate(InboundFormBaseSchema.shape.port) }}
      >
        <InputNumber min={isUdsListen ? 0 : 1} max={65535} />
      </FormField>

      <Form.Item
        label={
          <Tooltip title={t('pages.inbounds.meansNoLimit')}>
            {t('pages.inbounds.totalFlow')}
          </Tooltip>
        }
      >
        <InputNumber
          value={wTotal ? Math.round((wTotal / SizeFormatter.ONE_GB) * 100) / 100 : 0}
          min={0}
          step={1}
          onChange={(v) => {
            const bytes = NumberFormatter.toFixed((Number(v) || 0) * SizeFormatter.ONE_GB, 0);
            setV('total', bytes);
          }}
        />
      </Form.Item>

      <FormField name="trafficReset" label={t('pages.inbounds.periodicTrafficResetTitle')}>
        <Select
          options={TRAFFIC_RESETS.map((r) => ({
            value: r,
            label: t(`pages.inbounds.periodicTrafficReset.${r}`),
          }))}
        />
      </FormField>

      {trafficReset === 'monthly' && (
        <FormField
          name="trafficResetDay"
          label={t('pages.inbounds.periodicTrafficResetDay')}
          rules={{ validate: rhfZodValidate(InboundDbFieldsSchema.shape.trafficResetDay) }}
        >
          <InputNumber min={1} max={31} />
        </FormField>
      )}

      <Form.Item
        label={
          <Tooltip title={t('pages.inbounds.leaveBlankToNeverExpire')}>
            {t('pages.inbounds.expireDate')}
          </Tooltip>
        }
      >
        <DateTimePicker
          value={wExpiry > 0 ? dayjs(wExpiry) : null}
          onChange={(d) => setV('expiryTime', d ? d.valueOf() : 0)}
        />
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
      {protocol === Protocols.WIREGUARD && <WireguardFields wgPubKey={wgPubKey} regenInboundWg={regenInboundWg} />}

      {protocol === Protocols.AMNEZIAWG && (
        <AmneziawgFields
          awgPubKey={awgPubKey}
          inboundId={mode === 'edit' && dbInbound ? dbInbound.id : null}
          regenInboundAwg={regenInboundAwg}
          regenInboundAwgObfuscation={regenInboundAwgObfuscation}
          regenInboundAwgHeaderProtectionKey={regenInboundAwgHeaderProtectionKey}
          regenInboundAwgContentPaddingAddition={regenInboundAwgContentPaddingAddition}
          onAwgVersionChange={onAwgVersionChange}
          regenInboundAwgRekeyAfterTime={regenInboundAwgRekeyAfterTime}
          regenInboundAwgRekeyTimeout={regenInboundAwgRekeyTimeout}
          regenInboundAwgRejectAfterTime={regenInboundAwgRejectAfterTime}
          regenInboundAwgKeepaliveTimeout={regenInboundAwgKeepaliveTimeout}
          regenInboundAwgMaxHandshakeAttempts={regenInboundAwgMaxHandshakeAttempts}
          cpsProfiles={cpsProfiles}
          onCpsProfileChange={onCpsProfileChange}
          onRegenCps={onRegenCps}
          quicCapturing={quicCapturing}
          onQuicCapture={onQuicCapture}
          onQuicCaptureAll={onQuicCaptureAll}
        />
      )}

      {protocol === Protocols.TUN && <TunFields />}

      {protocol === Protocols.TUNNEL && <TunnelFields />}

      {protocol === Protocols.HTTP && <HttpFields />}
      {protocol === Protocols.MIXED && <MixedFields mixedUdpOn={mixedUdpOn} />}

      {protocol === Protocols.MTPROTO && <MtprotoFields />}

      {protocol === Protocols.SHADOWSOCKS && <ShadowsocksFields isSSWith2022={isSSWith2022} />}

      {protocol === Protocols.VLESS && <VlessFields saving={saving} selectedVlessAuth={selectedVlessAuth} vlessAuthKind={vlessAuthKind} network={network} security={security} getNewVlessEnc={getNewVlessEnc} clearVlessEnc={clearVlessEnc} />}

      {isFallbackHost && fallbacksCard}
      {(protocol === Protocols.VLESS || protocol === Protocols.TROJAN)
        && network === 'tcp' && !isFallbackHost && (
          <Alert
            className="mt-12"
            type="info"
            showIcon
            title={t('pages.inbounds.fallbacks.needsTls')}
          />
        )}
    </>
  );

  /*
   * Switching `network` swaps which per-network key appears on the wire. Clear
   * the old network's blob and seed the new one with schema defaults, plus the
   * FinalMask mkcp-legacy UDP mask when moving to mKCP (removed otherwise).
   */
  const onNetworkChange = (next: string) => {
    const ALL = ['tcpSettings', 'kcpSettings', 'wsSettings', 'grpcSettings', 'httpupgradeSettings', 'xhttpSettings'];
    const current = (getV('streamSettings') as Record<string, unknown>) ?? {};
    const cleaned: Record<string, unknown> = { ...current, network: next };
    for (const k of ALL) {
      if (k !== `${next}Settings`) delete cleaned[k];
    }
    cleaned[`${next}Settings`] = newStreamSlice(next);
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
    } else {
      const fm = cleaned.finalmask as Record<string, unknown> | undefined;
      if (fm && Array.isArray(fm.udp)) {
        const udp = (fm.udp as unknown[]).filter((m) => (m as { type?: string })?.type !== 'mkcp-legacy');
        cleaned.finalmask = { ...fm, udp };
      }
    }
    setV('streamSettings', cleaned);
  };

  const streamTab = (
    <>
      {hasSelectableTransport && (
        <Form.Item label={t('transmission')}>
          <Select
            style={{ width: '75%' }}
            value={network}
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
          dropdown is hidden above. */}
      {protocol === Protocols.HYSTERIA && <HysteriaFields />}

      {hasSelectableTransport && (
        <>
          {network === 'tcp' && <RawForm />}

          {network === 'ws' && <WsForm />}

          {network === 'grpc' && <GrpcForm />}

          {network === 'xhttp' && <XhttpForm />}

          {network === 'httpupgrade' && <HttpUpgradeForm />}

          {network === 'kcp' && <KcpForm />}
        </>
      )}

      {/* The legacy externalProxy section is replaced by the Hosts page; the
          field is still parsed/rendered for backward compatibility but is no
          longer editable here. */}

      <SockoptForm toggleSockopt={toggleSockopt} network={network} />

      {/* Transport masks don't apply to tunnel (a transparent forwarder), so
          its stream tab is just sockopt + TProxy. */}
      {protocol !== Protocols.TUNNEL && (
        <Controller
          control={control}
          name="streamSettings.finalmask"
          render={({ field }) => (
            <FinalMaskField
              key={`${protocol}:${network}`}
              value={field.value}
              onChange={field.onChange}
              network={network}
              protocol={protocol}
            />
          )}
        />
      )}
    </>
  );

  const tlsOk = canEnableTls({ protocol, streamSettings: { network, security } });
  const realityOk = canEnableReality({ protocol, streamSettings: { network, security } });
  const tlsOnly = protocol === Protocols.HYSTERIA;

  const securityTab = (
    <>
      <Form.Item label={t('pages.inbounds.securityTab')}>
        <Radio.Group
          value={security}
          buttonStyle="solid"
          disabled={!tlsOk}
          onChange={(e) => onSecurityChange(e.target.value)}
        >
          {!tlsOnly && <Radio.Button value="none">{t('none')}</Radio.Button>}
          <Radio.Button value="tls">TLS</Radio.Button>
          {realityOk && <Radio.Button value="reality">Reality</Radio.Button>}
        </Radio.Group>
      </Form.Item>

      {security === 'tls' && (
        <TlsForm
          saving={saving}
          setCertFromPanel={setCertFromPanel}
          clearCertFiles={clearCertFiles}
          pinFromCert={pinFromCert}
          pinFromRemote={pinFromRemote}
          getNewEchCert={getNewEchCert}
          clearEchCert={clearEchCert}
        />
      )}

      {security === 'reality' && (
        <RealityForm
          saving={saving}
          scanning={scanning}
          scanResult={scanResult}
          scanRealityTarget={scanRealityTarget}
          scanRealityCandidates={scanRealityCandidates}
          applyRealityScanResult={applyRealityScanResult}
          randomizeShortIds={randomizeShortIds}
          randomizeSpiderX={randomizeSpiderX}
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
                  <AdvancedAllEditor streamEnabled={streamEnabled} sniffingEnabled={sniffingSupported} />
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
                      path="streamSettings"
                      wrapKey="streamSettings"
                      minHeight="320px"
                      maxHeight="540px"
                    />
                  </>
                ),
              }]
              : []),
            ...(sniffingSupported
              ? [{
                key: 'sniffing',
                label: t('pages.inbounds.advanced.sniffing'),
                children: (
                  <>
                    <div className="advanced-editor-meta">
                      {t('pages.inbounds.advanced.sniffingHelp')}{' '}
                      <code>{'{ sniffing: { ... } }'}</code>.
                    </div>
                    <AdvancedSliceEditor
                      path="sniffing"
                      wrapKey="sniffing"
                      minHeight="240px"
                      maxHeight="420px"
                    />
                  </>
                ),
              }]
              : []),
          ]}
        />
      </div>
    </div>
  );

  const sniffingTab = <SniffingTab />;

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
        <FormProvider {...methods}>
          <Form
            colon={false}
            labelCol={{ sm: { span: 8 } }}
            wrapperCol={{ sm: { span: 14 } }}
            labelWrap
          >
            <Tabs items={[
              { key: 'basic', label: t('pages.xray.basicTemplate'), children: basicTab, forceRender: true },
              ...(([
                Protocols.VLESS,
                Protocols.SHADOWSOCKS,
                Protocols.HTTP,
                Protocols.MIXED,
                Protocols.TUNNEL,
                Protocols.TUN,
                Protocols.WIREGUARD,
                Protocols.MTPROTO,
                Protocols.AMNEZIAWG,
              ] as string[]).includes(protocol) || isFallbackHost
                ? [{ key: 'protocol', label: t('pages.inbounds.protocol'), children: protocolTab, forceRender: true }]
                : []),
              ...(streamEnabled
                ? [
                  { key: 'stream', label: t('pages.inbounds.streamTab'), children: streamTab, forceRender: true },
                  ...(protocol !== Protocols.WIREGUARD && protocol !== Protocols.TUNNEL
                    ? [{ key: 'security', label: t('pages.inbounds.securityTab'), children: securityTab, forceRender: true }]
                    : []),
                ]
                : []),
              ...(sniffingSupported
                ? [{ key: 'sniffing', label: t('pages.inbounds.sniffingTab'), children: sniffingTab, forceRender: true }]
                : []),
              { key: 'advanced', label: t('pages.xray.advancedTemplate'), children: advancedTab, forceRender: true },
            ]} />
          </Form>
        </FormProvider>
      </Modal>
    </>
  );
}
