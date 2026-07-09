import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Divider, Form, Input, message, Modal, Select, Tabs, Tag } from 'antd';
import { LoginOutlined, SaveOutlined } from '@ant-design/icons';
import { FormProvider, useForm, useWatch } from 'react-hook-form';

import { HttpUtil } from '@/utils';
import { FormField } from '@/components/form/rhf';
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

interface NordFormValues {
  token: string;
  manualKey: string;
  countryId: number | null;
  cityId: number | null;
  serverId: number | null;
}

const EMPTY: NordFormValues = {
  token: '',
  manualKey: '',
  countryId: null,
  cityId: null,
  serverId: null,
};

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
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);
  const [nordData, setNordData] = useState<NordData | null>(null);
  const [countries, setCountries] = useState<Country[]>([]);
  const [cities, setCities] = useState<City[]>([]);
  const [servers, setServers] = useState<NordServer[]>([]);
  const methods = useForm<NordFormValues>({ defaultValues: EMPTY });
  const cityId = useWatch({ control: methods.control, name: 'cityId' });
  const serverId = useWatch({ control: methods.control, name: 'serverId' });

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
    methods.setValue('serverId', filteredServers.length > 0 ? filteredServers[0].id : null);
  }, [filteredServers, methods]);

  const fetchCountries = useCallback(async () => {
    const msg = await HttpUtil.post<string>('/panel/api/xray/nord/countries');
    if (msg?.success && msg.obj) setCountries(JSON.parse(msg.obj));
  }, []);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const msg = await HttpUtil.post<string>('/panel/api/xray/nord/data');
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
      const msg = await HttpUtil.post<string>('/panel/api/xray/nord/reg', { token: methods.getValues('token') });
      if (msg?.success && msg.obj) {
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
      const msg = await HttpUtil.post<string>('/panel/api/xray/nord/setKey', { key: methods.getValues('manualKey') });
      if (msg?.success && msg.obj) {
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
      const msg = await HttpUtil.post('/panel/api/xray/nord/del');
      if (msg?.success) {
        onRemoveOutbound(nordOutboundIndex);
        onRemoveRoutingRules({ prefix: 'nord-' });
        setNordData(null);
        methods.reset(EMPTY);
        setCountries([]);
        setCities([]);
        setServers([]);
      }
    } finally {
      setLoading(false);
    }
  }

  async function fetchServers(newCountryId: number) {
    setLoading(true);
    setServers([]);
    setCities([]);
    methods.setValue('serverId', null);
    methods.setValue('cityId', null);
    try {
      const msg = await HttpUtil.post<string>('/panel/api/xray/nord/servers', { countryId: newCountryId });
      if (!msg?.success || !msg.obj) return;
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
      if (next.length === 0) messageApi.warning(t('pages.xray.nord.noServers'));
    } finally {
      setLoading(false);
    }
  }

  function buildNordOutbound(): Record<string, unknown> | null {
    const selectedServerId = methods.getValues('serverId');
    const server = servers.find((s) => s.id === selectedServerId);
    if (!server) return null;
    const tech = server.technologies?.find((tt) => tt.id === 35);
    const publicKey = tech?.metadata?.find((m) => m.name === 'public_key')?.value;
    if (!publicKey) {
      messageApi.error(t('pages.xray.nord.noPublicKey'));
      return null;
    }
    return {
      tag: `nord-${server.hostname}`,
      protocol: 'wireguard',
      settings: {
        secretKey: nordData?.private_key,
        address: ['10.5.0.2/32'],
        peers: [{ publicKey, endpoint: `${server.station}:51820` }],
        // Userspace TUN — same reasoning as the WARP outbound (#5205): kernel
        // TUN fails silently on many VPS setups and diverges from the data
        // path the panel's connectivity test exercises.
        noKernelTun: true,
      },
    };
  }

  function addOutbound() {
    const ob = buildNordOutbound();
    if (!ob) return;
    onAddOutbound(ob);
    messageApi.success(t('pages.xray.nord.outboundAdded'));
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
    messageApi.success(t('pages.xray.nord.outboundUpdated'));
    onClose();
  }

  return (
    <>
      {messageContextHolder}
      <Modal open={open} title="NordVPN NordLynx" footer={null} onCancel={onClose}>
      <FormProvider {...methods}>
      {nordData == null ? (
        <Tabs
          defaultActiveKey="token"
          items={[
            {
              key: 'token',
              label: t('pages.xray.nord.accessToken'),
              children: (
                <Form
                  colon={false}
                  labelCol={{ md: { span: 6 } }}
                  wrapperCol={{ md: { span: 18 } }}
                  className="mt-20"
                >
                  <FormField name="token" label={t('pages.xray.nord.accessToken')}>
                    <Input placeholder={t('pages.xray.nord.accessToken')} />
                  </FormField>
                  <Button type="primary" className="mt-10" loading={loading} icon={<LoginOutlined />} onClick={login}>
                    {t('login')}
                  </Button>
                </Form>
              ),
            },
            {
              key: 'key',
              label: t('pages.xray.nord.privateKey'),
              children: (
                <Form
                  colon={false}
                  labelCol={{ md: { span: 6 } }}
                  wrapperCol={{ md: { span: 18 } }}
                  className="mt-20"
                >
                  <FormField name="manualKey" label={t('pages.xray.nord.privateKey')}>
                    <Input placeholder={t('pages.xray.nord.privateKey')} />
                  </FormField>
                  <Button type="primary" className="mt-10" loading={loading} icon={<SaveOutlined />} onClick={saveKey}>
                    {t('save')}
                  </Button>
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
                  <td>{t('pages.xray.nord.accessToken')}</td>
                  <td>{nordData.token}</td>
                </tr>
              )}
              <tr>
                <td>{t('pages.xray.nord.privateKey')}</td>
                <td>{nordData.private_key}</td>
              </tr>
            </tbody>
          </table>

          <Button loading={loading} type="primary" danger className="mt-8" onClick={logout}>
            {t('logout')}
          </Button>

          <Divider className="zero-margin">{t('pages.xray.warp.settings')}</Divider>

          <Form colon={false} labelCol={{ md: { span: 6 } }} wrapperCol={{ md: { span: 18 } }} className="mt-10">
            <FormField
              name="countryId"
              label={t('pages.xray.outbound.country')}
              transform={{ input: (v) => v ?? undefined }}
              onAfterChange={(v) => fetchServers(v as number)}
            >
              <Select
                showSearch={{ optionFilterProp: 'label' }}
                options={countries.map((c) => ({
                  value: c.id,
                  label: `${c.name} (${c.code})`,
                }))}
              />
            </FormField>

            {cities.length > 0 && (
              <FormField name="cityId" label={t('pages.xray.outbound.city')}>
                <Select
                  showSearch={{ optionFilterProp: 'label' }}
                  options={[{ value: null, label: t('pages.xray.outbound.allCities') }, ...cities.map((c) => ({ value: c.id, label: c.name }))]}
                />
              </FormField>
            )}

            {filteredServers.length > 0 && (
              <FormField name="serverId" label={t('pages.xray.outbound.server')}>
                <Select
                  showSearch={{ optionFilterProp: 'label' }}
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
              </FormField>
            )}
          </Form>

          <Divider className="my-10">{t('pages.xray.outbound.outboundStatus')}</Divider>
          {nordOutboundIndex >= 0 ? (
            <>
              <Tag color="green">{t('enabled')}</Tag>
              <Button type="primary" danger loading={loading} className="ml-8" onClick={resetOutbound}>
                {t('reset')}
              </Button>
            </>
          ) : (
            <>
              <Tag color="orange">{t('disabled')}</Tag>
              <Button
                type="primary"
                className="ml-8"
                disabled={!serverId}
                loading={loading}
                onClick={addOutbound}
              >
                {t('pages.xray.warp.addOutbound')}
              </Button>
            </>
          )}
        </>
      )}
      </FormProvider>
      </Modal>
    </>
  );
}
