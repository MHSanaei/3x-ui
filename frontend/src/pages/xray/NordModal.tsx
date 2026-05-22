import { useCallback, useEffect, useMemo, useState } from 'react';
import { Button, Divider, Form, Input, message, Modal, Select, Tabs, Tag } from 'antd';
import { LoginOutlined, SaveOutlined } from '@ant-design/icons';

import { HttpUtil } from '@/utils';
import './NordModal.css';

interface NordModalProps {
  open: boolean;
  templateSettings: { outbounds?: { tag?: string }[] } | null;
  onClose: () => void;
  onAddOutbound: (outbound: Record<string, unknown>) => void;
  onResetOutbound: (payload: { index: number; outbound: Record<string, unknown>; oldTag?: string; newTag: string }) => void;
  onRemoveOutbound: (index: number) => void;
  onRemoveRoutingRules: (payload: { prefix: string }) => void;
}

interface NordData {
  token?: string;
  private_key?: string;
}

interface Country {
  id: number;
  name: string;
  code: string;
}

interface City {
  id: number;
  name: string;
}

interface NordServer {
  id: number;
  name: string;
  hostname: string;
  station: string;
  load: number;
  technologies?: { id: number; metadata?: { name: string; value: string }[] }[];
  location_ids?: number[];
  cityId?: number | null;
  cityName?: string;
}

function loadColor(load: number): string {
  if (load < 30) return 'green';
  if (load < 70) return 'orange';
  return 'red';
}

