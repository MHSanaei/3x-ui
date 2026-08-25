import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Divider, Form, Input, message, Modal, Select, Tabs } from 'antd';
import { LoginOutlined, SaveOutlined } from '@ant-design/icons';
import { FormProvider, useForm, useWatch } from 'react-hook-form';

import { HttpUtil } from '@/utils';
import { FormField } from '@/components/form/rhf';
import { countryFlag, countryName } from '../outbounds/outbounds-tab-helpers';
import './NordModal.css';

interface NordModalProps {
  open: boolean;
  templateSettings: { outbounds?: NordOutboundRow[] } | null;
  onClose: () => void;
  onAddOutbound: (outbound: Record<string, unknown>) => void;
  onResetOutbound: (payload: {
    index: number;
    outbound: Record<string, unknown>;
    oldTag?: string;
    newTag: string;
  }) => void;
}

interface NordOutboundRow {
  tag?: string;
  protocol?: string;
  settings?: unknown;
}

interface NordAddedRow {
  index: number;
  tag: string;
  endpoint: string;
  resettable: boolean;
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

interface NordServerOption {
  value: number;
  label: string;
  searchText: string;
  server: NordServer;
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

function loadLevel(load: number): 'low' | 'medium' | 'high' {
  if (load < 30) return 'low';
  if (load < 70) return 'medium';
  return 'high';
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isResettableNordOutbound(outbound: NordOutboundRow): boolean {
  if (outbound.protocol !== 'wireguard' || !isRecord(outbound.settings)) return false;
  return (
    Array.isArray(outbound.settings.address) &&
    outbound.settings.address.length > 0 &&
    Array.isArray(outbound.settings.peers) &&
    outbound.settings.peers.length > 0
  );
}

function nordOutboundEndpoint(outbound: NordOutboundRow): string {
  if (!isRecord(outbound.settings) || !Array.isArray(outbound.settings.peers)) return '';
  const peer = outbound.settings.peers.find(isRecord);
  return typeof peer?.endpoint === 'string' ? peer.endpoint : '';
}

export default function NordModal({
  open,
  templateSettings,
  onClose,
  onAddOutbound,
  onResetOutbound,
}: NordModalProps) {
  const { t, i18n } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);
  const [nordData, setNordData] = useState<NordData | null>(null);
  const [countries, setCountries] = useState<Country[]>([]);
  const [cities, setCities] = useState<City[]>([]);
  const [servers, setServers] = useState<NordServer[]>([]);
  const methods = useForm<NordFormValues>({ defaultValues: EMPTY });
  const cityId = useWatch({ control: methods.control, name: 'cityId' });
  const serverId = useWatch({ control: methods.control, name: 'serverId' });
  const locale = i18n.resolvedLanguage || i18n.language;

  const nordRows = useMemo<NordAddedRow[]>(() => {
    const list = templateSettings?.outbounds;
    if (!list) return [];
    return list.flatMap((outbound, index) => {
      const tag = outbound?.tag;
      if (typeof tag !== 'string' || !tag.startsWith('nord-')) return [];
      return [
        {
          index,
          tag,
          endpoint: nordOutboundEndpoint(outbound),
          resettable: isResettableNordOutbound(outbound),
        },
      ];
    });
  }, [templateSettings?.outbounds]);

  const addedTags = useMemo(() => new Set(nordRows.map((row) => row.tag)), [nordRows]);

  const filteredServers = useMemo(() => {
    if (!cityId) return servers;
    return servers.filter((s) => s.cityId === cityId);
  }, [cityId, servers]);

  const selectedServer = filteredServers.find((server) => server.id === serverId);
  const selectedTag = selectedServer ? `nord-${selectedServer.hostname}` : '';
  const selectedAlreadyAdded = Boolean(selectedTag && addedTags.has(selectedTag));
  const serverOptions = useMemo<NordServerOption[]>(
    () =>
      filteredServers.map((server) => ({
        value: server.id,
        label: server.hostname,
        searchText:
          `${server.cityName ?? ''} ${server.name} ${server.hostname} ${server.station}`.toLowerCase(),
        server,
      })),
    [filteredServers],
  );

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
    if (!open) return;
    let cancelled = false;
    void (async () => {
      await fetchData();
      if (cancelled) return;
    })();
    return () => {
      cancelled = true;
    };
  }, [open, fetchData]);

