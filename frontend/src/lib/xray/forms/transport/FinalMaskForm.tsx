import { useEffect, useRef } from 'react';
import { AutoComplete, Button, Divider, Form, Input, InputNumber, Select, Space, Switch } from 'antd';
import { DeleteOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import type { FormInstance } from 'antd/es/form';
import type { NamePath } from 'antd/es/form/interface';

import { RandomUtil } from '@/utils';
import { OutboundProtocols, UTLS_FINGERPRINT } from '@/schemas/primitives';

const UTLS_FINGERPRINT_OPTIONS = Object.values(UTLS_FINGERPRINT).map((value) => ({ value, label: value }));

export interface FinalMaskFormProps {
  name: NamePath;
  network: string;
  protocol: string;
  form: FormInstance;
  // When true, all sections (TCP / UDP / QUIC) are shown regardless of
  // network/protocol. Used by the global sub-JSON finalmask editor where
  // the masks apply to every stream rather than one specific transport.
  showAll?: boolean;
}

const TCP_NETWORKS = ['raw', 'tcp', 'httpupgrade', 'ws', 'grpc', 'xhttp'];
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
    !Number.isSafeInteger(min) || !Number.isSafeInteger(max)
    || min < GECKO_MIN_PACKET_SIZE || max < min || max > GECKO_MAX_PACKET_SIZE
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
  return Promise.reject(new Error(
    `Use a range like 512-1200 (${GECKO_MIN_PACKET_SIZE}-${GECKO_MAX_PACKET_SIZE}, max ≥ min)`,
  ));
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
      return {
        password: '', ascii: '', customTable: '', customTables: [],
        paddingMin: 0, paddingMax: 0,
      };
    case 'header-custom':
      return { clients: [], servers: [] };
    default:
      return {};
  }
}