export default function NordModal({
  open,
  templateSettings,
  onClose,
  onAddOutbound,
  onResetOutbound,
  onRemoveOutbound,
  onRemoveRoutingRules,
}: NordModalProps) {
  const [messageApi, messageContextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);
  const [nordData, setNordData] = useState<NordData | null>(null);
  const [token, setToken] = useState('');
  const [manualKey, setManualKey] = useState('');
  const [countries, setCountries] = useState<Country[]>([]);
  const [cities, setCities] = useState<City[]>([]);
  const [servers, setServers] = useState<NordServer[]>([]);
  const [countryId, setCountryId] = useState<number | null>(null);
  const [cityId, setCityId] = useState<number | null>(null);
  const [serverId, setServerId] = useState<number | null>(null);

  const nordOutboundIndex = useMemo(() => {
    const list = templateSettings?.outbounds;
    if (!list) return -1;
    return list.findIndex((o) => o?.tag?.startsWith?.('nord-'));
  }, [templateSettings?.outbounds]);

  const filteredServers = useMemo(() => {
    if (!cityId) return servers;
    return servers.filter((s) => s.cityId === cityId);
  }, [cityId, servers]);

  useEffect(() => {
    setServerId(filteredServers.length > 0 ? filteredServers[0].id : null);
  }, [filteredServers]);

  const fetchCountries = useCallback(async () => {
    const msg = await HttpUtil.post('/panel/xray/nord/countries');
    if (msg?.success) setCountries(JSON.parse(msg.obj));
  }, []);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/xray/nord/data');
      if (msg?.success) {
        const next = msg.obj ? JSON.parse(msg.obj) : null;
        setNordData(next);
        if (next) await fetchCountries();
      }
    } finally {
      setLoading(false);
    }
  }, [fetchCountries]);

  useEffect(() => {
    if (open) fetchData();
  }, [open, fetchData]);

  async function login() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/xray/nord/reg', { token });
      if (msg?.success) {
        setNordData(JSON.parse(msg.obj));
        await fetchCountries();
      }
    } finally {
      setLoading(false);
    }
  }

  async function saveKey() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/xray/nord/setKey', { key: manualKey });
      if (msg?.success) {
        setNordData(JSON.parse(msg.obj));
        await fetchCountries();
      }
    } finally {
      setLoading(false);
    }
  }

  async function logout() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/xray/nord/del');
      if (msg?.success) {
        onRemoveOutbound(nordOutboundIndex);
        onRemoveRoutingRules({ prefix: 'nord-' });
        setNordData(null);
        setToken('');
        setManualKey('');
        setCountries([]);
        setCities([]);
        setServers([]);
        setCountryId(null);
        setCityId(null);
        setServerId(null);
      }
    } finally {
      setLoading(false);
    }
  }

  async function fetchServers(newCountryId: number) {
    setCountryId(newCountryId);
    setLoading(true);
    setServers([]);
    setCities([]);
    setServerId(null);
    setCityId(null);
    try {
      const msg = await HttpUtil.post('/panel/xray/nord/servers', { countryId: newCountryId });
      if (!msg?.success) return;
      const data = JSON.parse(msg.obj);
      const locations = data.locations || [];
      const locToCity: Record<number, City> = {};
      const citiesMap = new Map<number, City>();
      for (const loc of locations) {
        if (loc.country?.city) {
          citiesMap.set(loc.country.city.id, loc.country.city);
          locToCity[loc.id] = loc.country.city;
        }
      }
      setCities(Array.from(citiesMap.values()).sort((a, b) => a.name.localeCompare(b.name)));
      const next: NordServer[] = (data.servers || [])
        .map((s: NordServer) => {
          const firstLocId = (s.location_ids || [])[0];
          const city = firstLocId != null ? locToCity[firstLocId] : null;
          return { ...s, cityId: city?.id || null, cityName: city?.name || 'Unknown' };
        })
        .sort((a: NordServer, b: NordServer) => a.load - b.load);
      setServers(next);
      if (next.length === 0) messageApi.warning('No servers found for the selected country');
    } finally {
      setLoading(false);
    }
  }

  function buildNordOutbound(): Record<string, unknown> | null {
    const server = servers.find((s) => s.id === serverId);
    if (!server) return null;
    const tech = server.technologies?.find((tt) => tt.id === 35);
    const publicKey = tech?.metadata?.find((m) => m.name === 'public_key')?.value;
    if (!publicKey) {
      messageApi.error('Selected server does not advertise a NordLynx public key.');
      return null;
    }
    return {
      tag: `nord-${server.hostname}`,
      protocol: 'wireguard',
      settings: {
        secretKey: nordData?.private_key,
        address: ['10.5.0.2/32'],
        peers: [{ publicKey, endpoint: `${server.station}:51820` }],
        noKernelTun: false,
      },
    };
  }

  function addOutbound() {
    const ob = buildNordOutbound();
    if (!ob) return;
    onAddOutbound(ob);
    messageApi.success('NordVPN outbound added');
    onClose();
  }

  function resetOutbound() {
    if (nordOutboundIndex === -1) return;
    const ob = buildNordOutbound();
    if (!ob) return;
    const oldTag = templateSettings?.outbounds?.[nordOutboundIndex]?.tag;
    onResetOutbound({
      index: nordOutboundIndex,
      outbound: ob,
      oldTag,
      newTag: ob.tag as string,
    });
    messageApi.success('NordVPN outbound updated');
    onClose();
  }

  return (
    <>
      {messageContextHolder}
      <Modal open={open} title="NordVPN NordLynx" footer={null} onCancel={onClose}>
      {nordData == null ? (
        <Tabs
          defaultActiveKey="token"
          items={[
            {
              key: 'token',
              label: 'Access token',
              children: (
                <Form
                  colon={false}
                  labelCol={{ md: { span: 6 } }}
                  wrapperCol={{ md: { span: 18 } }}
                  className="mt-20"
                >
                  <Form.Item label="Access token">
                    <Input
                      value={token}
                      placeholder="Access token"
                      onChange={(e) => setToken(e.target.value)}
                    />
                    <Button type="primary" className="mt-10" loading={loading} icon={<LoginOutlined />} onClick={login}>
                      Login
                    </Button>
                  </Form.Item>
                </Form>
              ),
            },
            {
              key: 'key',
              label: 'Private key',
              children: (
                <Form
                  colon={false}
                  labelCol={{ md: { span: 6 } }}
                  wrapperCol={{ md: { span: 18 } }}
                  className="mt-20"
                >
                  <Form.Item label="Private key">
                    <Input
                      value={manualKey}
                      placeholder="Private key"
                      onChange={(e) => setManualKey(e.target.value)}
                    />
                    <Button type="primary" className="mt-10" loading={loading} icon={<SaveOutlined />} onClick={saveKey}>
                      Save
                    </Button>
                  </Form.Item>
                </Form>
              ),
            },
          ]}
        />
      ) : (
        <>
          <table className="nord-data-table">
            <tbody>
              {nordData.token && (
                <tr className="row-odd">
                  <td>Access token</td>
                  <td>{nordData.token}</td>
                </tr>
              )}
              <tr>
                <td>Private key</td>
                <td>{nordData.private_key}</td>
              </tr>
            </tbody>
          </table>

          <Button loading={loading} type="primary" danger className="mt-8" onClick={logout}>
            Logout
          </Button>

          <Divider className="zero-margin">Settings</Divider>

          <Form colon={false} labelCol={{ md: { span: 6 } }} wrapperCol={{ md: { span: 18 } }} className="mt-10">
            <Form.Item label="Country">
              <Select
                value={countryId ?? undefined}
                showSearch
                optionFilterProp="label"
                onChange={(v) => fetchServers(v)}
                options={countries.map((c) => ({
                  value: c.id,
                  label: `${c.name} (${c.code})`,
                }))}
              />
            </Form.Item>

            {cities.length > 0 && (
              <Form.Item label="City">
                <Select
                  value={cityId}
                  showSearch
                  optionFilterProp="label"
                  onChange={setCityId}
                  options={[{ value: null, label: 'All cities' }, ...cities.map((c) => ({ value: c.id, label: c.name }))]}
                />
              </Form.Item>
            )}

            {filteredServers.length > 0 && (
              <Form.Item label="Server">
                <Select
                  value={serverId}
                  showSearch
                  optionFilterProp="label"
                  onChange={setServerId}
                  options={filteredServers.map((s) => ({
                    value: s.id,
                    label: `${s.cityName} ${s.name} ${s.hostname}`,
                    children: (
                      <span className="server-row">
                        <span className="server-name">
                          {s.cityName} - {s.name}
                        </span>
                        <Tag color={loadColor(s.load)} className="server-load-tag">
                          {s.load}%
                        </Tag>
                      </span>
                    ),
                  }))}
                />
              </Form.Item>
            )}
          </Form>

          <Divider className="my-10">Outbound status</Divider>
          {nordOutboundIndex >= 0 ? (
            <>
              <Tag color="green">Enabled</Tag>
              <Button type="primary" danger loading={loading} className="ml-8" onClick={resetOutbound}>
                Reset
              </Button>
            </>
          ) : (
            <>
              <Tag color="orange">Disabled</Tag>
              <Button
                type="primary"
                className="ml-8"
                disabled={!serverId}
                loading={loading}
                onClick={addOutbound}
              >
                Add outbound
              </Button>
            </>
          )}
        </>
      )}
      </Modal>
    </>
  );
}
