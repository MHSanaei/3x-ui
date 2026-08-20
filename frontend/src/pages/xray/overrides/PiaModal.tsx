import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Divider, Form, Input, message, Modal, Select } from 'antd';
import { LoginOutlined } from '@ant-design/icons';
import { FormProvider, useForm, useWatch } from 'react-hook-form';

import { HttpUtil } from '@/utils';
import { FormField } from '@/components/form/rhf';
import { countryFlag, countryName } from '../outbounds/outbounds-tab-helpers';
import './PiaModal.css';

interface PiaOutboundRow {
  tag?: string;
  piaHostname?: string;
}

interface PiaModalProps {
  open: boolean;
  templateSettings: { outbounds?: PiaOutboundRow[] } | null;
  onClose: () => void;
  onAddOutbound: (outbound: Record<string, unknown>) => void;
  onResetOutbound: (payload: {
    index: number;
    outbound: Record<string, unknown>;
    oldTag?: string;
    newTag: string;
  }) => void;
}

interface PiaAccount {
  username?: string;
  accountHint?: string;
}

interface PiaCountry {
  code: string;
}

interface PiaRegion {
  id: string;
  name: string;
}

interface PiaServer {
  hostname: string;
  ip: string;
  regionId: string;
  regionName: string;
}

interface PiaKey {
  tag: string;
  hostname: string;
  secretKey: string;
  address: string;
  publicKey: string;
  endpoint: string;
}

interface PiaFormValues {
  username: string;
  password: string;
  countryCode: string | null;
  regionId: string | null;
  hostname: string | null;
}

const EMPTY: PiaFormValues = {
  username: '',
  password: '',
  countryCode: null,
  regionId: null,
  hostname: null,
};

function piaHostnameOf(outbound: PiaOutboundRow): string {
  if (typeof outbound.piaHostname === 'string' && outbound.piaHostname.trim()) {
    return outbound.piaHostname.trim();
  }
  return '';
}

function piaTagPart(s: string, stripDomain: boolean): string {
  s = s.trim().toLowerCase();
  if (stripDomain) {
    const i = s.indexOf('.');
    if (i > 0) s = s.slice(0, i);
  }
  return s.replaceAll('_', '-');
}

function piaOutboundTag(regionId: string, hostname: string): string {
  const region = piaTagPart(regionId, false);
  const server = piaTagPart(hostname, true);
  if (!region) return `pia-${server}`;
  return `pia-${region}-${server}`;
}

function buildPiaOutbound(key: PiaKey): Record<string, unknown> {
  return {
    tag: key.tag || `pia-${key.hostname}`,
    piaHostname: key.hostname,
    protocol: 'wireguard',
    settings: {
      secretKey: key.secretKey,
      address: [key.address],
      mtu: 1420,
      noKernelTun: true,
      peers: [
        {
          publicKey: key.publicKey,
          endpoint: key.endpoint,
          allowedIPs: ['0.0.0.0/0'],
          keepAlive: 25,
        },
      ],
    },
  };
}

