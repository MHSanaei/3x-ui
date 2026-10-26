import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Card,
  Col,
  ConfigProvider,
  Empty,
  Input,
  InputNumber,
  Layout,
  Popover,
  QRCode,
  Row,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import {
  CodeOutlined,
  CloudDownloadOutlined,
  CopyOutlined,
  DeleteOutlined,
  PlusOutlined,
  QrcodeOutlined,
  ReloadOutlined,
  SaveOutlined,
} from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';

import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import AppSidebar from '@/layouts/AppSidebar';
import { HttpUtil } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import { NetworkInfo, StepTitle, fetchNetwork } from '@/pages/clean-ip/common';
import { mergeDetected, whiteDnsLink, resolverList, type DnsTunnelProfile } from './links';
import '@/pages/clean-ip/CleanIpPage.css';

interface ResolverResult {
  ip: string;
  status: string;
  ms: number;
  name: string;
}

interface ResolverReport {
  at: number;
  network: { ip: string; asn: number; org: string; isp: string };
  zone: string;
  results: ResolverResult[];
}

interface TunnelCommand {
  token: string;
  expiresAt: number;
  unix: string;
  windows: string;
}

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };
const EMPTY_PROFILE: DnsTunnelProfile = {
  name: '',
  engine: 'stormdns',
  domain: '',
  key: '',
  method: 1,
};

async function fetchProfiles(): Promise<DnsTunnelProfile[]> {
  const msg = await HttpUtil.get('/panel/api/dnstunnel/profiles', undefined, {
    silent: true,
  });
  if (!msg?.success) throw new Error(msg?.msg || 'profiles');
  return (msg.obj as DnsTunnelProfile[] | null) ?? [];
}

