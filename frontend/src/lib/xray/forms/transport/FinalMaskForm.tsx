import i18next from 'i18next';
import { useEffect, useRef, useState } from 'react';
import {
  AutoComplete,
  Button,
  Divider,
  Form,
  Input,
  InputNumber,
  Select,
  Space,
  Switch,
  Typography,
} from 'antd';
import { DeleteOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { FormInstance } from 'antd/es/form';
import type { NamePath } from 'antd/es/form/interface';

import { RandomUtil } from '@/utils';
import { activateOnKey } from '@/utils/a11y';
import { OutboundProtocols, UTLS_FINGERPRINT } from '@/schemas/primitives';
import { upgradeLegacyUdpHop } from '@/lib/xray/udphop-mask';
import { upgradeLegacyXdnsMasks, XDNS_LEGACY_EDNS0 } from '@/lib/xray/xdns-mask';

const UTLS_FINGERPRINT_OPTIONS = Object.values(UTLS_FINGERPRINT).map((value) => ({
  value,
  label: value,
}));

export interface FinalMaskFormProps {
  name: NamePath;
  network: string;
  protocol: string;
  form: FormInstance;
  // When true, all sections (TCP / UDP / QUIC) are shown regardless of
  // network/protocol. Used by the global sub-JSON finalmask editor where
  // the masks apply to every stream rather than one specific transport.
  showAll?: boolean;
  // udphop runs only on the dialing side (the core refuses it on a listener), so a server
  // keeps the advertised hop range in quicParams.udpHop and a client gets the mask.
  side?: 'server' | 'client';
}

const TCP_NETWORKS = ['raw', 'tcp', 'httpupgrade', 'ws', 'grpc', 'xhttp', 'xdrive'];
const DEFAULT_GECKO_PACKET_SIZE = { min: 512, max: 1200 };
// Xray-core caps the Gecko output packet size at its internal buffer (2048)
// and needs 1 <= min <= max; mirror those bounds so the panel rejects what
// core would reject at runtime (salamander/conn.go).
const GECKO_MIN_PACKET_SIZE = 1;
const GECKO_MAX_PACKET_SIZE = 2048;

export function parseGeckoPacketSize(value: unknown): { min: number; max: number } | null {
  const str = typeof value === 'string' ? value.trim() : String(value ?? '').trim();
  const match = /^(\d+)-(\d+)$/.exec(str);
  if (!match) return null;
  const min = Number(match[1]);
  const max = Number(match[2]);
  if (
    !Number.isSafeInteger(min) ||
    !Number.isSafeInteger(max) ||
    min < GECKO_MIN_PACKET_SIZE ||
    max < min ||
    max > GECKO_MAX_PACKET_SIZE
  ) {
    return null;
  }
  return { min, max };
}

function formatGeckoPacketSize(min: number, max: number): string {
  return `${min}-${max}`;
}

function splitGeckoPacketSize(value: unknown): { min: number | null; max: number | null } {
  const str = typeof value === 'string' ? value.trim() : String(value ?? '').trim();
  const [minRaw = '', maxRaw = ''] = str.split('-', 2);
  const min = /^\d+$/.test(minRaw) ? Number(minRaw) : null;
  const max = /^\d+$/.test(maxRaw) ? Number(maxRaw) : null;
  return { min, max };
}

function validateGeckoPacketSize(_rule: unknown, value: unknown): Promise<void> {
  if (parseGeckoPacketSize(value)) return Promise.resolve();
  return Promise.reject(
    new Error(
      `Use a range like 512-1200 (${GECKO_MIN_PACKET_SIZE}-${GECKO_MAX_PACKET_SIZE}, max ≥ min)`,
    ),
  );
}

// xray-core's salamander smPSKMinLen counts bytes; a shorter password loads, then the listener
// (stopping Xray) or the dial fails, so the backend refuses it at save too.
const SALAMANDER_MIN_PSK_BYTES = 4;

function validateSalamanderPassword(_rule: unknown, value: unknown): Promise<void> {
  const bytes = new TextEncoder().encode(typeof value === 'string' ? value : '').length;
  if (bytes >= SALAMANDER_MIN_PSK_BYTES) return Promise.resolve();
  return Promise.reject(
    new Error(
      `Use at least ${SALAMANDER_MIN_PSK_BYTES} bytes; xray-core refuses a shorter password`,
    ),
  );
}

function asPath(name: NamePath): (string | number)[] {
  return Array.isArray(name) ? [...name] : [name];
}

function defaultTcpMaskSettings(type: string): Record<string, unknown> {
  switch (type) {
    case 'fragment':
      // `lengths`/`delays` are per-segment range arrays (xray-core #6334);
      // a single length entry reproduces the legacy single-range behavior.
      return { packets: '1-3', lengths: ['100-200'], delays: [], maxSplit: '' };
    case 'sudoku':
      return defaultSudokuSettings();
    case 'header-custom':
      return { clients: [], servers: [] };
    case 'xmc':
      return {
        hostname: '',
        profiles: [defaultXmcProfile()],
        password: RandomUtil.randomLowerAndNum(16),
      };
    default:
      return {};
  }
}

function defaultSudokuSettings(): Record<string, unknown> {
  return {
    password: '',
    ascii: '',
    customTable: '',
    customTables: [],
    paddingMin: 0,
    paddingMax: 0,
  };
}

function defaultXmcProfile(): Record<string, unknown> {
  return { username: '', uuid: '', texturesValue: '', texturesSignature: '' };
}

// xray-core #6487 replaced the xmc mask's `usernames` string list with
// `profiles` objects carrying a Mojang-signed session profile, and dropped the
// "default to Dream" fallback so at least one complete profile is now
// mandatory. The signature can only come from Mojang's session server, so a
// legacy username cannot be upgraded automatically — carry it into a profile
// stub instead, which keeps the operator's player names visible and leaves the
// per-field validators pointing at exactly what still has to be filled in.
export function migrateXmcSettings(settings: Record<string, unknown>): {
  next: Record<string, unknown>;
  changed: boolean;
} {
  const out: Record<string, unknown> = { ...settings };
  let changed = false;
  if (!Array.isArray(out.profiles) && Array.isArray(out.usernames)) {
    out.profiles = out.usernames
      .filter((name): name is string => typeof name === 'string' && name.trim() !== '')
      .map((name) => ({ ...defaultXmcProfile(), username: name }));
    changed = true;
  }
  if ('usernames' in out) {
    delete out.usernames;
    changed = true;
  }
  if (!Array.isArray(out.profiles)) {
    out.profiles = [];
    changed = true;
  }
  return { next: out, changed };
}

// xray-core #6334 replaced a fragment mask's single `length`/`delay` ranges
// with `lengths`/`delays` arrays (the singular keys remain in core only as a
// fallback). Lift any legacy singular value into a one-element array so the
// list UI shows it, and drop the singular key so we never emit both.
function migrateFragmentSettings(settings: Record<string, unknown>): {
  next: Record<string, unknown>;
  changed: boolean;
} {
  const out: Record<string, unknown> = { ...settings };
  let changed = false;
  if (!Array.isArray(out.lengths) && typeof out.length === 'string' && out.length.trim() !== '') {
    out.lengths = [out.length];
    changed = true;
  }
  if ('length' in out) {
    delete out.length;
    changed = true;
  }
  if (!Array.isArray(out.delays) && typeof out.delay === 'string' && out.delay.trim() !== '') {
    out.delays = [out.delay];
    changed = true;
  }
  if ('delay' in out) {
    delete out.delay;
    changed = true;
  }
  return { next: out, changed };
}

function defaultUdpMaskSettings(type: string): Record<string, unknown> {
  switch (type) {
    case 'salamander':
      return { password: '' };
    case 'mkcp-legacy':
      return { header: '', value: '' };
    case 'xdns':
      return { domains: [] };
    case 'xicmp':
      return { dgram: false, ips: [] };
    case 'realm':
      return { url: '', stunServers: [] };
    case 'header-custom':
      return { client: [], server: [] };
    case 'noise':
      return { reset: 0, noise: [] };
    case 'sudoku':
      return defaultSudokuSettings();
    case 'udphop':
      return { mode: 'intervalRemote', interval: '5-10', remotePorts: '' };
    default:
      return {};
  }
}

function defaultClientServerItem(): Record<string, unknown> {
  return { delay: 0, rand: 0, randRange: '0-255', type: 'array', packet: [] };
}

function defaultUdpClientServerItem(): Record<string, unknown> {
  return { rand: 0, randRange: '0-255', type: 'array', packet: [] };
}

function defaultNoiseItem(): Record<string, unknown> {
  return {
    rand: '1-8192',
    randRange: '0-255',
    type: 'array',
    packet: [],
    delay: '10-20',
  };
}

function defaultQuicParams(): Record<string, unknown> {
  return {
    congestion: 'bbr',
    debug: false,
    maxIdleTimeout: 30,
    keepAlivePeriod: 10,
    disablePathMTUDiscovery: false,
    maxIncomingStreams: 1024,
    initStreamReceiveWindow: 8388608,
    maxStreamReceiveWindow: 8388608,
    initConnectionReceiveWindow: 20971520,
    maxConnectionReceiveWindow: 20971520,
  };
}

function defaultUdpHop(): Record<string, unknown> {
  return { ports: '20000-50000', interval: '5-10' };
}

export default function FinalMaskForm({
  name,
  network,
  protocol,
  form,
  showAll = false,
  side = 'client',
}: FinalMaskFormProps) {
  const base = asPath(name);

  // Migrate legacy mask shapes once on mount so configs saved before #6334 (fragment
  // ranges), #6487 (xmc profiles) and #7090 (xdns names/addrs) render in the list UI.
  const migratedRef = useRef(false);
  useEffect(() => {
    if (migratedRef.current) return;
    migratedRef.current = true;
    const tcp = form.getFieldValue([...base, 'tcp']);
    if (Array.isArray(tcp)) {
      let anyChanged = false;
      const next = tcp.map((mask) => {
        if (!mask || typeof mask !== 'object') return mask;
        const m = mask as Record<string, unknown>;
        if (m.type !== 'fragment' && m.type !== 'xmc') return mask;
        if (!m.settings || typeof m.settings !== 'object') return mask;
        const settings = m.settings as Record<string, unknown>;
        const { next: migrated, changed } =
          m.type === 'fragment' ? migrateFragmentSettings(settings) : migrateXmcSettings(settings);
        if (!changed) return mask;
        anyChanged = true;
        return { ...m, settings: migrated };
      });
      if (anyChanged) form.setFieldValue([...base, 'tcp'], next);
    }
    const udp = form.getFieldValue([...base, 'udp']);
    if (Array.isArray(udp)) {
      const { next, changed } = upgradeLegacyXdnsMasks(udp);
      if (changed) form.setFieldValue([...base, 'udp'], next);
    }
    if (side === 'client') {
      const finalmask = form.getFieldValue(base) as Record<string, unknown> | undefined;
      if (finalmask && typeof finalmask === 'object') {
        const { next, changed } = upgradeLegacyUdpHop(finalmask);
        if (changed) form.setFieldValue(base, next);
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const isHysteria = protocol === OutboundProtocols.Hysteria || protocol === 'hysteria';
  // Wireguard carries no user-selectable transport (always a UDP listener/
  // dialer), so only the UDP mask section applies — TCP masks would never
  // wrap anything even though the leftover network value may be 'tcp'.
  const isWireguard = protocol === 'wireguard';
  const showTcp = showAll || (!isWireguard && TCP_NETWORKS.includes(network));
  // xhttp's HTTP/3 dial runs the UDP masks too, which is where a client's udphop lives.
  const showUdp =
    showAll ||
    isHysteria ||
    isWireguard ||
    network === 'kcp' ||
    (side === 'client' && network === 'xhttp');
  const showQuic = showAll || isHysteria || network === 'xhttp';
  const quicParams = Form.useWatch([...base, 'quicParams'], { form, preserve: true });
  const hasQuicParams = quicParams != null;

  if (!showTcp && !showUdp && !showQuic) return null;

  return (
    <>
      {showTcp && <TcpMasksList base={base} form={form} />}
      {showUdp && (
        <UdpMasksList
          base={base}
          form={form}
          isHysteria={isHysteria}
          isWireguard={isWireguard}
          network={network}
          allowUdpHop={side === 'client'}
        />
      )}
      {showQuic && (
        <>
          <Form.Item label={i18next.t('ui.qUICParams')}>
            <Switch
              checked={hasQuicParams}
              onChange={(v) => {
                form.setFieldValue([...base, 'quicParams'], v ? defaultQuicParams() : undefined);
              }}
            />
          </Form.Item>
          {hasQuicParams && (
            <QuicParamsForm
              base={[...base, 'quicParams']}
              form={form}
              showUdpHop={side === 'server'}
            />
          )}
        </>
      )}
    </>
  );
}

function TcpMasksList({ base, form }: { base: (string | number)[]; form: FormInstance }) {
  const { t } = useTranslation();
  return (
    <Form.List name={[...base, 'tcp']}>
      {(fields, { add, remove }) => (
        <>
          <Form.Item label={i18next.t('ui.tCPMasks')}>
            <Button
              type="primary"
              size="small"
              icon={<PlusOutlined />}
              aria-label={t('add')}
              onClick={() =>
                add({ type: 'fragment', settings: defaultTcpMaskSettings('fragment') })
              }
            />
          </Form.Item>
          {fields.map((field, mIdx) => (
            <TcpMaskItem
              key={field.key}
              fieldName={field.name}
              displayIndex={mIdx + 1}
              form={form}
              listPath={[...base, 'tcp']}
              onRemove={() => remove(field.name)}
            />
          ))}
        </>
      )}
    </Form.List>
  );
}

function TcpMaskItem({
  fieldName,
  displayIndex,
  form,
  listPath,
  onRemove,
}: {
  fieldName: number;
  displayIndex: number;
  form: FormInstance;
  listPath: (string | number)[];
  onRemove: () => void;
}) {
  // Absolute path for setFieldValue side effects (resetting settings on
  // type change). All Form.Item `name=` use RELATIVE paths within the
  // outer Form.List context.
  const absolutePath = [...listPath, fieldName];
  const { t } = useTranslation();

  return (
    <div>
      <Divider style={{ margin: 0 }}>
        {i18next.t('ui.tCPMask')} {displayIndex}
        <DeleteOutlined
          className="danger-icon"
          role="button"
          tabIndex={0}
          aria-label={t('remove')}
          onClick={onRemove}
          onKeyDown={activateOnKey(onRemove)}
        />
      </Divider>

      <Form.Item label={i18next.t('pages.inbounds.form.type')} name={[fieldName, 'type']}>
        <Select
          onChange={(v) =>
            form.setFieldValue([...absolutePath, 'settings'], defaultTcpMaskSettings(v))
          }
          options={[
            { value: 'fragment', label: i18next.t('ui.fragment') },
            { value: 'header-custom', label: i18next.t('ui.headerCustom') },
            { value: 'sudoku', label: 'Sudoku' },
            { value: 'xmc', label: 'XMC (Minecraft)' },
          ]}
        />
      </Form.Item>

      <Form.Item
        noStyle
        shouldUpdate={(prev, curr) => {
          const a = getDeep(prev, [...absolutePath, 'type']);
          const b = getDeep(curr, [...absolutePath, 'type']);
          return a !== b;
        }}
      >
        {({ getFieldValue }) => {
          const type = getFieldValue([...absolutePath, 'type']) as string | undefined;
          if (type === 'fragment') {
            return (
              <>
                <Form.Item
                  label={i18next.t('pages.settings.subFormats.packets')}
                  name={[fieldName, 'settings', 'packets']}
                  rules={[{ validator: validateFragmentPackets }]}
                >
                  <AutoComplete
                    options={[
                      { value: 'tlshello', label: 'tlshello' },
                      { value: '1-3', label: '1-3' },
                      { value: '1-5', label: '1-5' },
                    ]}
                    placeholder={i18next.t('ui.tlshelloOrNMEG13')}
                  />
                </Form.Item>
                <FragmentRangeList
                  listName={[fieldName, 'settings', 'lengths']}
                  label={i18next.t('ui.lengths')}
                  placeholder="e.g. 100-200"
                  minItems={1}
                  validator={validateFragmentLength}
                />
                <FragmentRangeList
                  listName={[fieldName, 'settings', 'delays']}
                  label={i18next.t('ui.delays')}
                  placeholder="e.g. 10-20 or 0"
                  validator={validateFragmentDelayEntry}
                />
                <Form.Item
                  label={i18next.t('ui.maxSplit')}
                  name={[fieldName, 'settings', 'maxSplit']}
                >
                  <Input />
                </Form.Item>
              </>
            );
          }
          if (type === 'sudoku') return <SudokuSettings fieldName={fieldName} />;
          if (type === 'header-custom') {
            return (
              <HeaderCustomGroups
                tcpFieldName={fieldName}
                form={form}
                absoluteSettingsPath={[...absolutePath, 'settings']}
              />
            );
          }
          if (type === 'xmc') {
            return (
              <>
                <Form.Item
                  label={i18next.t('ui.hostname')}
                  name={[fieldName, 'settings', 'hostname']}
                >
                  <Input placeholder={i18next.t('ui.serverAddressMimickedInTheHandshake')} />
                </Form.Item>
                <XmcProfilesList tcpFieldName={fieldName} />
                <Form.Item label={i18next.t('password')} required>
                  <Space.Compact block>
                    <Form.Item
                      name={[fieldName, 'settings', 'password']}
                      noStyle
                      rules={[
                        { required: true, message: i18next.t('pages.login.toasts.emptyPassword') },
                      ]}
                    >
                      <Input
                        placeholder={i18next.t('ui.obfuscationPassword')}
                        style={{ width: 'calc(100% - 32px)' }}
                      />
                    </Form.Item>
                    <Button
                      icon={<ReloadOutlined />}
                      aria-label={t('regenerate')}
                      onClick={() =>
                        form.setFieldValue(
                          [...absolutePath, 'settings', 'password'],
                          RandomUtil.randomLowerAndNum(16),
                        )
                      }
                    />
                  </Space.Compact>
                </Form.Item>
              </>
            );
          }
          return null;
        }}
      </Form.Item>
    </div>
  );
}

function SudokuSettings({ fieldName }: { fieldName: number }) {
  return (
    <>
      <Form.Item label={i18next.t('password')} name={[fieldName, 'settings', 'password']}>
        <Input />
      </Form.Item>
      <Form.Item label="ASCII" name={[fieldName, 'settings', 'ascii']}>
        <Input />
      </Form.Item>
      <Form.Item label={i18next.t('ui.customTable')} name={[fieldName, 'settings', 'customTable']}>
        <Input />
      </Form.Item>
      <Form.Item
        label={i18next.t('ui.customTables')}
        name={[fieldName, 'settings', 'customTables']}
      >
        <Select mode="tags" style={{ width: '100%' }} tokenSeparators={[',']} />
      </Form.Item>
      <Form.Item label={i18next.t('ui.paddingMin')} name={[fieldName, 'settings', 'paddingMin']}>
        <InputNumber min={0} />
      </Form.Item>
      <Form.Item label={i18next.t('ui.paddingMax')} name={[fieldName, 'settings', 'paddingMax']}>
        <InputNumber min={0} />
      </Form.Item>
    </>
  );
}

// xray's fragment `packets` accepts "tlshello" or an arbitrary packet-number
// range like "1-3" (#5075 — presets only covered the common cases).
function validateFragmentPackets(_rule: unknown, value: unknown): Promise<void> {
  const str = typeof value === 'string' ? value.trim() : String(value ?? '').trim();
  if (str.length === 0 || str === 'tlshello' || /^\d+-\d+$/.test(str)) {
    return Promise.resolve();
  }
  return Promise.reject(new Error('Use "tlshello" or a packet range like 1-3'));
}

function validateFragmentLength(_rule: unknown, value: unknown): Promise<void> {
  const str = typeof value === 'string' ? value.trim() : String(value ?? '').trim();
  if (str.length === 0) {
    return Promise.reject(
      new Error('Length is required — xray rejects a fragment mask whose LengthMin is 0'),
    );
  }
  const min = Number(str.split('-')[0]);
  if (!Number.isFinite(min) || min <= 0) {
    return Promise.reject(new Error('Length minimum must be greater than 0 (e.g. 100-200)'));
  }
  return Promise.resolve();
}

// A delay segment is a millisecond value or range; 0 is allowed (no delay),
// but an empty row would serialize as "" and break xray's Int32Range parse,
// so require a value and let the user remove the row instead.
function validateFragmentDelayEntry(_rule: unknown, value: unknown): Promise<void> {
  const str = typeof value === 'string' ? value.trim() : String(value ?? '').trim();
  if (str.length === 0) {
    return Promise.reject(
      new Error("Delay is required — remove the row if you don't want a delay"),
    );
  }
  if (!/^\d+(?:-\d+)?$/.test(str)) {
    return Promise.reject(new Error('Use a delay in ms, e.g. 10 or 10-20'));
  }
  return Promise.resolve();
}

// Per-segment range list for a fragment mask's `lengths`/`delays` (xray-core
// #6334): an editable list of dash-range strings. xray applies entry N to
// fragment segment N, clamping to the last entry. `minItems` keeps at least
// one length row so the config never collapses to an empty (rejected) list.
function FragmentRangeList({
  listName,
  label,
  placeholder,
  validator,
  minItems = 0,
}: {
  listName: (string | number)[];
  label: string;
  placeholder: string;
  validator?: (rule: unknown, value: unknown) => Promise<void>;
  minItems?: number;
}) {
  const { t } = useTranslation();
  return (
    <Form.List name={listName}>
      {(fields, { add, remove }) => (
        <>
          <Form.Item label={label}>
            <Button
              type="primary"
              size="small"
              icon={<PlusOutlined />}
              aria-label={t('add')}
              onClick={() => add('')}
            />
          </Form.Item>
          {fields.map((field, idx) => (
            <Form.Item
              key={field.key}
              label={`#${idx + 1}`}
              name={field.name}
              rules={validator ? [{ validator }] : undefined}
            >
              <Input
                placeholder={placeholder}
                suffix={
                  fields.length > minItems ? (
                    <DeleteOutlined
                      className="danger-icon"
                      role="button"
                      tabIndex={0}
                      aria-label={t('remove')}
                      onClick={() => remove(field.name)}
                      onKeyDown={activateOnKey(() => remove(field.name))}
                    />
                  ) : null
                }
              />
            </Form.Item>
          ))}
        </>
      )}
    </Form.List>
  );
}

// randRange bytes must sit in 0-255 — xray rejects the whole config with
// "invalid randRange" otherwise (reversed ranges like "200-100" are fine,
// xray reorders them).
function validateRandRange(_rule: unknown, value: unknown): Promise<void> {
  const str = typeof value === 'string' ? value.trim() : String(value ?? '').trim();
  if (str.length === 0) return Promise.resolve();
  const m = /^(\d{1,3})(?:-(\d{1,3}))?$/.exec(str);
  if (!m) return Promise.reject(new Error('Use a byte value or range like 0-255'));
  const from = Number(m[1]);
  const to = m[2] !== undefined ? Number(m[2]) : from;
  if (from > 255 || to > 255) {
    return Promise.reject(new Error('randRange bytes must be within 0-255'));
  }
  return Promise.resolve();
}

function getDeep(obj: unknown, path: (string | number)[]): unknown {
  let cur: unknown = obj;
  for (const key of path) {
    if (cur == null || typeof cur !== 'object') return undefined;
    cur = (cur as Record<string | number, unknown>)[key];
  }
  return cur;
}

// Mojang hands the profile UUID back undashed from the session server and
// dashed from most other endpoints; xray-core parses either, so accept both
// rather than forcing the operator to reformat what they pasted.
const XMC_UUID_PATTERN =
  /^(?:[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}|[0-9a-fA-F]{32})$/;
const XMC_USERNAME_PATTERN = /^[A-Za-z0-9_]{3,16}$/;

function validateXmcUsername(_rule: unknown, value: unknown): Promise<void> {
  if (typeof value === 'string' && XMC_USERNAME_PATTERN.test(value)) return Promise.resolve();
  return Promise.reject(new Error('3-16 characters, letters/digits/underscore only'));
}

function validateXmcUuid(_rule: unknown, value: unknown): Promise<void> {
  if (typeof value === 'string' && XMC_UUID_PATTERN.test(value.trim())) return Promise.resolve();
  return Promise.reject(new Error('Enter the profile UUID (dashed or 32 hex characters)'));
}

// Each mask needs at least one fully signed profile since xray-core #6487 —
// an empty or partial list makes the core reject the whole config, so the
// panel blocks the save here rather than letting the backend drop the mask.
function XmcProfilesList({ tcpFieldName }: { tcpFieldName: number }) {
  const { t } = useTranslation();
  return (
    <Form.List name={[tcpFieldName, 'settings', 'profiles']}>
      {(profiles, { add, remove }) => (
        <>
          <Form.Item
            label={i18next.t('ui.profiles')}
            extra="Signed Minecraft session profiles; resolve the UUID by username, then fetch the profile with unsigned=false."
          >
            <Button
              type="primary"
              size="small"
              icon={<PlusOutlined />}
              aria-label={t('add')}
              onClick={() => add(defaultXmcProfile())}
            />
          </Form.Item>
          {profiles.map((profile, idx) => (
            <div key={profile.key}>
              <Divider style={{ margin: 0 }}>
                {i18next.t('pages.settings.profile')} {idx + 1}
                <DeleteOutlined
                  className="danger-icon"
                  role="button"
                  tabIndex={0}
                  aria-label={t('remove')}
                  onClick={() => remove(profile.name)}
                  onKeyDown={activateOnKey(() => remove(profile.name))}
                />
              </Divider>
              <Form.Item
                label={i18next.t('username')}
                name={[profile.name, 'username']}
                rules={[{ validator: validateXmcUsername }]}
              >
                <Input placeholder="Notch" />
              </Form.Item>
              <Form.Item
                label="UUID"
                name={[profile.name, 'uuid']}
                rules={[{ validator: validateXmcUuid }]}
              >
                <Input placeholder="069a79f4-44e9-4726-a5be-fca90e38aaf5" />
              </Form.Item>
              <Form.Item
                label={i18next.t('ui.texturesValue')}
                name={[profile.name, 'texturesValue']}
                rules={[{ required: true, message: i18next.t('ui.texturesValueIsRequired') }]}
              >
                <Input.TextArea
                  autoSize={{ minRows: 2, maxRows: 4 }}
                  placeholder={i18next.t('ui.base64ValueFromTheSessionProfile')}
                />
              </Form.Item>
              <Form.Item
                label={i18next.t('ui.texturesSignature')}
                name={[profile.name, 'texturesSignature']}
                rules={[{ required: true, message: i18next.t('ui.texturesSignatureIsRequired') }]}
              >
                <Input.TextArea
                  autoSize={{ minRows: 2, maxRows: 4 }}
                  placeholder={i18next.t('ui.base64SignatureFromTheSessionProfile')}
                />
              </Form.Item>
            </div>
          ))}
        </>
      )}
    </Form.List>
  );
}

const HEADER_CUSTOM_TCP_GROUPS = [
  {
    key: 'clients',
    get label() {
      return i18next.t('clients');
    },
    tooltip: undefined,
  },
  {
    key: 'servers',
    get label() {
      return i18next.t('ui.servers');
    },
    tooltip: undefined,
  },
  {
    key: 'errors',
    get label() {
      return i18next.t('ui.errors');
    },
    get tooltip() {
      return i18next.t('ui.sentInsteadWhenTheMatchingClientGroupFailsToMatchEGAnHTTP400');
    },
  },
] as const;

function HeaderCustomGroups({
  tcpFieldName,
  form,
  absoluteSettingsPath,
}: {
  tcpFieldName: number;
  form: FormInstance;
  absoluteSettingsPath: (string | number)[];
}) {
  const { t } = useTranslation();
  return (
    <>
      {HEADER_CUSTOM_TCP_GROUPS.map(({ key: groupKey, label, tooltip }) => (
        <Form.List key={groupKey} name={[tcpFieldName, 'settings', groupKey]}>
          {(groups, { add: addGroup, remove: removeGroup }) => (
            <>
              <Form.Item label={label} tooltip={tooltip}>
                <Button
                  type="primary"
                  size="small"
                  icon={<PlusOutlined />}
                  aria-label={t('add')}
                  onClick={() => addGroup([defaultClientServerItem()])}
                />
              </Form.Item>
              {groups.map((group, gi) => (
                <div key={group.key}>
                  <Divider style={{ margin: 0 }}>
                    {label} {i18next.t('pages.clients.group')} {gi + 1}
                    <DeleteOutlined
                      className="danger-icon"
                      role="button"
                      tabIndex={0}
                      aria-label={t('remove')}
                      onClick={() => removeGroup(group.name)}
                      onKeyDown={activateOnKey(() => removeGroup(group.name))}
                    />
                  </Divider>
                  <Form.List name={[group.name]}>
                    {(items, { add: addItem, remove: removeItem }) => (
                      <>
                        <Form.Item label={i18next.t('ui.items')}>
                          <Button
                            size="small"
                            icon={<PlusOutlined />}
                            aria-label={t('add')}
                            onClick={() => addItem(defaultClientServerItem())}
                          />
                        </Form.Item>
                        {items.map((item) => (
                          <ItemEditor
                            key={item.key}
                            fieldName={item.name}
                            form={form}
                            absoluteItemPath={[
                              ...absoluteSettingsPath,
                              groupKey,
                              group.name,
                              item.name,
                            ]}
                            delayMode="number"
                            allowVariables
                            onRemove={() => removeItem(item.name)}
                          />
                        ))}
                      </>
                    )}
                  </Form.List>
                </div>
              ))}
            </>
          )}
        </Form.List>
      ))}
    </>
  );
}

function UdpMasksList({
  base,
  form,
  isHysteria,
  isWireguard,
  network,
  allowUdpHop,
}: {
  base: (string | number)[];
  form: FormInstance;
  isHysteria: boolean;
  isWireguard: boolean;
  network: string;
  allowUdpHop: boolean;
}) {
  const { t } = useTranslation();
  return (
    <Form.List name={[...base, 'udp']}>
      {(fields, { add, remove }) => (
        <>
          <Form.Item label={i18next.t('ui.uDPMasks')}>
            <Button
              type="primary"
              size="small"
              icon={<PlusOutlined />}
              aria-label={t('add')}
              onClick={() => {
                const def = isHysteria || isWireguard ? 'salamander' : 'mkcp-legacy';
                add({ type: def, settings: defaultUdpMaskSettings(def) });
              }}
            />
          </Form.Item>
          {fields.map((field, mIdx) => (
            <UdpMaskItem
              key={field.key}
              fieldName={field.name}
              displayIndex={mIdx + 1}
              form={form}
              listPath={[...base, 'udp']}
              isHysteria={isHysteria}
              isWireguard={isWireguard}
              network={network}
              allowUdpHop={allowUdpHop}
              onRemove={() => remove(field.name)}
            />
          ))}
        </>
      )}
    </Form.List>
  );
}

function UdpMaskItem({
  fieldName,
  displayIndex,
  form,
  listPath,
  isHysteria,
  isWireguard,
  network,
  allowUdpHop,
  onRemove,
}: {
  fieldName: number;
  displayIndex: number;
  form: FormInstance;
  listPath: (string | number)[];
  isHysteria: boolean;
  isWireguard: boolean;
  network: string;
  allowUdpHop: boolean;
  onRemove: () => void;
}) {
  const absolutePath = [...listPath, fieldName];
  const { t } = useTranslation();

  const onTypeChange = (v: string) => {
    form.setFieldValue([...absolutePath, 'settings'], defaultUdpMaskSettings(v));
    if (network === 'kcp') {
      const kcpMtuPath = [...listPath.slice(0, -1), 'kcpSettings', 'mtu'];
      form.setFieldValue(kcpMtuPath, v === 'xdns' ? 900 : 1350);
    }
  };

  const udpHopOption = allowUdpHop ? [{ value: 'udphop', label: i18next.t('ui.uDPHop') }] : [];
  const options = isHysteria
    ? [{ value: 'salamander', label: 'Salamander (Hysteria2)' }, ...udpHopOption]
    : [
        // Salamander is the mask xray-core's own wireguard finalmask example
        // uses; it stays hysteria-only elsewhere to keep legacy parity.
        ...(isWireguard ? [{ value: 'salamander', label: 'Salamander' }] : []),
        { value: 'mkcp-legacy', label: i18next.t('ui.mKCPLegacy') },
        { value: 'xdns', label: 'xDNS' },
        { value: 'xicmp', label: 'xICMP' },
        { value: 'realm', label: 'Realm' },
        { value: 'header-custom', label: i18next.t('ui.headerCustom') },
        { value: 'noise', label: i18next.t('ui.noise') },
        { value: 'sudoku', label: 'Sudoku' },
        ...udpHopOption,
      ];

  return (
    <div>
      <Divider style={{ margin: 0 }}>
        {i18next.t('ui.uDPMask')} {displayIndex}
        <DeleteOutlined
          className="danger-icon"
          role="button"
          tabIndex={0}
          aria-label={t('remove')}
          onClick={onRemove}
          onKeyDown={activateOnKey(onRemove)}
        />
      </Divider>

      <Form.Item label={i18next.t('pages.inbounds.form.type')} name={[fieldName, 'type']}>
        <Select onChange={onTypeChange} options={options} />
      </Form.Item>

      <Form.Item
        noStyle
        shouldUpdate={(prev, curr) =>
          getDeep(prev, [...absolutePath, 'type']) !== getDeep(curr, [...absolutePath, 'type'])
        }
      >
        {({ getFieldValue }) => {
          const type = getFieldValue([...absolutePath, 'type']) as string | undefined;
          if (type === 'salamander') {
            return (
              <SalamanderUdpMaskSettings
                fieldName={fieldName}
                form={form}
                absolutePath={absolutePath}
              />
            );
          }
          if (type === 'mkcp-legacy') {
            return (
              <>
                <Form.Item label={i18next.t('ui.header')} name={[fieldName, 'settings', 'header']}>
                  <Select
                    options={[
                      { value: '', label: i18next.t('ui.originalAES128GCM') },
                      { value: 'dns', label: 'DNS' },
                      { value: 'dtls', label: 'DTLS 1.2' },
                      { value: 'srtp', label: 'SRTP' },
                      { value: 'utp', label: 'uTP' },
                      { value: 'wechat', label: i18next.t('ui.weChatVideo') },
                      { value: 'wireguard', label: 'WireGuard' },
                    ]}
                  />
                </Form.Item>
                <Form.Item
                  label={i18next.t('pages.xray.ruleForm.value')}
                  name={[fieldName, 'settings', 'value']}
                >
                  <Input placeholder={i18next.t('ui.passwordAES128GCMOrDomainDNSHeader')} />
                </Form.Item>
              </>
            );
          }
          if (type === 'xdns') {
            return <XdnsSettings udpFieldName={fieldName} />;
          }
          if (type === 'udphop') {
            return <UdpHopSettings udpFieldName={fieldName} />;
          }
          if (type === 'xicmp') {
            return (
              <>
                <Form.Item
                  label={i18next.t('ui.dgram')}
                  name={[fieldName, 'settings', 'dgram']}
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
                <Form.Item label={i18next.t('ui.iPs')} name={[fieldName, 'settings', 'ips']}>
                  <Select mode="tags" style={{ width: '100%' }} tokenSeparators={[',']} />
                </Form.Item>
              </>
            );
          }
          if (type === 'realm') {
            return (
              <>
                <Form.Item label="URL" name={[fieldName, 'settings', 'url']}>
                  <Input placeholder="realm://token@host:port/id" />
                </Form.Item>
                <Form.Item
                  label={i18next.t('ui.sTUNServers')}
                  name={[fieldName, 'settings', 'stunServers']}
                >
                  <Select
                    mode="tags"
                    style={{ width: '100%' }}
                    tokenSeparators={[',']}
                    placeholder="host:port"
                  />
                </Form.Item>
                <Form.Item label={i18next.t('ui.iPMode')} name={[fieldName, 'settings', 'ipMode']}>
                  <Select
                    allowClear
                    placeholder="dual"
                    options={[
                      { value: 'dual', label: i18next.t('ui.dual') },
                      { value: 'v4', label: 'IPv4' },
                      { value: 'v6', label: 'IPv6' },
                    ]}
                  />
                </Form.Item>
                <Form.Item
                  label={i18next.t('ui.portMappingUPnPNATPMP')}
                  name={[fieldName, 'settings', 'portMapping', 'enabled']}
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
                <Form.Item
                  label={i18next.t('ui.mappingTimeoutS')}
                  name={[fieldName, 'settings', 'portMapping', 'timeout']}
                >
                  <InputNumber min={0} placeholder={i18next.t('ui.10Default')} />
                </Form.Item>
                <Form.Item
                  label={i18next.t('ui.mappingLifetimeS')}
                  name={[fieldName, 'settings', 'portMapping', 'lifetime']}
                >
                  <InputNumber min={0} placeholder={i18next.t('ui.600Default')} />
                </Form.Item>
                <Divider plain style={{ margin: '8px 0' }}>
                  {i18next.t('ui.tLSOptional')}
                </Divider>
                <Form.Item
                  label={i18next.t('ui.serverName')}
                  name={[fieldName, 'settings', 'tlsConfig', 'serverName']}
                >
                  <Input placeholder={i18next.t('ui.sNIForTheRealmServerLeaveEmptyToSkipTLS')} />
                </Form.Item>
                <Form.Item label="ALPN" name={[fieldName, 'settings', 'tlsConfig', 'alpn']}>
                  <Select
                    mode="multiple"
                    style={{ width: '100%' }}
                    options={[
                      { value: 'h3', label: 'h3' },
                      { value: 'h2', label: 'h2' },
                      { value: 'http/1.1', label: 'http/1.1' },
                    ]}
                  />
                </Form.Item>
                <Form.Item
                  label={i18next.t('pages.inbounds.form.fingerprint')}
                  name={[fieldName, 'settings', 'tlsConfig', 'fingerprint']}
                >
                  <Select allowClear style={{ width: '100%' }} options={UTLS_FINGERPRINT_OPTIONS} />
                </Form.Item>
                <Form.Item
                  label={i18next.t('ui.allowInsecure')}
                  name={[fieldName, 'settings', 'tlsConfig', 'allowInsecure']}
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
              </>
            );
          }
          if (type === 'header-custom') {
            return (
              <UdpHeaderCustom
                udpFieldName={fieldName}
                form={form}
                absoluteSettingsPath={[...absolutePath, 'settings']}
              />
            );
          }
          if (type === 'sudoku') return <SudokuSettings fieldName={fieldName} />;
          if (type === 'noise') {
            return (
              <NoiseItems
                udpFieldName={fieldName}
                form={form}
                absoluteSettingsPath={[...absolutePath, 'settings']}
              />
            );
          }
          return null;
        }}
      </Form.Item>
    </div>
  );
}

function SalamanderUdpMaskSettings({
  fieldName,
  form,
  absolutePath,
}: {
  fieldName: number;
  form: FormInstance;
  absolutePath: (string | number)[];
}) {
  const { t } = useTranslation();
  const packetSizePath = [...absolutePath, 'settings', 'packetSize'];
  const packetSize = Form.useWatch(packetSizePath, { form, preserve: true });
  const mode = typeof packetSize === 'string' && packetSize.trim() !== '' ? 'gecko' : 'salamander';

  return (
    <>
      <Form.Item
        label={i18next.t('pages.inbounds.info.mode')}
        extra={
          mode === 'gecko'
            ? 'Salamander plus Gecko: splits each packet into random-padded fragments sized within the range below, defeating packet-length fingerprinting. Stored as Salamander with packetSize.'
            : 'Scrambles each packet into random-looking bytes.'
        }
      >
        <Select
          value={mode}
          onChange={(next) => {
            if (next === 'gecko') {
              const current = form.getFieldValue(packetSizePath);
              form.setFieldValue(
                packetSizePath,
                parseGeckoPacketSize(current)
                  ? current
                  : formatGeckoPacketSize(
                      DEFAULT_GECKO_PACKET_SIZE.min,
                      DEFAULT_GECKO_PACKET_SIZE.max,
                    ),
              );
            } else {
              form.setFieldValue(packetSizePath, undefined);
            }
          }}
          options={[
            { value: 'salamander', label: 'Salamander' },
            { value: 'gecko', label: i18next.t('ui.geckoExperimental') },
          ]}
        />
      </Form.Item>

      <Form.Item label={i18next.t('password')}>
        <Space.Compact block>
          <Form.Item
            name={[fieldName, 'settings', 'password']}
            noStyle
            rules={[{ validator: validateSalamanderPassword }]}
          >
            <Input
              placeholder={i18next.t('ui.obfuscationPassword')}
              style={{ width: 'calc(100% - 32px)' }}
            />
          </Form.Item>
          <Button
            icon={<ReloadOutlined />}
            aria-label={t('regenerate')}
            onClick={() =>
              form.setFieldValue(
                [...absolutePath, 'settings', 'password'],
                RandomUtil.randomLowerAndNum(16),
              )
            }
          />
        </Space.Compact>
      </Form.Item>

      {mode === 'gecko' && (
        <Form.Item
          label={i18next.t('ui.packetSize')}
          name={[fieldName, 'settings', 'packetSize']}
          rules={[{ validator: validateGeckoPacketSize }]}
          extra="Serialized as a string range, for example 512-1200."
        >
          <GeckoPacketSizeInput />
        </Form.Item>
      )}
    </>
  );
}

function GeckoPacketSizeInput({
  value,
  onChange,
}: {
  value?: string;
  onChange?: (value: string) => void;
}) {
  const { min, max } = splitGeckoPacketSize(value);

  return (
    <Space.Compact block>
      <InputNumber
        prefix="Min"
        min={GECKO_MIN_PACKET_SIZE}
        max={GECKO_MAX_PACKET_SIZE}
        precision={0}
        value={min}
        placeholder={String(DEFAULT_GECKO_PACKET_SIZE.min)}
        onChange={(next) => onChange?.(`${next ?? ''}-${max ?? ''}`)}
        style={{ width: '50%' }}
      />
      <InputNumber
        prefix="Max"
        min={GECKO_MIN_PACKET_SIZE}
        max={GECKO_MAX_PACKET_SIZE}
        precision={0}
        value={max}
        placeholder={String(DEFAULT_GECKO_PACKET_SIZE.max)}
        onChange={(next) => onChange?.(`${min ?? ''}-${next ?? ''}`)}
        style={{ width: '50%' }}
      />
    </Space.Compact>
  );
}

function UdpHeaderCustom({
  udpFieldName,
  form,
  absoluteSettingsPath,
}: {
  udpFieldName: number;
  form: FormInstance;
  absoluteSettingsPath: (string | number)[];
}) {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item
        label={i18next.t('pages.inbounds.info.mode')}
        name={[udpFieldName, 'settings', 'mode']}
        tooltip={i18next.t('ui.prefixPutsTheHeaderOnEveryPacketStandaloneExchangesItAsSeparateH')}
      >
        <Select
          allowClear
          placeholder={i18next.t('pages.clients.prefix')}
          options={[
            { value: 'prefix', label: i18next.t('pages.clients.prefix') },
            { value: 'standalone', label: i18next.t('ui.standalone') },
          ]}
        />
      </Form.Item>
      {(['client', 'server'] as const).map((groupKey) => (
        <Form.List key={groupKey} name={[udpFieldName, 'settings', groupKey]}>
          {(items, { add, remove }) => (
            <>
              <Form.Item label={groupKey === 'client' ? 'Client' : 'Server'}>
                <Button
                  type="primary"
                  size="small"
                  icon={<PlusOutlined />}
                  aria-label={t('add')}
                  onClick={() => add(defaultUdpClientServerItem())}
                />
              </Form.Item>
              {items.map((item, ci) => (
                <div key={item.key}>
                  <Divider style={{ margin: 0 }}>
                    {groupKey === 'client' ? 'Client' : 'Server'} {ci + 1}
                    <DeleteOutlined
                      className="danger-icon"
                      role="button"
                      tabIndex={0}
                      aria-label={t('remove')}
                      onClick={() => remove(item.name)}
                      onKeyDown={activateOnKey(() => remove(item.name))}
                    />
                  </Divider>
                  <ItemEditor
                    fieldName={item.name}
                    form={form}
                    absoluteItemPath={[...absoluteSettingsPath, groupKey, item.name]}
                    allowVariables
                    onRemove={() => remove(item.name)}
                  />
                </div>
              ))}
            </>
          )}
        </Form.List>
      ))}
    </>
  );
}

const XDNS_RECORD_TYPE_OPTIONS = [
  { value: 16, label: 'TXT' },
  { value: 1, label: 'A' },
  { value: 28, label: 'AAAA' },
  { value: 5, label: 'CNAME' },
];

// Every xdns key needs a registered field: the finalmask watch drops keys without one.
// Resolvers and extraPoll are read by clients only; a server keeps them for the share link.
function XdnsSettings({ udpFieldName }: { udpFieldName: number }) {
  const { t } = useTranslation();
  return (
    <>
      <Form.List name={[udpFieldName, 'settings', 'domains']}>
        {(domains, { add, remove }) => (
          <>
            <Form.Item label={i18next.t('pages.xray.dns.domains')}>
              <Button
                type="primary"
                size="small"
                icon={<PlusOutlined />}
                aria-label={t('add')}
                onClick={() => add({ names: [], edns0: XDNS_LEGACY_EDNS0 })}
              />
            </Form.Item>
            {domains.map((domain, di) => (
              <div key={domain.key}>
                <Divider style={{ margin: 0 }}>
                  {i18next.t('ui.domain')} {di + 1}
                  <DeleteOutlined
                    className="danger-icon"
                    role="button"
                    tabIndex={0}
                    aria-label={t('remove')}
                    onClick={() => remove(domain.name)}
                    onKeyDown={activateOnKey(() => remove(domain.name))}
                  />
                </Divider>
                <Form.Item label={i18next.t('ui.names')} name={[domain.name, 'names']}>
                  <Select
                    mode="tags"
                    style={{ width: '100%' }}
                    tokenSeparators={[',']}
                    placeholder="t.example.com"
                  />
                </Form.Item>
                <Form.Item label={i18next.t('ui.recordTypes')} name={[domain.name, 'types']}>
                  <Select
                    mode="multiple"
                    options={XDNS_RECORD_TYPE_OPTIONS}
                    placeholder={i18next.t('ui.tXTClientAllServer')}
                  />
                </Form.Item>
                <Form.Item label="EDNS0" name={[domain.name, 'edns0']}>
                  <InputNumber min={512} max={4096} placeholder="off" />
                </Form.Item>
                <Form.Item label={i18next.t('ui.lengthLimit')} name={[domain.name, 'lenLimit']}>
                  <InputNumber min={0} max={255} placeholder="255" />
                </Form.Item>
                <Form.Item label={i18next.t('ui.labelLimit')} name={[domain.name, 'labelLimit']}>
                  <InputNumber min={0} max={63} placeholder="63" />
                </Form.Item>
              </div>
            ))}
          </>
        )}
      </Form.List>
      <Form.List name={[udpFieldName, 'settings', 'resolvers']}>
        {(resolvers, { add, remove }) => (
          <>
            <Form.Item label={i18next.t('ui.resolversClient')}>
              <Button
                type="primary"
                size="small"
                icon={<PlusOutlined />}
                aria-label={t('add')}
                onClick={() => add({ addrs: [] })}
              />
            </Form.Item>
            {resolvers.map((resolver, ri) => (
              <Form.Item key={resolver.key} label={`Resolver ${ri + 1}`}>
                <Space.Compact block>
                  <Form.Item name={[resolver.name, 'addrs']} noStyle>
                    <Select
                      mode="tags"
                      style={{ width: '100%' }}
                      tokenSeparators={[',', ' ']}
                      placeholder="udp://8.8.8.8:53, tcp://1.1.1.1"
                    />
                  </Form.Item>
                  <Button
                    icon={<DeleteOutlined />}
                    aria-label={t('remove')}
                    onClick={() => remove(resolver.name)}
                  />
                </Space.Compact>
              </Form.Item>
            ))}
          </>
        )}
      </Form.List>
      <Form.Item
        label={i18next.t('ui.extraPollClient')}
        name={[udpFieldName, 'settings', 'extraPoll']}
      >
        <InputNumber min={0} max={3} placeholder="0" />
      </Form.Item>
    </>
  );
}

const UDP_HOP_MODES = [
  {
    value: 'intervalRemote',
    get label() {
      return i18next.t('ui.rotateServerPortInterval');
    },
  },
  {
    value: 'perConnRemote',
    get label() {
      return i18next.t('ui.randomServerPortPerConnection');
    },
  },
  {
    value: 'intervalLocal',
    get label() {
      return i18next.t('ui.rotateLocalPortInterval');
    },
  },
];

// xray-core lowercases each comma-separated mode, so a lowercase one (from an mport
// import) still maps onto its option here.
function udpHopModesFromWire(value: unknown): string[] {
  if (typeof value !== 'string') return [];
  return value
    .split(',')
    .map((mode) => mode.trim())
    .filter(Boolean)
    .map(
      (mode) =>
        UDP_HOP_MODES.find((m) => m.value.toLowerCase() === mode.toLowerCase())?.value ?? mode,
    );
}

function validateUdpHopInterval(_: unknown, value: unknown) {
  if (value == null || value === '') return Promise.resolve();
  const bounds = String(value)
    .split('-')
    .map((v) => Number(v.trim()));
  const valid = bounds.length <= 2 && bounds.every((v) => Number.isInteger(v) && v >= 5);
  return valid ? Promise.resolve() : Promise.reject(new Error('seconds ≥ 5, e.g. 30 or 5-10'));
}

// Client-only hopping (xray-core 26.9.9); every key needs a registered field because the
// finalmask watch drops keys without one.
function UdpHopSettings({ udpFieldName }: { udpFieldName: number }) {
  return (
    <>
      <Form.Item
        label={i18next.t('pages.inbounds.info.mode')}
        name={[udpFieldName, 'settings', 'mode']}
        getValueProps={(value) => ({ value: udpHopModesFromWire(value) })}
        normalize={(value: string[]) => value.join(',')}
        rules={[{ required: true, message: 'pick at least one mode' }]}
      >
        <Select mode="multiple" options={UDP_HOP_MODES} />
      </Form.Item>
      <Form.Item
        label={i18next.t('ui.intervalS')}
        name={[udpFieldName, 'settings', 'interval']}
        rules={[{ validator: validateUdpHopInterval }]}
      >
        <Input placeholder="30 or 5-10" />
      </Form.Item>
      <Form.Item
        label={i18next.t('ui.remotePorts')}
        name={[udpFieldName, 'settings', 'remotePorts']}
      >
        <Input placeholder="20000-50000" />
      </Form.Item>
      <Form.Item label={i18next.t('ui.remoteIPs')} name={[udpFieldName, 'settings', 'remoteIPs']}>
        <Select mode="tags" style={{ width: '100%' }} tokenSeparators={[',', ' ']} />
      </Form.Item>
    </>
  );
}

// xray-core reads reset as an Int32Range: a plain number or a "min-max" string.
function validateNoiseReset(_: unknown, value: unknown) {
  if (value == null || value === '') return Promise.resolve();
  const bounds = String(value)
    .split('-')
    .map((v) => Number(v.trim()));
  const valid =
    bounds.length <= 2 &&
    bounds.every((v) => Number.isInteger(v) && v >= 0) &&
    bounds[0] <= bounds[bounds.length - 1];
  return valid ? Promise.resolve() : Promise.reject(new Error('seconds, e.g. 0 or 60-120'));
}

function NoiseItems({
  udpFieldName,
  form,
  absoluteSettingsPath,
}: {
  udpFieldName: number;
  form: FormInstance;
  absoluteSettingsPath: (string | number)[];
}) {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item
        label={i18next.t('reset')}
        name={[udpFieldName, 'settings', 'reset']}
        tooltip={i18next.t('ui.secondsBeforeTheNoiseIsSentToADestinationAgain0SendsItOnce')}
        rules={[{ validator: validateNoiseReset }]}
      >
        <Input placeholder="0 or 60-120" />
      </Form.Item>
      <Form.List name={[udpFieldName, 'settings', 'noise']}>
        {(items, { add, remove }) => (
          <>
            <Form.Item label={i18next.t('ui.noise')}>
              <Button
                type="primary"
                size="small"
                icon={<PlusOutlined />}
                aria-label={t('add')}
                onClick={() => add(defaultNoiseItem())}
              />
            </Form.Item>
            {items.map((item, ni) => (
              <div key={item.key}>
                <Divider style={{ margin: 0 }}>
                  {i18next.t('ui.noise')} {ni + 1}
                  <DeleteOutlined
                    className="danger-icon"
                    role="button"
                    tabIndex={0}
                    aria-label={t('remove')}
                    onClick={() => remove(item.name)}
                    onKeyDown={activateOnKey(() => remove(item.name))}
                  />
                </Divider>
                <ItemEditor
                  fieldName={item.name}
                  form={form}
                  absoluteItemPath={[...absoluteSettingsPath, 'noise', item.name]}
                  delayMode="string"
                  onRemove={() => remove(item.name)}
                />
              </div>
            ))}
          </>
        )}
      </Form.List>
    </>
  );
}

// xray-core 26.6 header-custom: an item is exactly one of raw bytes, a reused variable
// or a transform ("exactly one item kind must be set"); capture works with any of them.
type ItemKind = 'bytes' | 'reuse' | 'transform';

const ITEM_KIND_OPTIONS: { value: ItemKind; label: string }[] = [
  {
    value: 'bytes',
    get label() {
      return i18next.t('ui.bytes');
    },
  },
  {
    value: 'reuse',
    get label() {
      return i18next.t('ui.reuseVariable');
    },
  },
  {
    value: 'transform',
    get label() {
      return i18next.t('ui.transform');
    },
  },
];

function itemKindOf(item: unknown): ItemKind {
  const fields = (item ?? {}) as Record<string, unknown>;
  if (typeof fields.reuse === 'string') return 'reuse';
  if (fields.transform != null) return 'transform';
  return 'bytes';
}

function validateCustomVarName(_: unknown, value: unknown) {
  if (value == null || value === '') return Promise.resolve();
  return /^[A-Za-z_][A-Za-z0-9_]*$/.test(String(value))
    ? Promise.resolve()
    : Promise.reject(new Error('letters, digits and _, not starting with a digit'));
}

// Hands the form only a parsed JSON object, so half-typed text never reaches the config.
function TransformEditor({
  value,
  onChange,
}: {
  value?: Record<string, unknown>;
  onChange?: (next: Record<string, unknown> | undefined) => void;
}) {
  const [text, setText] = useState(() => (value ? JSON.stringify(value) : ''));
  const [invalid, setInvalid] = useState(false);
  return (
    <>
      <Input.TextArea
        value={text}
        autoSize={{ minRows: 2, maxRows: 8 }}
        placeholder='{"op": "concat", "args": [{"reuse": "nonce"}]}'
        onChange={(e) => {
          const next = e.target.value;
          setText(next);
          if (next.trim() === '') {
            setInvalid(false);
            onChange?.(undefined);
            return;
          }
          try {
            const parsed: unknown = JSON.parse(next);
            const isObject = !!parsed && typeof parsed === 'object' && !Array.isArray(parsed);
            setInvalid(!isObject);
            if (isObject) onChange?.(parsed as Record<string, unknown>);
          } catch {
            setInvalid(true);
          }
        }}
      />
      {invalid && (
        <Typography.Text type="danger">
          {i18next.t('pages.inbounds.form.xdrive.templateInvalid')}
        </Typography.Text>
      )}
    </>
  );
}

function ItemEditor({
  fieldName,
  form,
  absoluteItemPath,
  delayMode,
  allowVariables = false,
  onRemove: _onRemove,
}: {
  fieldName: number;
  form: FormInstance;
  absoluteItemPath: (string | number)[];
  delayMode?: 'number' | 'string';
  allowVariables?: boolean;
  onRemove?: () => void;
}) {
  const { t } = useTranslation();
  const [kind, setKind] = useState<ItemKind>(() =>
    allowVariables ? itemKindOf(form.getFieldValue(absoluteItemPath)) : 'bytes',
  );
  const onKindChange = (next: ItemKind) => {
    setKind(next);
    for (const key of ['type', 'packet', 'rand', 'randRange', 'reuse', 'transform']) {
      form.setFieldValue([...absoluteItemPath, key], undefined);
    }
    if (next === 'bytes') {
      form.setFieldValue([...absoluteItemPath, 'type'], 'array');
      form.setFieldValue([...absoluteItemPath, 'rand'], 0);
    }
  };
  /**
   * Switching to `array` clears the packet instead of emptying it to `[]`:
   * that branch is rand-driven, and xray-core counts even an empty array as a
   * packet, rejecting an item that carries both a packet and a rand. That
   * error fails the whole config, so one such item keeps every inbound offline.
   */
  const onTypeChange = (v: string) => {
    if (v === 'base64') {
      form.setFieldValue([...absoluteItemPath, 'packet'], RandomUtil.randomBase64());
    } else if (v === 'array') {
      form.setFieldValue([...absoluteItemPath, 'rand'], delayMode === 'string' ? '1-8192' : 0);
      form.setFieldValue([...absoluteItemPath, 'packet'], undefined);
    } else {
      form.setFieldValue([...absoluteItemPath, 'packet'], '');
    }
  };

  return (
    <>
      {allowVariables && (
        <Form.Item label={i18next.t('ui.kind')}>
          <Select value={kind} onChange={onKindChange} options={ITEM_KIND_OPTIONS} />
        </Form.Item>
      )}
      {kind === 'bytes' && (
        <Form.Item label={i18next.t('pages.inbounds.form.type')} name={[fieldName, 'type']}>
          <Select
            onChange={onTypeChange}
            options={[
              { value: 'array', label: i18next.t('ui.array') },
              { value: 'str', label: i18next.t('ui.string') },
              { value: 'hex', label: 'Hex' },
              { value: 'base64', label: 'Base64' },
              // Only the noise mask parses tag expressions (xray-core 26.9.30, #6862).
              ...(delayMode === 'string'
                ? [{ value: 'exp', label: i18next.t('ui.expression') }]
                : []),
            ]}
          />
        </Form.Item>
      )}

      {delayMode === 'number' && (
        <Form.Item
          label={i18next.t('pages.settings.subFormats.delayMs')}
          name={[fieldName, 'delay']}
        >
          <InputNumber min={0} />
        </Form.Item>
      )}
      {delayMode === 'string' && (
        <Form.Item label={i18next.t('discord.fields.delay')} name={[fieldName, 'delay']}>
          <Input placeholder="10-20" />
        </Form.Item>
      )}

      {kind === 'reuse' && (
        <Form.Item
          label={i18next.t('ui.reuse')}
          name={[fieldName, 'reuse']}
          tooltip={i18next.t('ui.writesTheBytesAnEarlierItemCapturedUnderThisName')}
          rules={[
            { required: true, message: 'name the captured variable' },
            { validator: validateCustomVarName },
          ]}
        >
          <Input placeholder="variable name" />
        </Form.Item>
      )}
      {kind === 'transform' && (
        <Form.Item
          label={i18next.t('ui.transform')}
          name={[fieldName, 'transform']}
          tooltip={i18next.t('ui.anExpressionTreeEGOpConcatArgsTypeStrBytesOKReuseNonce')}
          rules={[{ required: true, message: 'enter the transform expression' }]}
        >
          <TransformEditor />
        </Form.Item>
      )}

      <Form.Item
        noStyle
        shouldUpdate={(prev, curr) =>
          getDeep(prev, [...absoluteItemPath, 'type']) !==
          getDeep(curr, [...absoluteItemPath, 'type'])
        }
      >
        {({ getFieldValue }) => {
          if (kind !== 'bytes') return null;
          const type = getFieldValue([...absoluteItemPath, 'type']) as string | undefined;
          if (type === 'array') {
            return (
              <>
                <Form.Item label={i18next.t('ui.rand')} name={[fieldName, 'rand']}>
                  {delayMode === 'string' ? (
                    <Input placeholder="0 or 1-8192" />
                  ) : (
                    <InputNumber min={0} />
                  )}
                </Form.Item>
                {/* Cleared must become undefined, not '': xray parses an
                    explicit "" as the range 0-0 (all-zero fill bytes), while
                    an omitted randRange falls back to the 0-255 default. */}
                <Form.Item
                  label={i18next.t('ui.randRange')}
                  name={[fieldName, 'randRange']}
                  normalize={(v) => (v === '' ? undefined : v)}
                  rules={[{ validator: validateRandRange }]}
                >
                  <Input placeholder="0-255" />
                </Form.Item>
              </>
            );
          }
          if (type === 'base64') {
            return (
              <Form.Item label={i18next.t('pages.settings.subFormats.packet')}>
                <Space.Compact block>
                  <Form.Item name={[fieldName, 'packet']} noStyle>
                    <Input placeholder="binary data" style={{ width: 'calc(100% - 32px)' }} />
                  </Form.Item>
                  <Button
                    icon={<ReloadOutlined />}
                    aria-label={t('regenerate')}
                    onClick={() =>
                      form.setFieldValue([...absoluteItemPath, 'packet'], RandomUtil.randomBase64())
                    }
                  />
                </Space.Compact>
              </Form.Item>
            );
          }
          return (
            <Form.Item
              label={i18next.t('pages.settings.subFormats.packet')}
              name={[fieldName, 'packet']}
            >
              <Input placeholder={type === 'exp' ? '<b 0d0a0d0a><t><rc 20-40>' : 'binary data'} />
            </Form.Item>
          );
        }}
      </Form.Item>

      {allowVariables && (
        <Form.Item
          label={i18next.t('ui.capture')}
          name={[fieldName, 'capture']}
          tooltip={i18next.t('ui.savesTheBytesThisItemWritesOrReadsUnderANameLaterItemsCanReuse')}
          normalize={(v) => (v === '' ? undefined : v)}
          rules={[{ validator: validateCustomVarName }]}
        >
          <Input placeholder="variable name" />
        </Form.Item>
      )}
    </>
  );
}

function QuicParamsForm({
  base,
  form,
  showUdpHop,
}: {
  base: (string | number)[];
  form: FormInstance;
  showUdpHop: boolean;
}) {
  const congestion = Form.useWatch([...base, 'congestion'], form) as string | undefined;
  const udpHop = Form.useWatch([...base, 'udpHop'], { form, preserve: true }) as
    | Record<string, unknown>
    | undefined;
  const hasUdpHop = udpHop != null;

  return (
    <>
      <Form.Item label={i18next.t('ui.congestion')} name={[...base, 'congestion']}>
        <Select
          options={[
            { value: 'reno', label: 'Reno' },
            { value: 'bbr', label: 'BBR' },
            { value: 'brutal', label: 'Brutal' },
            { value: 'force-brutal', label: i18next.t('ui.forceBrutal') },
          ]}
        />
      </Form.Item>
      {congestion === 'bbr' && (
        <Form.Item label={i18next.t('ui.bBRProfile')} name={[...base, 'bbrProfile']}>
          <Select
            allowClear
            placeholder="standard"
            options={[
              { value: 'conservative', label: i18next.t('ui.conservative') },
              { value: 'standard', label: i18next.t('pages.clients.qrStandard') },
              { value: 'aggressive', label: i18next.t('ui.aggressive') },
            ]}
          />
        </Form.Item>
      )}
      <Form.Item
        label={i18next.t('pages.index.logLevelDebug')}
        name={[...base, 'debug']}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>

      {(congestion === 'brutal' || congestion === 'force-brutal') && (
        <>
          <Form.Item label={i18next.t('ui.brutalUp')} name={[...base, 'brutalUp']}>
            <Input placeholder="e.g. 60 mbps" />
          </Form.Item>
          <Form.Item label={i18next.t('ui.brutalDown')} name={[...base, 'brutalDown']}>
            <Input placeholder="e.g. 100 mbps" />
          </Form.Item>
          <Form.Item
            label={i18next.t('ui.brutalDisableLossComp')}
            name={[...base, 'brutalDisableLossCompensation']}
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
        </>
      )}

      {/* Advertised to clients (link mport, JSON-subscription udphop mask); the server
          itself listens on its own port. */}
      {showUdpHop && (
        <>
          <Form.Item
            label={i18next.t('ui.uDPHop')}
            tooltip={i18next.t('ui.advertisedToClientsAsAHopRangeTheServerListensOnItsOwnPortOnly')}
          >
            <Switch
              checked={hasUdpHop}
              onChange={(v) => {
                form.setFieldValue([...base, 'udpHop'], v ? defaultUdpHop() : undefined);
              }}
            />
          </Form.Item>
          {hasUdpHop && (
            <>
              <Form.Item label={i18next.t('ui.hopPorts')} name={[...base, 'udpHop', 'ports']}>
                <Input placeholder="e.g. 20000-50000" />
              </Form.Item>
              <Form.Item
                label={i18next.t('ui.hopIntervalS')}
                name={[...base, 'udpHop', 'interval']}
              >
                <Input placeholder="e.g. 5-10" />
              </Form.Item>
            </>
          )}
        </>
      )}

      <Form.Item label={i18next.t('ui.maxIdleTimeoutS')} name={[...base, 'maxIdleTimeout']}>
        <InputNumber min={4} max={120} />
      </Form.Item>
      <Form.Item label={i18next.t('ui.keepAlivePeriodS')} name={[...base, 'keepAlivePeriod']}>
        <InputNumber min={2} max={60} />
      </Form.Item>
      <Form.Item
        label={i18next.t('ui.disablePathMTUDis')}
        name={[...base, 'disablePathMTUDiscovery']}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      <Form.Item
        label={i18next.t('ui.disableChromeParrot')}
        name={[...base, 'disableChromeParrot']}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      <Form.Item
        label={i18next.t('ui.disableGSO')}
        name={[...base, 'disableGSO']}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      <Form.Item
        label={i18next.t('ui.disableStatelessReset')}
        name={[...base, 'disableStatelessReset']}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>

      <Form.Item label={i18next.t('ui.maxIncomingStreams')} name={[...base, 'maxIncomingStreams']}>
        <InputNumber min={8} placeholder={i18next.t('ui.1024Default')} />
      </Form.Item>
      <Form.Item
        label={i18next.t('ui.initStreamWindow')}
        name={[...base, 'initStreamReceiveWindow']}
      >
        <InputNumber min={16384} placeholder={i18next.t('ui.8388608Default')} />
      </Form.Item>
      <Form.Item label={i18next.t('ui.maxStreamWindow')} name={[...base, 'maxStreamReceiveWindow']}>
        <InputNumber min={16384} placeholder={i18next.t('ui.8388608Default')} />
      </Form.Item>
      <Form.Item
        label={i18next.t('ui.initConnWindow')}
        name={[...base, 'initConnectionReceiveWindow']}
      >
        <InputNumber min={16384} placeholder={i18next.t('ui.20971520Default')} />
      </Form.Item>
      <Form.Item
        label={i18next.t('ui.maxConnWindow')}
        name={[...base, 'maxConnectionReceiveWindow']}
      >
        <InputNumber min={16384} placeholder={i18next.t('ui.20971520Default')} />
      </Form.Item>
    </>
  );
}