export default function PiaModal({
  open,
  templateSettings,
  onClose,
  onAddOutbound,
  onResetOutbound,
}: PiaModalProps) {
  const { t, i18n } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);
  const [piaData, setPiaData] = useState<PiaAccount | null>(null);
  const [countries, setCountries] = useState<PiaCountry[]>([]);
  const [regions, setRegions] = useState<PiaRegion[]>([]);
  const [servers, setServers] = useState<PiaServer[]>([]);
  const methods = useForm<PiaFormValues>({ defaultValues: EMPTY });
  const regionId = useWatch({ control: methods.control, name: 'regionId' });
  const hostname = useWatch({ control: methods.control, name: 'hostname' });
  const locale = i18n.resolvedLanguage || i18n.language;

  const piaRows = useMemo(() => {
    const list = templateSettings?.outbounds;
    if (!list) return [];
    return list.flatMap((outbound, index) => {
      if (!outbound?.tag?.startsWith?.('pia-')) return [];
      return [{ index, tag: outbound.tag, hostname: piaHostnameOf(outbound) }];
    });
  }, [templateSettings?.outbounds]);

  const addedHostnames = useMemo(
    () => new Set(piaRows.map((row) => row.hostname).filter(Boolean)),
    [piaRows],
  );
  const addedTags = useMemo(
    () => new Set(piaRows.map((row) => row.tag).filter(Boolean)),
    [piaRows],
  );

  const filteredServers = useMemo(() => {
    if (!regionId) return servers;
    return servers.filter((s) => s.regionId === regionId);
  }, [regionId, servers]);

  const selectedServer = filteredServers.find((s) => s.hostname === hostname);
  const selectedTag = selectedServer
    ? piaOutboundTag(selectedServer.regionId, selectedServer.hostname)
    : '';
  const selectedAlreadyAdded = Boolean(
    (hostname && addedHostnames.has(hostname)) || (selectedTag && addedTags.has(selectedTag)),
  );

  useEffect(() => {
    methods.setValue('hostname', filteredServers.length > 0 ? filteredServers[0].hostname : null);
  }, [filteredServers, methods]);

  const fetchCountries = useCallback(async () => {
    const msg = await HttpUtil.post<PiaCountry[]>('/panel/api/xray/pia/countries');
    if (msg?.success && Array.isArray(msg.obj)) setCountries(msg.obj);
  }, []);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const msg = await HttpUtil.post<PiaAccount | null>('/panel/api/xray/pia/data');
      if (msg?.success) {
        const next = msg.obj ?? null;
        setPiaData(next);
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
      const msg = await HttpUtil.post<PiaAccount>('/panel/api/xray/pia/reg', {
        username: methods.getValues('username'),
        password: methods.getValues('password'),
      });
      if (msg?.success && msg.obj) {
        setPiaData(msg.obj);
        methods.setValue('password', '');
        await fetchCountries();
      }
    } finally {
      setLoading(false);
    }
  }

  async function logout() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/xray/pia/del');
      if (msg?.success) {
        setPiaData(null);
        methods.reset(EMPTY);
        setCountries([]);
        setRegions([]);
        setServers([]);
      }
    } finally {
      setLoading(false);
    }
  }

  async function fetchServers(newCountryCode: string) {
    setLoading(true);
    setServers([]);
    setRegions([]);
    methods.setValue('hostname', null);
    methods.setValue('regionId', null);
    try {
      const msg = await HttpUtil.post<{ regions?: PiaRegion[]; servers?: PiaServer[] }>(
        '/panel/api/xray/pia/servers',
        { countryCode: newCountryCode },
      );
      if (!msg?.success || !msg.obj) return;
      const nextRegions = msg.obj.regions || [];
      const nextServers = msg.obj.servers || [];
      setRegions(nextRegions);
      setServers(nextServers);
      if (nextServers.length === 0) messageApi.warning(t('pages.xray.pia.noServers'));
    } finally {
      setLoading(false);
    }
  }

  async function provisionOutbound(
    selectedHostname: string,
  ): Promise<Record<string, unknown> | null> {
    if (!selectedHostname) return null;
    const msg = await HttpUtil.post<PiaKey>('/panel/api/xray/pia/addKey', {
      hostname: selectedHostname,
    });
    if (!msg?.success) return null;
    if (!msg.obj?.secretKey || !msg.obj.publicKey || !msg.obj.endpoint || !msg.obj.address) {
      messageApi.error(t('pages.xray.pia.provisionFailed'));
      return null;
    }
    return buildPiaOutbound(msg.obj);
  }

  async function addOutbound() {
    const selected = methods.getValues('hostname');
    if (!selected || selectedAlreadyAdded) return;
    setLoading(true);
    try {
      const ob = await provisionOutbound(selected);
      if (!ob) return;
      const tag = typeof ob.tag === 'string' ? ob.tag : '';
      if (tag && templateSettings?.outbounds?.some((outbound) => outbound?.tag === tag)) return;
      onAddOutbound(ob);
      messageApi.success(t('pages.xray.pia.outboundAdded'));
    } finally {
      setLoading(false);
    }
  }

  async function resetOutbound(index: number, selectedHostname: string) {
    if (!selectedHostname) return;
    setLoading(true);
    try {
      const ob = await provisionOutbound(selectedHostname);
      if (!ob) return;
      const oldTag = templateSettings?.outbounds?.[index]?.tag;
      onResetOutbound({
        index,
        outbound: ob,
        oldTag,
        newTag: ob.tag as string,
      });
      messageApi.success(t('pages.xray.pia.outboundUpdated'));
    } finally {
      setLoading(false);
    }
  }

  return (
    <>
      {messageContextHolder}
      <Modal open={open} title="Private Internet Access WireGuard" footer={null} onCancel={onClose}>
        <FormProvider {...methods}>
          {piaData == null ? (
            <Form
              colon={false}
              labelCol={{ md: { span: 6 } }}
              wrapperCol={{ md: { span: 18 } }}
              className="mt-20"
            >
              <FormField name="username" label={t('pages.xray.pia.username')}>
                <Input placeholder={t('pages.xray.pia.username')} autoComplete="username" />
              </FormField>
              <FormField name="password" label={t('pages.xray.pia.password')}>
                <Input.Password
                  placeholder={t('pages.xray.pia.password')}
                  autoComplete="current-password"
                />
              </FormField>
              <Button
                type="primary"
                className="mt-10"
                loading={loading}
                icon={<LoginOutlined />}
                onClick={() => void login()}
              >
                {t('login')}
              </Button>
            </Form>
          ) : (
            <>
              <table className="pia-data-table">
                <tbody>
                  <tr className="row-odd">
                    <td>{t('pages.xray.pia.account')}</td>
                    <td>{piaData.accountHint || piaData.username}</td>
                  </tr>
                </tbody>
              </table>

              <Button
                loading={loading}
                type="primary"
                danger
                className="mt-8"
                onClick={() => void logout()}
              >
                {t('logout')}
              </Button>

              <Divider className="zero-margin">{t('pages.xray.warp.settings')}</Divider>

              <Form
                colon={false}
                labelCol={{ md: { span: 6 } }}
                wrapperCol={{ md: { span: 18 } }}
                className="mt-10"
              >
                <FormField
                  name="countryCode"
                  label={t('pages.xray.outbound.country')}
                  transform={{ input: (v) => v ?? undefined }}
                  onAfterChange={(v) => void fetchServers(v as string)}
                >
                  <Select
                    data-testid="pia-country-select"
                    showSearch={{ optionFilterProp: 'label' }}
                    options={countries.map((c) => {
                      const name = countryName(c.code, locale) || c.code;
                      const flag = countryFlag(c.code);
                      return {
                        value: c.code,
                        label: `${flag ? `${flag} ` : ''}${name} (${c.code})`,
                      };
                    })}
                  />
                </FormField>

                {regions.length > 0 && (
                  <FormField name="regionId" label={t('pages.xray.pia.region')}>
                    <Select
                      data-testid="pia-region-select"
                      showSearch={{ optionFilterProp: 'label' }}
                      options={[
                        { value: null, label: t('pages.xray.pia.allRegions') },
                        ...regions.map((r) => ({ value: r.id, label: r.name })),
                      ]}
                    />
                  </FormField>
                )}

                {filteredServers.length > 0 && (
                  <FormField name="hostname" label={t('pages.xray.outbound.server')}>
                    <Select
                      data-testid="pia-server-select"
                      showSearch={{ optionFilterProp: 'label' }}
                      options={filteredServers.map((s) => ({
                        value: s.hostname,
                        label: `${s.regionName} ${s.hostname} ${s.ip}`,
                      }))}
                    />
                  </FormField>
                )}
              </Form>

              <Button
                type="primary"
                className="mt-10"
                disabled={!hostname || selectedAlreadyAdded}
                loading={loading}
                onClick={() => void addOutbound()}
              >
                {t('pages.xray.warp.addOutbound')}
              </Button>
              {selectedAlreadyAdded && (
                <div className="pia-already-added">
                  {t('pages.xray.pia.alreadyAdded', { reset: t('reset') })}
                </div>
              )}

              {piaRows.length > 0 && (
                <>
                  <Divider className="my-10">{t('pages.xray.pia.addedServers')}</Divider>
                  <table className="pia-added-table" data-testid="pia-added-table">
                    <tbody>
                      {piaRows.map((row) => (
                        <tr key={`${row.index}-${row.tag}`}>
                          <td>{row.tag}</td>
                          <td>
                            <Button
                              type="primary"
                              danger
                              size="small"
                              loading={loading}
                              disabled={!row.tag}
                              data-testid={`pia-reset-${row.index}`}
                              onClick={() =>
                                void resetOutbound(row.index, row.hostname || row.tag || '')
                              }
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
