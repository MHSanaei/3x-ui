import { useMemo } from 'react';
import { Button, Divider, Form, Input, InputNumber, Select, Switch } from 'antd';
import { DeleteOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';

import { RandomUtil } from '@/utils';
import { Protocols } from '@/models/outbound';

interface StreamShape {
  network?: string;
  kcp?: { mtu?: number };
  finalmask: {
    tcp?: MaskRow[];
    udp?: MaskRow[];
    enableQuicParams?: boolean;
    quicParams?: QuicParams;
  };
  addTcpMask: (type?: string) => void;
  delTcpMask: (index: number) => void;
  addUdpMask: (type?: string) => void;
  delUdpMask: (index: number) => void;
}

interface MaskRow {
  type: string;
  settings: Record<string, unknown>;
  _getDefaultSettings: (type: string, settings: Record<string, unknown>) => Record<string, unknown>;
}

interface ItemRow {
  type: string;
  packet: string | unknown[];
  delay?: number | string;
  rand?: number | string;
  randRange?: string;
}

interface QuicParams {
  congestion: string;
  debug?: boolean;
  brutalUp?: number | string;
  brutalDown?: number | string;
  hasUdpHop?: boolean;
  udpHop?: { ports: string; interval: string | number };
  maxIdleTimeout?: number;
  keepAlivePeriod?: number;
  disablePathMTUDiscovery?: boolean;
  maxIncomingStreams?: number;
  initStreamReceiveWindow?: number;
  maxStreamReceiveWindow?: number;
  initConnectionReceiveWindow?: number;
  maxConnectionReceiveWindow?: number;
}

interface FinalMaskFormProps {
  stream: StreamShape;
  protocol: string;
  onChange: () => void;
}

function changeMaskType(mask: MaskRow, type: string) {
  mask.type = type;
  mask.settings = mask._getDefaultSettings(type, {});
}

function changeItemType(item: ItemRow, type: string) {
  item.type = type;
  if (type === 'base64') item.packet = RandomUtil.randomBase64();
  else if (type === 'array') {
    item.rand = 0;
    item.packet = [];
  } else item.packet = '';
}

function newClientServerItem(): ItemRow {
  return { delay: 0, rand: 0, randRange: '0-255', type: 'array', packet: [] };
}

function newUdpClientServerItem(): ItemRow {
  return { rand: 0, randRange: '0-255', type: 'array', packet: [] };
}

function newNoiseItem(): ItemRow {
  return { rand: '1-8192', randRange: '0-255', type: 'array', packet: [], delay: '10-20' };
}

export default function FinalMaskForm({ stream, protocol, onChange }: FinalMaskFormProps) {
  const isHysteria = protocol === Protocols.Hysteria || protocol === 'hysteria';
  const network = stream?.network || '';

  const showTcp = useMemo(
    () => ['raw', 'tcp', 'httpupgrade', 'ws', 'grpc', 'xhttp'].includes(network),
    [network],
  );
  const showUdp = isHysteria || network === 'kcp';
  const showQuic = isHysteria || network === 'xhttp';

  function notify() {
    onChange();
  }

  function changeUdpMaskType(mask: MaskRow, type: string) {
    changeMaskType(mask, type);
    if (network === 'kcp' && stream.kcp) {
      stream.kcp.mtu = type === 'xdns' ? 900 : 1350;
    }
    notify();
  }

  function addUdpMaskWithDefault() {
    const def = isHysteria ? 'salamander' : 'mkcp-aes128gcm';
    stream.addUdpMask(def);
    notify();
  }

  const tcpMasks = stream.finalmask.tcp || [];
  const udpMasks = stream.finalmask.udp || [];

  if (!showTcp && !showUdp && !showQuic) return null;

  return (
    <Form colon={false} labelCol={{ md: { span: 8 } }} wrapperCol={{ md: { span: 14 } }}>
      {showTcp && (
        <>
          <Form.Item label="TCP Masks">
            <Button
              type="primary"
              size="small"
              icon={<PlusOutlined />}
              onClick={() => {
                stream.addTcpMask('fragment');
                notify();
              }}
            />
          </Form.Item>

          {tcpMasks.map((mask, mIdx) => (
            <div key={`tcp-${mIdx}`}>
              <Divider style={{ margin: 0 }}>
                TCP Mask {mIdx + 1}
                <DeleteOutlined
                  className="danger-icon"
                  onClick={() => {
                    stream.delTcpMask(mIdx);
                    notify();
                  }}
                />
              </Divider>

              <Form.Item label="Type">
                <Select
                  value={mask.type}
                  onChange={(v) => {
                    changeMaskType(mask, v);
                    notify();
                  }}
                  options={[
                    { value: 'fragment', label: 'Fragment' },
                    { value: 'header-custom', label: 'Header Custom' },
                    { value: 'sudoku', label: 'Sudoku' },
                  ]}
                />
              </Form.Item>

              {mask.type === 'fragment' && (
                <>
                  <Form.Item label="Packets">
                    <Select
                      value={mask.settings.packets as string}
                      onChange={(v) => {
                        (mask.settings as Record<string, unknown>).packets = v;
                        notify();
                      }}
                      options={[
                        { value: 'tlshello', label: 'tlshello' },
                        { value: '1-3', label: '1-3' },
                        { value: '1-5', label: '1-5' },
                      ]}
                    />
                  </Form.Item>
                  {(['length', 'delay', 'maxSplit'] as const).map((field) => (
                    <Form.Item key={field} label={field === 'maxSplit' ? 'Max Split' : field.charAt(0).toUpperCase() + field.slice(1)}>
                      <Input
                        value={(mask.settings[field] as string) || ''}
                        onChange={(e) => {
                          (mask.settings as Record<string, unknown>)[field] = e.target.value;
                          notify();
                        }}
                      />
                    </Form.Item>
                  ))}
                </>
              )}

              {mask.type === 'sudoku' && (
                <>
                  {(['password', 'ascii', 'customTable', 'customTables'] as const).map((field) => (
                    <Form.Item key={field} label={field === 'customTable' ? 'Custom Table' : field === 'customTables' ? 'Custom Tables' : field.charAt(0).toUpperCase() + field.slice(1)}>
                      <Input
                        value={(mask.settings[field] as string) || ''}
                        onChange={(e) => {
                          (mask.settings as Record<string, unknown>)[field] = e.target.value;
                          notify();
                        }}
                      />
                    </Form.Item>
                  ))}
                  {(['paddingMin', 'paddingMax'] as const).map((field) => (
                    <Form.Item key={field} label={field === 'paddingMin' ? 'Padding Min' : 'Padding Max'}>
                      <InputNumber
                        value={(mask.settings[field] as number) || 0}
                        min={0}
                        onChange={(v) => {
                          (mask.settings as Record<string, unknown>)[field] = Number(v) || 0;
                          notify();
                        }}
                      />
                    </Form.Item>
                  ))}
                </>
              )}

              {mask.type === 'header-custom' && (
                <HeaderCustomGroups mask={mask} kind="tcp" onChange={notify} />
              )}
            </div>
          ))}
        </>
      )}

      {showUdp && (
        <>
          <Form.Item label="UDP Masks">
            <Button type="primary" size="small" icon={<PlusOutlined />} onClick={addUdpMaskWithDefault} />
          </Form.Item>

          {udpMasks.map((mask, mIdx) => (
            <div key={`udp-${mIdx}`}>
              <Divider style={{ margin: 0 }}>
                UDP Mask {mIdx + 1}
                <DeleteOutlined
                  className="danger-icon"
                  onClick={() => {
                    stream.delUdpMask(mIdx);
                    notify();
                  }}
                />
              </Divider>

              <Form.Item label="Type">
                <Select
                  value={mask.type}
                  onChange={(v) => changeUdpMaskType(mask, v)}
                  options={
                    isHysteria
                      ? [{ value: 'salamander', label: 'Salamander (Hysteria2)' }]
                      : [
                          { value: 'mkcp-aes128gcm', label: 'mKCP AES-128-GCM' },
                          { value: 'header-dns', label: 'Header DNS' },
                          { value: 'header-dtls', label: 'Header DTLS 1.2' },
                          { value: 'header-srtp', label: 'Header SRTP' },
                          { value: 'header-utp', label: 'Header uTP' },
                          { value: 'header-wechat', label: 'Header WeChat Video' },
                          { value: 'header-wireguard', label: 'Header WireGuard' },
                          { value: 'mkcp-original', label: 'mKCP Original' },
                          { value: 'xdns', label: 'xDNS' },
                          { value: 'xicmp', label: 'xICMP' },
                          { value: 'header-custom', label: 'Header Custom' },
                          { value: 'noise', label: 'Noise' },
                        ]
                  }
                />
              </Form.Item>

              {['mkcp-aes128gcm', 'salamander'].includes(mask.type) && (
                <Form.Item label="Password">
                  <Input
                    value={(mask.settings.password as string) || ''}
                    placeholder="Obfuscation password"
                    onChange={(e) => {
                      (mask.settings as Record<string, unknown>).password = e.target.value;
                      notify();
                    }}
                  />
                </Form.Item>
              )}

              {mask.type === 'header-dns' && (
                <Form.Item label="Domain">
                  <Input
                    value={(mask.settings.domain as string) || ''}
                    placeholder="e.g., www.example.com"
                    onChange={(e) => {
                      (mask.settings as Record<string, unknown>).domain = e.target.value;
                      notify();
                    }}
                  />
                </Form.Item>
              )}

              {mask.type === 'xdns' && (
                <Form.Item label="Domains">
                  <Select
                    mode="tags"
                    value={(mask.settings.domains as string[]) || []}
                    style={{ width: '100%' }}
                    tokenSeparators={[',']}
                    placeholder="e.g., www.example.com"
                    onChange={(v) => {
                      (mask.settings as Record<string, unknown>).domains = v;
                      notify();
                    }}
                  />
                </Form.Item>
              )}

              {mask.type === 'noise' && (
                <NoiseItems mask={mask} onChange={notify} />
              )}

              {mask.type === 'header-custom' && (
                <UdpHeaderCustom mask={mask} onChange={notify} />
              )}

              {mask.type === 'xicmp' && (
                <>
                  <Form.Item label="IP">
                    <Input
                      value={(mask.settings.ip as string) || ''}
                      placeholder="0.0.0.0"
                      onChange={(e) => {
                        (mask.settings as Record<string, unknown>).ip = e.target.value;
                        notify();
                      }}
                    />
                  </Form.Item>
                  <Form.Item label="ID">
                    <InputNumber
                      value={(mask.settings.id as number) || 0}
                      min={0}
                      onChange={(v) => {
                        (mask.settings as Record<string, unknown>).id = Number(v) || 0;
                        notify();
                      }}
                    />
                  </Form.Item>
                </>
              )}
            </div>
          ))}
        </>
      )}

      {showQuic && (
        <>
          <Form.Item label="QUIC Params">
            <Switch
              checked={!!stream.finalmask.enableQuicParams}
              onChange={(v) => {
                stream.finalmask.enableQuicParams = v;
                notify();
              }}
            />
          </Form.Item>
          {stream.finalmask.enableQuicParams && stream.finalmask.quicParams && (
            <QuicParamsForm params={stream.finalmask.quicParams} onChange={notify} />
          )}
        </>
      )}
    </Form>
  );
}

function HeaderCustomGroups({
  mask,
  kind: _kind,
  onChange,
}: {
  mask: MaskRow;
  kind: 'tcp';
  onChange: () => void;
}) {
  const settings = mask.settings as { clients?: ItemRow[][]; servers?: ItemRow[][] };
  if (!settings.clients) settings.clients = [];
  if (!settings.servers) settings.servers = [];

  return (
    <>
      {(['clients', 'servers'] as const).map((groupKey) => (
        <div key={groupKey}>
          <Form.Item label={groupKey === 'clients' ? 'Clients' : 'Servers'}>
            <Button
              type="primary"
              size="small"
              icon={<PlusOutlined />}
              onClick={() => {
                (settings[groupKey] as ItemRow[][]).push([newClientServerItem()]);
                onChange();
              }}
            />
          </Form.Item>
          {(settings[groupKey] as ItemRow[][]).map((group, gi) => (
            <div key={`${groupKey}-${gi}`}>
              <Divider style={{ margin: 0 }}>
                {groupKey === 'clients' ? 'Clients' : 'Servers'} Group {gi + 1}
                <DeleteOutlined
                  className="danger-icon"
                  onClick={() => {
                    (settings[groupKey] as ItemRow[][]).splice(gi, 1);
                    onChange();
                  }}
                />
              </Divider>
              {group.map((item, _ii) => (
                <ItemEditor key={_ii} item={item} onChange={onChange} delayAsNumber />
              ))}
            </div>
          ))}
        </div>
      ))}
    </>
  );
}

function UdpHeaderCustom({ mask, onChange }: { mask: MaskRow; onChange: () => void }) {
  const settings = mask.settings as { client?: ItemRow[]; server?: ItemRow[] };
  if (!settings.client) settings.client = [];
  if (!settings.server) settings.server = [];
  return (
    <>
      {(['client', 'server'] as const).map((groupKey) => (
        <div key={groupKey}>
          <Form.Item label={groupKey === 'client' ? 'Client' : 'Server'}>
            <Button
              type="primary"
              size="small"
              icon={<PlusOutlined />}
              onClick={() => {
                (settings[groupKey] as ItemRow[]).push(newUdpClientServerItem());
                onChange();
              }}
            />
          </Form.Item>
          {(settings[groupKey] as ItemRow[]).map((item, ci) => (
            <div key={ci}>
              <Divider style={{ margin: 0 }}>
                {groupKey === 'client' ? 'Client' : 'Server'} {ci + 1}
                <DeleteOutlined
                  className="danger-icon"
                  onClick={() => {
                    (settings[groupKey] as ItemRow[]).splice(ci, 1);
                    onChange();
                  }}
                />
              </Divider>
              <ItemEditor item={item} onChange={onChange} />
            </div>
          ))}
        </div>
      ))}
    </>
  );
}

function NoiseItems({ mask, onChange }: { mask: MaskRow; onChange: () => void }) {
  const settings = mask.settings as { reset?: number; noise?: ItemRow[] };
  if (!settings.noise) settings.noise = [];

  return (
    <>
      <Form.Item label="Reset">
        <InputNumber
          value={settings.reset || 0}
          min={0}
          onChange={(v) => {
            settings.reset = Number(v) || 0;
            onChange();
          }}
        />
      </Form.Item>
      <Form.Item label="Noise">
        <Button
          type="primary"
          size="small"
          icon={<PlusOutlined />}
          onClick={() => {
            (settings.noise as ItemRow[]).push(newNoiseItem());
            onChange();
          }}
        />
      </Form.Item>
      {(settings.noise as ItemRow[]).map((n, ni) => (
        <div key={ni}>
          <Divider style={{ margin: 0 }}>
            Noise {ni + 1}
            <DeleteOutlined
              className="danger-icon"
              onClick={() => {
                (settings.noise as ItemRow[]).splice(ni, 1);
                onChange();
              }}
            />
          </Divider>
          <ItemEditor item={n} onChange={onChange} delayAsString />
        </div>
      ))}
    </>
  );
}

function ItemEditor({
  item,
  onChange,
  delayAsNumber,
  delayAsString,
}: {
  item: ItemRow;
  onChange: () => void;
  delayAsNumber?: boolean;
  delayAsString?: boolean;
}) {
  return (
    <>
      <Form.Item label="Type">
        <Select
          value={item.type}
          onChange={(v) => {
            changeItemType(item, v);
            onChange();
          }}
          options={[
            { value: 'array', label: 'Array' },
            { value: 'str', label: 'String' },
            { value: 'hex', label: 'Hex' },
            { value: 'base64', label: 'Base64' },
          ]}
        />
      </Form.Item>
      {delayAsNumber && (
        <Form.Item label="Delay (ms)">
          <InputNumber
            value={typeof item.delay === 'number' ? item.delay : 0}
            min={0}
            onChange={(v) => {
              item.delay = Number(v) || 0;
              onChange();
            }}
          />
        </Form.Item>
      )}
      {item.type === 'array' ? (
        <>
          <Form.Item label="Rand">
            {delayAsString ? (
              <Input
                value={String(item.rand ?? '')}
                onChange={(e) => {
                  item.rand = e.target.value;
                  onChange();
                }}
                placeholder="0 or 1-8192"
              />
            ) : (
              <InputNumber
                value={typeof item.rand === 'number' ? item.rand : 0}
                min={0}
                onChange={(v) => {
                  item.rand = Number(v) || 0;
                  onChange();
                }}
              />
            )}
          </Form.Item>
          <Form.Item label="Rand Range">
            <Input
              value={item.randRange || ''}
              placeholder="0-255"
              onChange={(e) => {
                item.randRange = e.target.value;
                onChange();
              }}
            />
          </Form.Item>
        </>
      ) : (
        <Form.Item label="Packet">
          {item.type === 'base64' ? (
            <Input.Group compact>
              <Input
                value={String(item.packet ?? '')}
                placeholder="binary data"
                style={{ width: 'calc(100% - 32px)' }}
                onChange={(e) => {
                  item.packet = e.target.value;
                  onChange();
                }}
              />
              <Button
                icon={<ReloadOutlined />}
                onClick={() => {
                  item.packet = RandomUtil.randomBase64();
                  onChange();
                }}
              />
            </Input.Group>
          ) : (
            <Input
              value={String(item.packet ?? '')}
              placeholder="binary data"
              onChange={(e) => {
                item.packet = e.target.value;
                onChange();
              }}
            />
          )}
        </Form.Item>
      )}
      {delayAsString && (
        <Form.Item label="Delay">
          <Input
            value={typeof item.delay === 'string' ? item.delay : ''}
            placeholder="10-20"
            onChange={(e) => {
              item.delay = e.target.value;
              onChange();
            }}
          />
        </Form.Item>
      )}
    </>
  );
}

function QuicParamsForm({ params, onChange }: { params: QuicParams; onChange: () => void }) {
  function update<K extends keyof QuicParams>(key: K, value: QuicParams[K]) {
    params[key] = value;
    onChange();
  }
  return (
    <>
      <Form.Item label="Congestion">
        <Select
          value={params.congestion}
          onChange={(v) => update('congestion', v)}
          options={[
            { value: 'reno', label: 'Reno' },
            { value: 'bbr', label: 'BBR' },
            { value: 'brutal', label: 'Brutal' },
            { value: 'force-brutal', label: 'Force Brutal' },
          ]}
        />
      </Form.Item>
      <Form.Item label="Debug">
        <Switch checked={!!params.debug} onChange={(v) => update('debug', v)} />
      </Form.Item>
      {['brutal', 'force-brutal'].includes(params.congestion) && (
        <>
          <Form.Item label="Brutal Up">
            <Input
              value={String(params.brutalUp ?? '')}
              placeholder="65537"
              onChange={(e) => update('brutalUp', e.target.value)}
            />
          </Form.Item>
          <Form.Item label="Brutal Down">
            <Input
              value={String(params.brutalDown ?? '')}
              placeholder="65537"
              onChange={(e) => update('brutalDown', e.target.value)}
            />
          </Form.Item>
        </>
      )}
      <Form.Item label="UDP Hop">
        <Switch checked={!!params.hasUdpHop} onChange={(v) => update('hasUdpHop', v)} />
      </Form.Item>
      {params.hasUdpHop && params.udpHop && (
        <>
          <Form.Item label="Hop Ports">
            <Input
              value={params.udpHop.ports || ''}
              placeholder="e.g. 20000-50000"
              onChange={(e) => {
                params.udpHop!.ports = e.target.value;
                onChange();
              }}
            />
          </Form.Item>
          <Form.Item label="Hop Interval (s)">
            <InputNumber
              value={Number(params.udpHop.interval) || 5}
              min={5}
              onChange={(v) => {
                params.udpHop!.interval = Number(v) || 5;
                onChange();
              }}
            />
          </Form.Item>
        </>
      )}
      {(
        [
          ['maxIdleTimeout', 'Max Idle Timeout (s)', 4, 120],
          ['keepAlivePeriod', 'Keep Alive Period (s)', 2, 60],
        ] as const
      ).map(([key, label, min, max]) => (
        <Form.Item key={key} label={label}>
          <InputNumber
            value={params[key] as number}
            min={min}
            max={max}
            onChange={(v) => update(key, Number(v) || min)}
          />
        </Form.Item>
      ))}
      <Form.Item label="Disable Path MTU Dis">
        <Switch checked={!!params.disablePathMTUDiscovery} onChange={(v) => update('disablePathMTUDiscovery', v)} />
      </Form.Item>
      {(
        [
          ['maxIncomingStreams', 'Max Incoming Streams', 8, '1024 = default'],
          ['initStreamReceiveWindow', 'Init Stream Window', 16384, '8388608 = default'],
          ['maxStreamReceiveWindow', 'Max Stream Window', 16384, '8388608 = default'],
          ['initConnectionReceiveWindow', 'Init Conn Window', 16384, '20971520 = default'],
          ['maxConnectionReceiveWindow', 'Max Conn Window', 16384, '20971520 = default'],
        ] as const
      ).map(([key, label, min, placeholder]) => (
        <Form.Item key={key} label={label}>
          <InputNumber
            value={params[key] as number}
            min={min}
            placeholder={placeholder}
            onChange={(v) => update(key, Number(v) || 0)}
          />
        </Form.Item>
      ))}
    </>
  );
}