export default function DnsTunnelPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);

  const copy = useCallback(
    async (text: string) => {
      try {
        await navigator.clipboard.writeText(text);
        messageApi.success(t('pages.cleanIp.copied'));
      } catch {
        messageApi.error(t('somethingWentWrong'));
      }
    },
    [messageApi, t],
  );

  // 1. the visitor's internet
  const network = useQuery({
    queryKey: ['cleanip', 'network'],
    queryFn: fetchNetwork,
  });

  // 2. tunnel servers
  const saved = useQuery({
    queryKey: ['dnstunnel', 'profiles'],
    queryFn: fetchProfiles,
  });
  const [draft, setDraft] = useState<DnsTunnelProfile[] | null>(null);
  const profiles = useMemo(() => draft ?? saved.data ?? [], [draft, saved.data]);
  const [saving, setSaving] = useState(false);
  const edit = (i: number, patch: Partial<DnsTunnelProfile>) =>
    setDraft(profiles.map((p, j) => (j === i ? { ...p, ...patch } : p)));
  const saveProfiles = useCallback(async () => {
    setSaving(true);
    try {
      const msg = await HttpUtil.post('/panel/api/dnstunnel/profiles', profiles, JSON_HEADERS);
      if (msg?.success) {
        messageApi.success(t('pages.dnsTunnel.toasts.saved'));
        setDraft(null);
        await saved.refetch();
      } else {
        messageApi.error(msg?.msg || t('pages.dnsTunnel.toasts.profiles'));
      }
    } finally {
      setSaving(false);
    }
  }, [profiles, saved, messageApi, t]);
  const [stormDir, setStormDir] = useState('/opt/stormdns');
  const [detecting, setDetecting] = useState(false);
  const [notInstalledIn, setNotInstalledIn] = useState('');
  const detect = useCallback(async () => {
    setDetecting(true);
    setNotInstalledIn('');
    try {
      const msg = await HttpUtil.get(
        '/panel/api/dnstunnel/detect',
        { dir: stormDir },
        { silent: true },
      );
      const found = (msg?.obj as DnsTunnelProfile[] | null) ?? [];
      if (msg?.success && found.length === 0) {
        setNotInstalledIn(stormDir);
      } else if (msg?.success) {
        setDraft(mergeDetected(profiles, found));
        messageApi.success(t('pages.dnsTunnel.detected'));
      } else {
        messageApi.error(msg?.msg || t('pages.dnsTunnel.toasts.detect'));
      }
    } finally {
      setDetecting(false);
    }
  }, [stormDir, profiles, messageApi, t]);

  // 3. resolver test (a one-day command run on a device on the ISP)
  const zones = useMemo(() => [...new Set((saved.data ?? []).map((p) => p.domain))], [saved.data]);
  const [zonePicked, setZone] = useState<string>();
  const zone = zonePicked ?? zones[0];
  const [extraResolvers, setExtraResolvers] = useState('');
  const [command, setCommand] = useState<TunnelCommand | null>(null);
  const [making, setMaking] = useState(false);
  const makeCommand = useCallback(async () => {
    if (!zone) return;
    setMaking(true);
    try {
      const resolvers = extraResolvers.split(/[\s,;]+/).filter(Boolean);
      const msg = await HttpUtil.post(
        '/panel/api/dnstunnel/command',
        { zone, resolvers },
        JSON_HEADERS,
      );
      if (msg?.success) setCommand(msg.obj as TunnelCommand);
      else messageApi.error(msg?.msg || t('pages.cleanIp.toasts.command'));
    } finally {
      setMaking(false);
    }
  }, [zone, extraResolvers, messageApi, t]);
  const reports = useQuery({
    queryKey: ['dnstunnel', 'reports', command?.token],
    enabled: !!command,
    refetchInterval: 5000,
    queryFn: async () => {
      const msg = await HttpUtil.get(`/panel/api/dnstunnel/command/${command?.token}`, undefined, {
        silent: true,
      });
      if (!msg?.success) throw new Error(msg?.msg || 'expired');
      return (msg.obj as ResolverReport[] | null) ?? [];
    },
  });
  const newestFirst = useMemo(() => [...(reports.data ?? [])].reverse(), [reports.data]);

  const pageClass = ['clean-ip-page', isDark ? 'is-dark' : '', isUltra ? 'is-ultra' : '']
    .filter(Boolean)
    .join(' ');
  const gutter: [number, number] = [isMobile ? 8 : 16, isMobile ? 8 : 16];
  const statusTag = (status: string) =>
    status === 'ok' ? (
      <Tag color="success">{t('pages.dnsTunnel.resolverOk')}</Tag>
    ) : (
      <Tag color={status === 'timeout' ? 'default' : 'error'}>{status}</Tag>
    );

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Row gutter={gutter}>
              <Col span={24}>
                <Card size="small">
                  <Typography.Paragraph className="clean-ip-intro">
                    {t('pages.dnsTunnel.intro')}
                  </Typography.Paragraph>
                  <Alert type="info" showIcon title={t('pages.cleanIp.vpnHint')} />
                </Card>
              </Col>

              <Col span={24}>
                <Card
                  size="small"
                  title={
                    <StepTitle
                      n={1}
                      extra={
                        <Button
                          size="small"
                          type="text"
                          icon={<ReloadOutlined aria-hidden />}
                          loading={network.isFetching}
                          aria-label={t('refresh')}
                          onClick={() => network.refetch()}
                        />
                      }
                    >
                      {t('pages.cleanIp.network')}
                    </StepTitle>
                  }
                >
                  <NetworkInfo
                    net={network.data}
                    loading={network.isLoading}
                    error={network.isError}
                  />
                </Card>
              </Col>

              <Col span={24}>
                <Card
                  size="small"
                  title={<StepTitle n={2}>{t('pages.dnsTunnel.servers')}</StepTitle>}
                >
                  <Typography.Paragraph type="secondary">
                    {t('pages.dnsTunnel.serversHint')}
                  </Typography.Paragraph>
                  {profiles.length === 0 && <Empty description={t('pages.dnsTunnel.noServers')} />}
                  <div className="dns-tunnel-profiles">
                    {profiles.map((p, i) => (
                      <div key={i} className="dns-tunnel-profile">
                        <div className="clean-ip-settings">
                          <label className="clean-ip-field">
                            <span>{t('pages.dnsTunnel.name')}</span>
                            <Input
                              value={p.name}
                              maxLength={40}
                              onChange={(e) => edit(i, { name: e.target.value })}
                            />
                          </label>
                          <label className="clean-ip-field">
                            <span>{t('pages.dnsTunnel.engine')}</span>
                            <Select
                              value={p.engine}
                              onChange={(engine) => edit(i, { engine })}
                              options={[
                                {
                                  value: 'stormdns',
                                  label: 'StormDNS (WhiteDNS)',
                                },
                                { value: 'dnstt', label: 'dnstt' },
                              ]}
                            />
                          </label>
                          <label className="clean-ip-field clean-ip-grow">
                            <span>{t('pages.cleanIp.domain')}</span>
                            <Input
                              className="clean-ip-mono"
                              value={p.domain}
                              placeholder="t.example.com"
                              onChange={(e) => edit(i, { domain: e.target.value.trim() })}
                            />
                          </label>
                          <label className="clean-ip-field clean-ip-grow">
                            <span>{t('pages.dnsTunnel.key')}</span>
                            <Input
                              className="clean-ip-mono"
                              value={p.key}
                              onChange={(e) => edit(i, { key: e.target.value.trim() })}
                            />
                          </label>
                          {p.engine === 'stormdns' && (
                            <label className="clean-ip-field">
                              <span>{t('pages.dnsTunnel.method')}</span>
                              <InputNumber
                                min={0}
                                max={5}
                                value={p.method}
                                onChange={(v) => edit(i, { method: Number(v) || 0 })}
                              />
                            </label>
                          )}
                          <Button
                            danger
                            type="text"
                            icon={<DeleteOutlined aria-hidden />}
                            aria-label={t('delete')}
                            onClick={() => setDraft(profiles.filter((_, j) => j !== i))}
                          />
                        </div>
                        {p.engine === 'stormdns' && p.domain && p.key && (
                          <div className="dns-tunnel-link">
                            <Typography.Text type="secondary">
                              {t('pages.dnsTunnel.whiteDnsLink')}
                            </Typography.Text>
                            <Space wrap>
                              <Button
                                size="small"
                                icon={<CopyOutlined aria-hidden />}
                                onClick={() => copy(whiteDnsLink(p))}
                              >
                                {t('pages.cleanIp.copy')}
                              </Button>
                              <Popover
                                trigger="click"
                                content={
                                  <QRCode value={whiteDnsLink(p)} size={240} bordered={false} />
                                }
                              >
                                <Button size="small" icon={<QrcodeOutlined aria-hidden />}>
                                  QR
                                </Button>
                              </Popover>
                            </Space>
                          </div>
                        )}
                        {p.engine === 'dnstt' && (
                          <Typography.Paragraph type="secondary" className="clean-ip-note">
                            {t('pages.dnsTunnel.dnsttHint')}
                          </Typography.Paragraph>
                        )}
                      </div>
                    ))}
                  </div>
                  <div className="clean-ip-settings clean-ip-note">
                    <Button
                      icon={<PlusOutlined aria-hidden />}
                      onClick={() => setDraft([...profiles, { ...EMPTY_PROFILE }])}
                    >
                      {t('pages.dnsTunnel.addServer')}
                    </Button>
                    <Space.Compact>
                      <Input
                        className="clean-ip-mono"
                        value={stormDir}
                        aria-label={t('pages.dnsTunnel.stormDir')}
                        onChange={(e) => setStormDir(e.target.value.trim())}
                      />
                      <Button
                        icon={<CloudDownloadOutlined aria-hidden />}
                        loading={detecting}
                        onClick={detect}
                      >
                        {t('pages.dnsTunnel.detect')}
                      </Button>
                    </Space.Compact>
                    <div className="clean-ip-actions">
                      <Button
                        type="primary"
                        icon={<SaveOutlined aria-hidden />}
                        loading={saving}
                        disabled={draft === null}
                        onClick={saveProfiles}
                      >
                        {t('pages.dnsTunnel.saveServers')}
                      </Button>
                    </div>
                  </div>
                  {notInstalledIn && (
                    <Alert
                      className="clean-ip-note"
                      type="warning"
                      showIcon
                      closable
                      onClose={() => setNotInstalledIn('')}
                      title={t('pages.dnsTunnel.notInstalled', { dir: notInstalledIn })}
                    />
                  )}
                </Card>
              </Col>

              <Col span={24}>
                <Card
                  size="small"
                  title={<StepTitle n={3}>{t('pages.dnsTunnel.resolverTest')}</StepTitle>}
                >
                  <Typography.Paragraph type="secondary">
                    {t('pages.dnsTunnel.resolverIntro')}
                  </Typography.Paragraph>
                  {!zones.length ? (
                    <Alert type="info" showIcon title={t('pages.dnsTunnel.needServer')} />
                  ) : (
                    <div className="clean-ip-settings">
                      <label className="clean-ip-field clean-ip-grow">
                        <span>{t('pages.cleanIp.domain')}</span>
                        <Select
                          value={zone}
                          onChange={setZone}
                          options={zones.map((z) => ({ value: z, label: z }))}
                        />
                      </label>
                      <label className="clean-ip-field clean-ip-grow">
                        <span>{t('pages.dnsTunnel.extraResolvers')}</span>
                        <Input
                          className="clean-ip-mono"
                          value={extraResolvers}
                          placeholder="185.51.200.2 78.157.42.100"
                          onChange={(e) => setExtraResolvers(e.target.value)}
                        />
                      </label>
                      <div className="clean-ip-actions">
                        <Button
                          icon={<CodeOutlined aria-hidden />}
                          loading={making}
                          onClick={makeCommand}
                        >
                          {t('pages.cleanIp.makeCommand')}
                        </Button>
                      </div>
                    </div>
                  )}
                  {command && (
                    <div className="clean-ip-command">
                      <Typography.Text type="secondary">
                        {t('pages.cleanIp.unixCmd')}
                      </Typography.Text>
                      <Typography.Paragraph copyable className="clean-ip-mono clean-ip-code">
                        {command.unix}
                      </Typography.Paragraph>
                      <Typography.Text type="secondary">
                        {t('pages.cleanIp.windowsCmd')}
                      </Typography.Text>
                      <Typography.Paragraph copyable className="clean-ip-mono clean-ip-code">
                        {command.windows}
                      </Typography.Paragraph>
                      {reports.isError ? (
                        <Alert type="warning" showIcon title={t('pages.cleanIp.toasts.expired')} />
                      ) : newestFirst.length === 0 ? (
                        <Typography.Paragraph type="secondary">
                          {t('pages.cleanIp.waiting')}
                        </Typography.Paragraph>
                      ) : (
                        newestFirst.map((r) => {
                          const ok = r.results.filter((x) => x.status === 'ok').map((x) => x.ip);
                          return (
                            <div key={r.at} className="clean-ip-report">
                              <div className="clean-ip-report-head">
                                {r.network.isp ? (
                                  <Tag color="processing">{r.network.isp}</Tag>
                                ) : null}
                                <span>
                                  {t('pages.dnsTunnel.reportLine', {
                                    org: r.network.org || r.network.ip,
                                    ok: ok.length,
                                    total: r.results.length,
                                  })}
                                </span>
                                <Button
                                  size="small"
                                  icon={<CopyOutlined aria-hidden />}
                                  disabled={!ok.length}
                                  onClick={() => copy(resolverList(ok))}
                                >
                                  {t('pages.dnsTunnel.copyResolvers')}
                                </Button>
                              </div>
                              <Table
                                size="small"
                                rowKey="ip"
                                dataSource={r.results}
                                pagination={r.results.length > 20 ? { pageSize: 20 } : false}
                                columns={[
                                  {
                                    title: t('pages.dnsTunnel.resolver'),
                                    dataIndex: 'ip',
                                    render: (ip: string) => (
                                      <span className="clean-ip-mono">{ip}</span>
                                    ),
                                  },
                                  {
                                    title: t('pages.dnsTunnel.resolverName'),
                                    dataIndex: 'name',
                                  },
                                  {
                                    title: t('pages.cleanIp.latency'),
                                    dataIndex: 'ms',
                                    width: 100,
                                    render: (ms: number) => `${ms} ms`,
                                  },
                                  {
                                    title: t('pages.cleanIp.serverCheck'),
                                    dataIndex: 'status',
                                    width: 120,
                                    render: statusTag,
                                  },
                                ]}
                              />
                            </div>
                          );
                        })
                      )}
                    </div>
                  )}
                </Card>
              </Col>
            </Row>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