  async function login() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post<string>('/panel/api/xray/nord/reg', {
        token: methods.getValues('token'),
      });
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
      const msg = await HttpUtil.post<string>('/panel/api/xray/nord/setKey', {
        key: methods.getValues('manualKey'),
      });
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
      const msg = await HttpUtil.post<string>('/panel/api/xray/nord/servers', {
        countryId: newCountryId,
      });
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
      methods.setValue('cityId', 0);
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
    if (selectedAlreadyAdded) return;
    const ob = buildNordOutbound();
    if (!ob) return;
    const tag = typeof ob.tag === 'string' ? ob.tag : '';
    if (tag && templateSettings?.outbounds?.some((outbound) => outbound?.tag === tag)) return;
    onAddOutbound(ob);
    messageApi.success(t('pages.xray.nord.outboundAdded'));
  }

  function resetOutbound(index: number) {
    const existing = templateSettings?.outbounds?.[index];
    if (
      !existing?.tag?.startsWith?.('nord-') ||
      !isResettableNordOutbound(existing) ||
      !isRecord(existing.settings) ||
      !nordData?.private_key
    ) {
      return;
    }
    const ob = {
      ...existing,
      settings: { ...existing.settings, secretKey: nordData.private_key },
    };
    onResetOutbound({
      index,
      outbound: ob,
      oldTag: existing.tag,
      newTag: existing.tag,
    });
    messageApi.success(t('pages.xray.nord.outboundUpdated'));
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title="NordVPN NordLynx"
        footer={null}
        width={680}
        className="nord-modal"
        onCancel={onClose}
      >
        <FormProvider {...methods}>
          {nordData == null ? (
            <Tabs
              defaultActiveKey="token"
              items={[
                {
                  key: 'token',
                  label: t('pages.xray.nord.accessToken'),
                  children: (
                    <Form colon={false} layout="vertical" className="nord-login-form">
                      <FormField name="token" label={t('pages.xray.nord.accessToken')}>
                        <Input placeholder={t('pages.xray.nord.accessToken')} />
                      </FormField>
                      <Button
                        type="primary"
                        className="nord-login-action"
                        loading={loading}
                        icon={<LoginOutlined />}
                        onClick={login}
                      >
                        {t('login')}
                      </Button>
                    </Form>
                  ),
                },
                {
                  key: 'key',
                  label: t('pages.xray.nord.privateKey'),
                  children: (
                    <Form colon={false} layout="vertical" className="nord-login-form">
                      <FormField name="manualKey" label={t('pages.xray.nord.privateKey')}>
                        <Input placeholder={t('pages.xray.nord.privateKey')} />
                      </FormField>
                      <Button
                        type="primary"
                        className="nord-login-action"
                        loading={loading}
                        icon={<SaveOutlined />}
                        onClick={saveKey}
                      >
                        {t('save')}
                      </Button>
                    </Form>
                  ),
                },
              ]}
            />
          ) : (
            <>
              <div className="nord-account-card">
                <table className="nord-data-table">
                  <tbody>
                    {nordData.token && (
                      <tr>
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
                <Button loading={loading} danger onClick={logout}>
                  {t('logout')}
                </Button>
              </div>

              <Divider className="nord-section-divider">{t('pages.xray.warp.settings')}</Divider>

              <Form colon={false} layout="vertical" className="nord-location-form">
                <div className="nord-location-grid">
                  <FormField
                    name="countryId"
                    label={t('pages.xray.outbound.country')}
                    transform={{ input: (v) => v ?? undefined }}
                    onAfterChange={(v) => fetchServers(v as number)}
                  >
                    <Select
                      data-testid="nord-country-select"
                      showSearch={{ optionFilterProp: 'label' }}
                      options={countries.map((c) => {
                        const name = countryName(c.code, locale) || c.name || c.code;
                        const flag = countryFlag(c.code);
                        return {
                          value: c.id,
                          label: `${flag ? `${flag} ` : ''}${name} (${c.code})`,
                        };
                      })}
                    />
                  </FormField>

                  {cities.length > 0 && (
                    <FormField name="cityId" label={t('pages.xray.outbound.city')}>
                      <Select
                        data-testid="nord-city-select"
                        showSearch={{ optionFilterProp: 'label' }}
                        options={[
                          { value: 0, label: t('pages.xray.outbound.allCities') },
                          ...cities.map((c) => ({ value: c.id, label: c.name })),
                        ]}
                      />
                    </FormField>
                  )}

                  {filteredServers.length > 0 && (
                    <div className="nord-server-field">
                      <FormField name="serverId" label={t('pages.xray.outbound.server')}>
                        <Select<number, NordServerOption>
                          data-testid="nord-server-select"
                          classNames={{ popup: { root: 'nord-server-popup' } }}
                          listHeight={320}
                          listItemHeight={58}
                          options={serverOptions}
                          showSearch={{
                            filterOption: (input, option) =>
                              option?.searchText.includes(input.trim().toLowerCase()) ?? false,
                          }}
                          optionRender={(option) => {
                            const server = option.data.server;
                            return (
                              <div className="nord-server-option">
                                <span className="nord-server-option-copy">
                                  <span className="nord-server-option-name">{server.name}</span>
                                  <span className="nord-server-option-meta">
                                    <span>{server.cityName}</span>
                                    <span aria-hidden="true">·</span>
                                    <span className="nord-server-option-hostname">
                                      {server.hostname}
                                    </span>
                                    <span aria-hidden="true">·</span>
                                    <span className="nord-server-option-address">
                                      {server.station}:51820
                                    </span>
                                  </span>
                                </span>
                                <span
                                  className={`nord-server-load nord-server-load-${loadLevel(server.load)}`}
                                  title={`${t('pages.xray.nord.serverLoad')}: ${server.load}%`}
                                >
                                  <span className="nord-server-load-dot" aria-hidden="true" />
                                  <span className="nord-server-load-label">
                                    {t('pages.xray.nord.serverLoad')}
                                  </span>
                                  <span className="nord-server-load-value">{server.load}%</span>
                                </span>
                              </div>
                            );
                          }}
                          labelRender={() =>
                            selectedServer ? (
                              <span className="nord-selected-server">
                                <span className="nord-selected-server-name">
                                  {selectedServer.name}
                                </span>
                                <span className="nord-selected-server-hostname">
                                  {selectedServer.hostname}
                                </span>
                                <span className="nord-selected-server-address">
                                  {selectedServer.station}:51820
                                </span>
                                <span
                                  className={`nord-server-load nord-server-load-${loadLevel(selectedServer.load)}`}
                                  title={`${t('pages.xray.nord.serverLoad')}: ${selectedServer.load}%`}
                                >
                                  <span className="nord-server-load-dot" aria-hidden="true" />
                                  <span className="nord-server-load-value">
                                    {selectedServer.load}%
                                  </span>
                                </span>
                              </span>
                            ) : null
                          }
                        />
                      </FormField>
                    </div>
                  )}
                </div>
              </Form>

              <div className="nord-add-actions">
                <div className="nord-already-added" aria-live="polite">
                  {selectedAlreadyAdded
                    ? t('pages.xray.nord.alreadyAdded', { reset: t('reset') })
                    : null}
                </div>
                <Button
                  type="primary"
                  disabled={!serverId || selectedAlreadyAdded}
                  loading={loading}
                  onClick={addOutbound}
                >
                  {t('pages.xray.warp.addOutbound')}
                </Button>
              </div>

              {nordRows.length > 0 && (
                <>
                  <Divider className="nord-section-divider">
                    {t('pages.xray.nord.addedServers')}
                  </Divider>
                  <table className="nord-added-table" data-testid="nord-added-table">
                    <tbody>
                      {nordRows.map((row) => (
                        <tr key={`${row.index}-${row.tag}`}>
                          <td>
                            <span className="nord-added-server-tag">{row.tag}</span>
                            {row.endpoint && (
                              <span className="nord-added-server-endpoint">{row.endpoint}</span>
                            )}
                          </td>
                          <td>
                            <Button
                              type="primary"
                              danger
                              size="small"
                              loading={loading}
                              disabled={!row.resettable}
                              data-testid={`nord-reset-${row.index}`}
                              onClick={() => resetOutbound(row.index)}
                            >
                              {t('reset')}
                            </Button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </>
              )}
            </>
          )}
        </FormProvider>
      </Modal>
    </>
  );
}