// xray-core #6334 replaced a fragment mask's single `length`/`delay` ranges
// with `lengths`/`delays` arrays (the singular keys remain in core only as a
// fallback). Lift any legacy singular value into a one-element array so the
// list UI shows it, and drop the singular key so we never emit both.
function migrateFragmentSettings(settings: Record<string, unknown>): { next: Record<string, unknown>; changed: boolean } {
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
    rand: '1-8192', randRange: '0-255', type: 'array', packet: [], delay: '10-20',
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

export default function FinalMaskForm({ name, network, protocol, form, showAll = false }: FinalMaskFormProps) {
  const base = asPath(name);

  // Migrate legacy single-range fragment masks to the per-segment arrays once
  // on mount so configs saved before #6334 render in the list UI.
  const migratedRef = useRef(false);
  useEffect(() => {
    if (migratedRef.current) return;
    migratedRef.current = true;
    const tcp = form.getFieldValue([...base, 'tcp']);
    if (!Array.isArray(tcp)) return;
    let anyChanged = false;
    const next = tcp.map((mask) => {
      if (!mask || typeof mask !== 'object') return mask;
      const m = mask as Record<string, unknown>;
      if (m.type !== 'fragment' || !m.settings || typeof m.settings !== 'object') return mask;
      const { next: migrated, changed } = migrateFragmentSettings(m.settings as Record<string, unknown>);
      if (!changed) return mask;
      anyChanged = true;
      return { ...m, settings: migrated };
    });
    if (anyChanged) form.setFieldValue([...base, 'tcp'], next);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const isHysteria = protocol === OutboundProtocols.Hysteria || protocol === 'hysteria';
  // Wireguard carries no user-selectable transport (always a UDP listener/
  // dialer), so only the UDP mask section applies — TCP masks would never
  // wrap anything even though the leftover network value may be 'tcp'.
  const isWireguard = protocol === 'wireguard';
  const showTcp = showAll || (!isWireguard && TCP_NETWORKS.includes(network));
  const showUdp = showAll || isHysteria || isWireguard || network === 'kcp';
  const showQuic = showAll || isHysteria || network === 'xhttp';
  const quicParams = Form.useWatch([...base, 'quicParams'], { form, preserve: true });
  const hasQuicParams = quicParams != null;

  if (!showTcp && !showUdp && !showQuic) return null;

  return (
    <>
      {showTcp && <TcpMasksList base={base} form={form} />}
      {showUdp && <UdpMasksList base={base} form={form} isHysteria={isHysteria} isWireguard={isWireguard} network={network} />}
      {showQuic && (
        <>
          <Form.Item label="QUIC Params">
            <Switch
              checked={hasQuicParams}
              onChange={(v) => {
                form.setFieldValue([...base, 'quicParams'], v ? defaultQuicParams() : undefined);
              }}
            />
          </Form.Item>
          {hasQuicParams && <QuicParamsForm base={[...base, 'quicParams']} form={form} />}
        </>
      )}
    </>
  );
}

function TcpMasksList({ base, form }: { base: (string | number)[]; form: FormInstance }) {
  return (
    <Form.List name={[...base, 'tcp']}>
      {(fields, { add, remove }) => (
        <>
          <Form.Item label="TCP Masks">
            <Button
              type="primary"
              size="small"
              icon={<PlusOutlined />}
              onClick={() => add({ type: 'fragment', settings: defaultTcpMaskSettings('fragment') })}
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
  fieldName, displayIndex, form, listPath, onRemove,
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

  return (
    <div>
      <Divider style={{ margin: 0 }}>
        TCP Mask {displayIndex}
        <DeleteOutlined className="danger-icon" onClick={onRemove} />
      </Divider>

      <Form.Item label="Type" name={[fieldName, 'type']}>
        <Select
          onChange={(v) =>
            form.setFieldValue([...absolutePath, 'settings'], defaultTcpMaskSettings(v))
          }
          options={[
            { value: 'fragment', label: 'Fragment' },
            { value: 'header-custom', label: 'Header Custom' },
            { value: 'sudoku', label: 'Sudoku' },
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
                  label="Packets"
                  name={[fieldName, 'settings', 'packets']}
                  rules={[{ validator: validateFragmentPackets }]}
                >
                  <AutoComplete
                    options={[
                      { value: 'tlshello', label: 'tlshello' },
                      { value: '1-3', label: '1-3' },
                      { value: '1-5', label: '1-5' },
                    ]}
                    placeholder="tlshello or n-m, e.g. 1-3"
                  />
                </Form.Item>
                <FragmentRangeList
                  listName={[fieldName, 'settings', 'lengths']}
                  label="Lengths"
                  placeholder="e.g. 100-200"
                  minItems={1}
                  validator={validateFragmentLength}
                />
                <FragmentRangeList
                  listName={[fieldName, 'settings', 'delays']}
                  label="Delays"
                  placeholder="e.g. 10-20 or 0"
                  validator={validateFragmentDelayEntry}
                />
                <Form.Item label="Max Split" name={[fieldName, 'settings', 'maxSplit']}>
                  <Input />
                </Form.Item>
              </>
            );
          }
          if (type === 'sudoku') {
            return (
              <>
                <Form.Item label="Password" name={[fieldName, 'settings', 'password']}><Input /></Form.Item>
                <Form.Item label="ASCII" name={[fieldName, 'settings', 'ascii']}><Input /></Form.Item>
                <Form.Item label="Custom Table" name={[fieldName, 'settings', 'customTable']}><Input /></Form.Item>
                <Form.Item label="Custom Tables" name={[fieldName, 'settings', 'customTables']}>
                  <Select mode="tags" style={{ width: '100%' }} tokenSeparators={[',']} />
                </Form.Item>
                <Form.Item label="Padding Min" name={[fieldName, 'settings', 'paddingMin']}>
                  <InputNumber min={0} />
                </Form.Item>
                <Form.Item label="Padding Max" name={[fieldName, 'settings', 'paddingMax']}>
                  <InputNumber min={0} />
                </Form.Item>
              </>
            );
          }
          if (type === 'header-custom') {
            return (
              <HeaderCustomGroups
                tcpFieldName={fieldName}
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
    return Promise.reject(new Error('Length is required — xray rejects a fragment mask whose LengthMin is 0'));
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
    return Promise.reject(new Error("Delay is required — remove the row if you don't want a delay"));
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
  listName, label, placeholder, validator, minItems = 0,
}: {
  listName: (string | number)[];
  label: string;
  placeholder: string;
  validator?: (rule: unknown, value: unknown) => Promise<void>;
  minItems?: number;
}) {
  return (
    <Form.List name={listName}>
      {(fields, { add, remove }) => (
        <>
          <Form.Item label={label}>
            <Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => add('')} />
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
                addonAfter={fields.length > minItems
                  ? <DeleteOutlined className="danger-icon" onClick={() => remove(field.name)} />
                  : null}
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

function HeaderCustomGroups({
  tcpFieldName, form, absoluteSettingsPath,
}: {
  tcpFieldName: number;
  form: FormInstance;
  absoluteSettingsPath: (string | number)[];
}) {
  return (
    <>
      {(['clients', 'servers'] as const).map((groupKey) => (
        <Form.List key={groupKey} name={[tcpFieldName, 'settings', groupKey]}>
          {(groups, { add: addGroup, remove: removeGroup }) => (
            <>
              <Form.Item label={groupKey === 'clients' ? 'Clients' : 'Servers'}>
                <Button
                  type="primary"
                  size="small"
                  icon={<PlusOutlined />}
                  onClick={() => addGroup([defaultClientServerItem()])}
                />
              </Form.Item>
              {groups.map((group, gi) => (
                <div key={group.key}>
                  <Divider style={{ margin: 0 }}>
                    {groupKey === 'clients' ? 'Clients' : 'Servers'} Group {gi + 1}
                    <DeleteOutlined className="danger-icon" onClick={() => removeGroup(group.name)} />
                  </Divider>
                  <Form.List name={[group.name]}>
                    {(items, { add: addItem, remove: removeItem }) => (
                      <>
                        <Form.Item label="Items">
                          <Button
                            size="small"
                            icon={<PlusOutlined />}
                            onClick={() => addItem(defaultClientServerItem())}
                          />
                        </Form.Item>
                        {items.map((item) => (
                          <ItemEditor
                            key={item.key}
                            fieldName={item.name}
                            form={form}
                            absoluteItemPath={[...absoluteSettingsPath, groupKey, group.name, item.name]}
                            delayMode="number"
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
  base, form, isHysteria, isWireguard, network,
}: { base: (string | number)[]; form: FormInstance; isHysteria: boolean; isWireguard: boolean; network: string }) {
  return (
    <Form.List name={[...base, 'udp']}>
      {(fields, { add, remove }) => (
        <>
          <Form.Item label="UDP Masks">
            <Button
              type="primary"
              size="small"
              icon={<PlusOutlined />}
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
              onRemove={() => remove(field.name)}
            />
          ))}
        </>
      )}
    </Form.List>
  );
}

function UdpMaskItem({
  fieldName, displayIndex, form, listPath, isHysteria, isWireguard, network, onRemove,
}: {
  fieldName: number;
  displayIndex: number;
  form: FormInstance;
  listPath: (string | number)[];
  isHysteria: boolean;
  isWireguard: boolean;
  network: string;
  onRemove: () => void;
}) {
  const absolutePath = [...listPath, fieldName];

  const onTypeChange = (v: string) => {
    form.setFieldValue([...absolutePath, 'settings'], defaultUdpMaskSettings(v));
    if (network === 'kcp') {
      const kcpMtuPath = [...listPath.slice(0, -1), 'kcpSettings', 'mtu'];
      form.setFieldValue(kcpMtuPath, v === 'xdns' ? 900 : 1350);
    }
  };

  const options = isHysteria
    ? [{ value: 'salamander', label: 'Salamander (Hysteria2)' }]
    : [
      // Salamander is the mask xray-core's own wireguard finalmask example
      // uses; it stays hysteria-only elsewhere to keep legacy parity.
      ...(isWireguard ? [{ value: 'salamander', label: 'Salamander' }] : []),
      { value: 'mkcp-legacy', label: 'mKCP Legacy' },
      { value: 'xdns', label: 'xDNS' },
      { value: 'xicmp', label: 'xICMP' },
      { value: 'realm', label: 'Realm' },
      { value: 'header-custom', label: 'Header Custom' },
      { value: 'noise', label: 'Noise' },
    ];

  return (
    <div>
      <Divider style={{ margin: 0 }}>
        UDP Mask {displayIndex}
        <DeleteOutlined className="danger-icon" onClick={onRemove} />
      </Divider>

      <Form.Item label="Type" name={[fieldName, 'type']}>
        <Select onChange={onTypeChange} options={options} />
      </Form.Item>

      <Form.Item
        noStyle
        shouldUpdate={(prev, curr) => getDeep(prev, [...absolutePath, 'type']) !== getDeep(curr, [...absolutePath, 'type'])}
      >
        {({ getFieldValue }) => {
          const type = getFieldValue([...absolutePath, 'type']) as string | undefined;
          if (type === 'salamander') {
            return <SalamanderUdpMaskSettings fieldName={fieldName} form={form} absolutePath={absolutePath} />;
          }
          if (type === 'mkcp-legacy') {
            return (
              <>
                <Form.Item label="Header" name={[fieldName, 'settings', 'header']}>
                  <Select
                    options={[
                      { value: '', label: 'Original / AES-128-GCM' },
                      { value: 'dns', label: 'DNS' },
                      { value: 'dtls', label: 'DTLS 1.2' },
                      { value: 'srtp', label: 'SRTP' },
                      { value: 'utp', label: 'uTP' },
                      { value: 'wechat', label: 'WeChat Video' },
                      { value: 'wireguard', label: 'WireGuard' },
                    ]}
                  />
                </Form.Item>
                <Form.Item label="Value" name={[fieldName, 'settings', 'value']}>
                  <Input placeholder="password (AES-128-GCM) or domain (DNS header)" />
                </Form.Item>
              </>
            );
          }
          if (type === 'xdns') {
            return (
              <Form.Item label="Domains" name={[fieldName, 'settings', 'domains']}>
                <Select mode="tags" style={{ width: '100%' }} tokenSeparators={[',']} />
              </Form.Item>
            );
          }
          if (type === 'xicmp') {
            return (
              <>
                <Form.Item label="Dgram" name={[fieldName, 'settings', 'dgram']} valuePropName="checked">
                  <Switch />
                </Form.Item>
                <Form.Item label="IPs" name={[fieldName, 'settings', 'ips']}>
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
                <Form.Item label="STUN Servers" name={[fieldName, 'settings', 'stunServers']}>
                  <Select mode="tags" style={{ width: '100%' }} tokenSeparators={[',']} placeholder="host:port" />
                </Form.Item>
                <Divider plain style={{ margin: '8px 0' }}>TLS (optional)</Divider>
                <Form.Item label="Server Name" name={[fieldName, 'settings', 'tlsConfig', 'serverName']}>
                  <Input placeholder="SNI for the realm server (leave empty to skip TLS)" />
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
                <Form.Item label="Fingerprint" name={[fieldName, 'settings', 'tlsConfig', 'fingerprint']}>
                  <Select
                    allowClear
                    style={{ width: '100%' }}
                    options={UTLS_FINGERPRINT_OPTIONS}
                  />
                </Form.Item>
                <Form.Item
                  label="Allow Insecure"
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
  fieldName, form, absolutePath,
}: {
  fieldName: number;
  form: FormInstance;
  absolutePath: (string | number)[];
}) {
  const packetSizePath = [...absolutePath, 'settings', 'packetSize'];
  const packetSize = Form.useWatch(packetSizePath, { form, preserve: true });
  const mode = typeof packetSize === 'string' && packetSize.trim() !== '' ? 'gecko' : 'salamander';

  return (
    <>
      <Form.Item
        label="Mode"
        extra={mode === 'gecko'
          ? 'Salamander plus Gecko: splits each packet into random-padded fragments sized within the range below, defeating packet-length fingerprinting. Stored as Salamander with packetSize.'
          : 'Scrambles each packet into random-looking bytes.'}
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
                  : formatGeckoPacketSize(DEFAULT_GECKO_PACKET_SIZE.min, DEFAULT_GECKO_PACKET_SIZE.max),
              );
            } else {
              form.setFieldValue(packetSizePath, undefined);
            }
          }}
          options={[
            { value: 'salamander', label: 'Salamander' },
            { value: 'gecko', label: 'Gecko experimental' },
          ]}
        />
      </Form.Item>

      <Form.Item label="Password">
        <Space.Compact block>
          <Form.Item name={[fieldName, 'settings', 'password']} noStyle>
            <Input placeholder="Obfuscation password" style={{ width: 'calc(100% - 32px)' }} />
          </Form.Item>
          <Button
            icon={<ReloadOutlined />}
            onClick={() => form.setFieldValue(
              [...absolutePath, 'settings', 'password'],
              RandomUtil.randomLowerAndNum(16),
            )}
          />
        </Space.Compact>
      </Form.Item>

      {mode === 'gecko' && (
        <Form.Item
          label="Packet size"
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
        addonBefore="Min"
        min={GECKO_MIN_PACKET_SIZE}
        max={GECKO_MAX_PACKET_SIZE}
        precision={0}
        value={min}
        placeholder={String(DEFAULT_GECKO_PACKET_SIZE.min)}
        onChange={(next) => onChange?.(`${next ?? ''}-${max ?? ''}`)}
        style={{ width: '50%' }}
      />
      <InputNumber
        addonBefore="Max"
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
  udpFieldName, form, absoluteSettingsPath,
}: {
  udpFieldName: number;
  form: FormInstance;
  absoluteSettingsPath: (string | number)[];
}) {
  return (
    <>
      {(['client', 'server'] as const).map((groupKey) => (
        <Form.List key={groupKey} name={[udpFieldName, 'settings', groupKey]}>
          {(items, { add, remove }) => (
            <>
              <Form.Item label={groupKey === 'client' ? 'Client' : 'Server'}>
                <Button
                  type="primary"
                  size="small"
                  icon={<PlusOutlined />}
                  onClick={() => add(defaultUdpClientServerItem())}
                />
              </Form.Item>
              {items.map((item, ci) => (
                <div key={item.key}>
                  <Divider style={{ margin: 0 }}>
                    {groupKey === 'client' ? 'Client' : 'Server'} {ci + 1}
                    <DeleteOutlined className="danger-icon" onClick={() => remove(item.name)} />
                  </Divider>
                  <ItemEditor
                    fieldName={item.name}
                    form={form}
                    absoluteItemPath={[...absoluteSettingsPath, groupKey, item.name]}
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

function NoiseItems({
  udpFieldName, form, absoluteSettingsPath,
}: {
  udpFieldName: number;
  form: FormInstance;
  absoluteSettingsPath: (string | number)[];
}) {
  return (
    <>
      <Form.Item label="Reset" name={[udpFieldName, 'settings', 'reset']}>
        <InputNumber min={0} />
      </Form.Item>
      <Form.List name={[udpFieldName, 'settings', 'noise']}>
        {(items, { add, remove }) => (
          <>
            <Form.Item label="Noise">
              <Button
                type="primary"
                size="small"
                icon={<PlusOutlined />}
                onClick={() => add(defaultNoiseItem())}
              />
            </Form.Item>
            {items.map((item, ni) => (
              <div key={item.key}>
                <Divider style={{ margin: 0 }}>
                  Noise {ni + 1}
                  <DeleteOutlined className="danger-icon" onClick={() => remove(item.name)} />
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

function ItemEditor({
  fieldName, form, absoluteItemPath, delayMode, onRemove: _onRemove,
}: {
  fieldName: number;
  form: FormInstance;
  absoluteItemPath: (string | number)[];
  delayMode?: 'number' | 'string';
  onRemove?: () => void;
}) {
  const onTypeChange = (v: string) => {
    if (v === 'base64') {
      form.setFieldValue([...absoluteItemPath, 'packet'], RandomUtil.randomBase64());
    } else if (v === 'array') {
      form.setFieldValue([...absoluteItemPath, 'rand'], delayMode === 'string' ? '1-8192' : 0);
      form.setFieldValue([...absoluteItemPath, 'packet'], []);
    } else {
      form.setFieldValue([...absoluteItemPath, 'packet'], '');
    }
  };

  return (
    <>
      <Form.Item label="Type" name={[fieldName, 'type']}>
        <Select
          onChange={onTypeChange}
          options={[
            { value: 'array', label: 'Array' },
            { value: 'str', label: 'String' },
            { value: 'hex', label: 'Hex' },
            { value: 'base64', label: 'Base64' },
          ]}
        />
      </Form.Item>

      {delayMode === 'number' && (
        <Form.Item label="Delay (ms)" name={[fieldName, 'delay']}>
          <InputNumber min={0} />
        </Form.Item>
      )}
      {delayMode === 'string' && (
        <Form.Item label="Delay" name={[fieldName, 'delay']}>
          <Input placeholder="10-20" />
        </Form.Item>
      )}

      <Form.Item
        noStyle
        shouldUpdate={(prev, curr) => getDeep(prev, [...absoluteItemPath, 'type']) !== getDeep(curr, [...absoluteItemPath, 'type'])}
      >
        {({ getFieldValue }) => {
          const type = getFieldValue([...absoluteItemPath, 'type']) as string | undefined;
          if (type === 'array') {
            return (
              <>
                <Form.Item label="Rand" name={[fieldName, 'rand']}>
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
                  label="Rand Range"
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
              <Form.Item label="Packet">
                <Space.Compact block>
                  <Form.Item name={[fieldName, 'packet']} noStyle>
                    <Input placeholder="binary data" style={{ width: 'calc(100% - 32px)' }} />
                  </Form.Item>
                  <Button
                    icon={<ReloadOutlined />}
                    onClick={() => form.setFieldValue([...absoluteItemPath, 'packet'], RandomUtil.randomBase64())}
                  />
                </Space.Compact>
              </Form.Item>
            );
          }
          return (
            <Form.Item label="Packet" name={[fieldName, 'packet']}>
              <Input placeholder="binary data" />
            </Form.Item>
          );
        }}
      </Form.Item>
    </>
  );
}

function QuicParamsForm({ base, form }: { base: (string | number)[]; form: FormInstance }) {
  const congestion = Form.useWatch([...base, 'congestion'], form) as string | undefined;
  const udpHop = Form.useWatch([...base, 'udpHop'], { form, preserve: true }) as Record<string, unknown> | undefined;
  const hasUdpHop = udpHop != null;

  return (
    <>
      <Form.Item label="Congestion" name={[...base, 'congestion']}>
        <Select
          options={[
            { value: 'reno', label: 'Reno' },
            { value: 'bbr', label: 'BBR' },
            { value: 'brutal', label: 'Brutal' },
            { value: 'force-brutal', label: 'Force Brutal' },
          ]}
        />
      </Form.Item>
      {congestion === 'bbr' && (
        <Form.Item label="BBR Profile" name={[...base, 'bbrProfile']}>
          <Select
            allowClear
            placeholder="standard"
            options={[
              { value: 'conservative', label: 'Conservative' },
              { value: 'standard', label: 'Standard' },
              { value: 'aggressive', label: 'Aggressive' },
            ]}
          />
        </Form.Item>
      )}
      <Form.Item label="Debug" name={[...base, 'debug']} valuePropName="checked">
        <Switch />
      </Form.Item>

      {(congestion === 'brutal' || congestion === 'force-brutal') && (
        <>
          <Form.Item label="Brutal Up" name={[...base, 'brutalUp']}>
            <Input placeholder="e.g. 60 mbps" />
          </Form.Item>
          <Form.Item label="Brutal Down" name={[...base, 'brutalDown']}>
            <Input placeholder="e.g. 100 mbps" />
          </Form.Item>
        </>
      )}

      <Form.Item label="UDP Hop">
        <Switch
          checked={hasUdpHop}
          onChange={(v) => {
            form.setFieldValue([...base, 'udpHop'], v ? defaultUdpHop() : undefined);
          }}
        />
      </Form.Item>
      {hasUdpHop && (
        <>
          <Form.Item label="Hop Ports" name={[...base, 'udpHop', 'ports']}>
            <Input placeholder="e.g. 20000-50000" />
          </Form.Item>
          <Form.Item label="Hop Interval (s)" name={[...base, 'udpHop', 'interval']}>
            <Input placeholder="e.g. 5-10" />
          </Form.Item>
        </>
      )}

      <Form.Item label="Max Idle Timeout (s)" name={[...base, 'maxIdleTimeout']}>
        <InputNumber min={4} max={120} />
      </Form.Item>
      <Form.Item label="Keep Alive Period (s)" name={[...base, 'keepAlivePeriod']}>
        <InputNumber min={2} max={60} />
      </Form.Item>
      <Form.Item label="Disable Path MTU Dis" name={[...base, 'disablePathMTUDiscovery']} valuePropName="checked">
        <Switch />
      </Form.Item>

      <Form.Item label="Max Incoming Streams" name={[...base, 'maxIncomingStreams']}>
        <InputNumber min={8} placeholder="1024 = default" />
      </Form.Item>
      <Form.Item label="Init Stream Window" name={[...base, 'initStreamReceiveWindow']}>
        <InputNumber min={16384} placeholder="8388608 = default" />
      </Form.Item>
      <Form.Item label="Max Stream Window" name={[...base, 'maxStreamReceiveWindow']}>
        <InputNumber min={16384} placeholder="8388608 = default" />
      </Form.Item>
      <Form.Item label="Init Conn Window" name={[...base, 'initConnectionReceiveWindow']}>
        <InputNumber min={16384} placeholder="20971520 = default" />
      </Form.Item>
      <Form.Item label="Max Conn Window" name={[...base, 'maxConnectionReceiveWindow']}>
        <InputNumber min={16384} placeholder="20971520 = default" />
      </Form.Item>
    </>
  );
}
