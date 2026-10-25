import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
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
  Popconfirm,
  Progress,
  Row,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import {
  CopyOutlined,
  DeleteOutlined,
  ReloadOutlined,
  SaveOutlined,
  ThunderboltOutlined,
  WifiOutlined,
} from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';

import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useHostsQuery } from '@/api/queries/useHostsQuery';
import { useHostMutations } from '@/api/queries/useHostMutations';
import { useAllSettings } from '@/api/queries/useAllSettings';
import AppSidebar from '@/layouts/AppSidebar';
import { HttpUtil } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import type { BulkAddHostValues } from '@/schemas/api/host';
import {
  ISP_SEP,
  groupPayload,
  hostAddress,
  ispGroupsOf,
  joinHost,
  mainGroups,
  probeIp,
  scanTargets,
} from './scan';
import './CleanIpPage.css';

interface CleanIpNetwork {
  ip: string;
  asn: number;
  org: string;
  country: string;
  isp: string;
}

interface ScanHit {
  ip: string;
  ms: number;
}

async function fetchNetwork(): Promise<CleanIpNetwork> {
  const msg = await HttpUtil.get('/panel/api/cleanip/network', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'network');
  return msg.obj as CleanIpNetwork;
}

export default function CleanIpPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);

  const network = useQuery({ queryKey: ['cleanip', 'network'], queryFn: fetchNetwork });
  const [ispTyped, setIsp] = useState<string | null>(null);
  const isp = ispTyped ?? network.data?.isp ?? '';

  const [perRange, setPerRange] = useState(8);
  const [timeoutMs, setTimeoutMs] = useState(3000);
  const [parallel, setParallel] = useState(8);
  const [running, setRunning] = useState(false);
  const [done, setDone] = useState(0);
  const [total, setTotal] = useState(0);
  const [hits, setHits] = useState<ScanHit[]>([]);
  const [selected, setSelected] = useState<string[]>([]);
  const stopRef = useRef(false);

  const startScan = useCallback(async () => {
    const targets = scanTargets(perRange);
    stopRef.current = false;
    setRunning(true);
    setDone(0);
    setTotal(targets.length);
    setHits([]);
    setSelected([]);
    const found: ScanHit[] = [];
    let next = 0;
    const worker = async () => {
      while (!stopRef.current && next < targets.length) {
        const ip = targets[next++];
        const r = await probeIp(ip, timeoutMs);
        if (stopRef.current) return;
        if (r.ok) {
          found.push({ ip, ms: r.ms });
          setHits([...found].sort((a, b) => a.ms - b.ms));
        }
        setDone((d) => d + 1);
      }
    };
    await Promise.all(Array.from({ length: parallel }, worker));
    setRunning(false);
    const best = [...found].sort((a, b) => a.ms - b.ms);
    setSelected(best.slice(0, 5).map((h) => h.ip));
  }, [perRange, timeoutMs, parallel]);

  const { hosts, refetch: refetchHosts } = useHostsQuery();
  const { bulkCreate, update, remove } = useHostMutations();
  const groups = useMemo(() => mainGroups(hosts), [hosts]);
  const [pickedGroupId, setGroupId] = useState<string>();
  const group = groups.find((g) => g.groupId === pickedGroupId) ?? groups[0];
  const groupId = group?.groupId;
  const ispGroups = useMemo(() => (group ? ispGroupsOf(group, hosts) : new Map()), [group, hosts]);
  const [portTyped, setPortTyped] = useState<{ groupId?: string; port: number } | null>(null);
  const port = portTyped && portTyped.groupId === groupId ? portTyped.port : group?.port || 443;
  const setPort = (value: number) => setPortTyped({ groupId, port: value });
  const [saving, setSaving] = useState(false);

  const { allSetting } = useAllSettings();
  const namesShowHost = String(allSetting.remarkTemplate ?? '').includes('{{HOST}}');

  const save = useCallback(async () => {
    const name = isp.trim();
    if (!group) return;
    if (!name) {
      messageApi.warning(t('pages.cleanIp.needIsp'));
      return;
    }
    if (!selected.length) {
      messageApi.warning(t('pages.cleanIp.needIps'));
      return;
    }
    const remark = `${group.remark ?? ''}${ISP_SEP}${name}`.slice(0, 256);
    const entries = selected.map((ip) => joinHost(ip, port));
    const existing = ispGroups.get(name);
    setSaving(true);
    try {
      const msg = existing
        ? await update(
            existing.groupId,
            groupPayload(existing, remark, entries) as unknown as BulkAddHostValues,
          )
        : await bulkCreate(groupPayload(group, remark, entries) as unknown as BulkAddHostValues);
      if (msg?.success) {
        messageApi.success(t('pages.cleanIp.saved', { count: entries.length, name: remark }));
        await refetchHosts();
      } else {
        messageApi.error(msg?.msg || t('somethingWentWrong'));
      }
    } finally {
      setSaving(false);
    }
  }, [isp, group, selected, port, ispGroups, update, bulkCreate, refetchHosts, messageApi, t]);

  const copyResults = useCallback(async () => {
    const name = isp.trim();
    const text = hits.map((h) => (name ? `${h.ip} ${name}` : h.ip)).join('\n');
    try {
      await navigator.clipboard.writeText(text);
      messageApi.success(t('pages.cleanIp.copied'));
    } catch {
      messageApi.error(t('somethingWentWrong'));
    }
  }, [hits, isp, messageApi, t]);

  const pageClass = ['clean-ip-page', isDark ? 'is-dark' : '', isUltra ? 'is-ultra' : '']
    .filter(Boolean)
    .join(' ');
  const net = network.data;
  const percent = total ? Math.round((done / total) * 100) : 0;

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Row gutter={[isMobile ? 8 : 16, isMobile ? 8 : 16]}>
              <Col span={24}>
                <Card size="small">
                  <Typography.Paragraph className="clean-ip-intro">
                    {t('pages.cleanIp.intro', { sep: ISP_SEP.trim() })}
                  </Typography.Paragraph>
                  <Alert type="info" showIcon title={t('pages.cleanIp.vpnHint')} />
                </Card>
              </Col>

              <Col xs={24} lg={8}>
                <Card
                  size="small"
                  title={
                    <Space>
                      <WifiOutlined aria-hidden />
                      {t('pages.cleanIp.network')}
                    </Space>
                  }
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
                  {network.isLoading ? (
                    <Typography.Text type="secondary">
                      {t('pages.cleanIp.detecting')}
                    </Typography.Text>
                  ) : network.isError ? (
                    <Alert type="warning" showIcon title={t('pages.cleanIp.toasts.network')} />
                  ) : net && !net.asn ? (
                    <Alert type="warning" showIcon title={t('pages.cleanIp.privateIp')} />
                  ) : net ? (
                    <div className="clean-ip-net">
                      <div className="clean-ip-net-isp">
                        {net.isp ? <Tag color="processing">{net.isp}</Tag> : null}
                        <span>{net.org}</span>
                      </div>
                      <Typography.Text type="secondary" className="clean-ip-mono">
                        AS{net.asn} · {net.country} · {net.ip}
                      </Typography.Text>
                      {!net.isp && (
                        <Alert type="warning" showIcon title={t('pages.cleanIp.noIsp')} />
                      )}
                    </div>
                  ) : null}
                  <label className="clean-ip-field">
                    <span>{t('pages.cleanIp.ispName')}</span>
                    <Input
                      value={isp}
                      maxLength={24}
                      placeholder="MCI"
                      onChange={(e) => setIsp(e.target.value.replace(/\s+/g, ''))}
                    />
                  </label>
                </Card>
              </Col>

              <Col xs={24} lg={16}>
                <Card
                  size="small"
                  title={
                    <Space>
                      <ThunderboltOutlined aria-hidden />
                      {t('pages.cleanIp.scan')}
                    </Space>
                  }
                >
                  <div className="clean-ip-settings">
                    <label className="clean-ip-field">
                      <span>{t('pages.cleanIp.perRange')}</span>
                      <InputNumber
                        min={1}
                        max={64}
                        value={perRange}
                        disabled={running}
                        onChange={(v) => setPerRange(Number(v) || 1)}
                      />
                    </label>
                    <label className="clean-ip-field">
                      <span>{t('pages.cleanIp.timeout')}</span>
                      <InputNumber
                        min={500}
                        max={10000}
                        step={500}
                        value={timeoutMs}
                        disabled={running}
                        onChange={(v) => setTimeoutMs(Number(v) || 3000)}
                      />
                    </label>
                    <label className="clean-ip-field">
                      <span>{t('pages.cleanIp.parallel')}</span>
                      <InputNumber
                        min={1}
                        max={32}
                        value={parallel}
                        disabled={running}
                        onChange={(v) => setParallel(Number(v) || 1)}
                      />
                    </label>
                    <div className="clean-ip-actions">
                      {running ? (
                        <Button danger onClick={() => (stopRef.current = true)}>
                          {t('pages.cleanIp.stop')}
                        </Button>
                      ) : (
                        <Button
                          type="primary"
                          icon={<ThunderboltOutlined aria-hidden />}
                          onClick={startScan}
                        >
                          {t('pages.cleanIp.start')}
                        </Button>
                      )}
                    </div>
                  </div>
                  {total > 0 && (
                    <div className="clean-ip-progress">
                      <Progress percent={percent} status={running ? 'active' : 'normal'} />
                      <Typography.Text type="secondary">
                        {t('pages.cleanIp.progress', { done, total, ok: hits.length })}
                      </Typography.Text>
                    </div>
                  )}
                  <Typography.Paragraph type="secondary" className="clean-ip-note">
                    {t('pages.cleanIp.quickNote')}
                  </Typography.Paragraph>
                </Card>
              </Col>

              <Col xs={24} lg={12}>
                <Card
                  size="small"
                  title={t('pages.cleanIp.results')}
                  extra={
                    <Button
                      size="small"
                      icon={<CopyOutlined aria-hidden />}
                      disabled={!hits.length}
                      onClick={copyResults}
                    >
                      {t('pages.cleanIp.copy')}
                    </Button>
                  }
                >
                  <Table
                    size="small"
                    rowKey="ip"
                    dataSource={hits}
                    pagination={hits.length > 50 ? { pageSize: 50 } : false}
                    locale={{ emptyText: <Empty description={t('pages.cleanIp.empty')} /> }}
                    rowSelection={{
                      selectedRowKeys: selected,
                      onChange: (keys) => setSelected(keys as string[]),
                    }}
                    columns={[
                      {
                        title: t('pages.cleanIp.ip'),
                        dataIndex: 'ip',
                        render: (ip: string) => <span className="clean-ip-mono">{ip}</span>,
                      },
                      {
                        title: t('pages.cleanIp.latency'),
                        dataIndex: 'ms',
                        width: 120,
                        render: (ms: number) => `${ms} ms`,
                      },
                    ]}
                  />
                </Card>
              </Col>

              <Col xs={24} lg={12}>
                <Card
                  size="small"
                  title={
                    <Space>
                      <SaveOutlined aria-hidden />
                      {t('pages.cleanIp.save')}
                    </Space>
                  }
                >
                  {!groups.length ? (
                    <Alert type="info" showIcon title={t('pages.cleanIp.noGroups')} />
                  ) : (
                    <>
                      <div className="clean-ip-settings">
                        <label className="clean-ip-field clean-ip-grow">
                          <span>{t('pages.cleanIp.group')}</span>
                          <Select
                            value={groupId}
                            onChange={setGroupId}
                            options={groups.map((g) => ({ value: g.groupId, label: g.remark }))}
                          />
                        </label>
                        <label className="clean-ip-field">
                          <span>{t('pages.cleanIp.port')}</span>
                          <InputNumber
                            min={1}
                            max={65535}
                            value={port}
                            onChange={(v) => setPort(Number(v) || 443)}
                          />
                        </label>
                        <div className="clean-ip-actions">
                          <Button
                            type="primary"
                            icon={<SaveOutlined aria-hidden />}
                            loading={saving}
                            disabled={!selected.length || !isp.trim()}
                            onClick={save}
                          >
                            {t('pages.cleanIp.saveFor', {
                              count: selected.length,
                              isp: isp.trim() || '?',
                            })}
                          </Button>
                        </div>
                      </div>
                      {!namesShowHost && (
                        <Alert
                          className="clean-ip-note"
                          type="warning"
                          showIcon
                          title={t('pages.cleanIp.templateHint', { tag: '{{HOST}}' })}
                        />
                      )}
                      <Typography.Title level={5} className="clean-ip-subtitle">
                        {t('pages.cleanIp.ispGroups')}
                      </Typography.Title>
                      {ispGroups.size === 0 ? (
                        <Typography.Text type="secondary">
                          {t('pages.cleanIp.noIspGroups')}
                        </Typography.Text>
                      ) : (
                        <div className="clean-ip-isp-list">
                          {[...ispGroups.entries()].map(([name, g]) => (
                            <div key={g.groupId} className="clean-ip-isp-row">
                              <Tag color="processing">{name}</Tag>
                              <span className="clean-ip-mono clean-ip-isp-ips">
                                {g.hosts.map(hostAddress).join('  ')}
                              </span>
                              <Popconfirm
                                title={t('pages.cleanIp.removeConfirm', { name })}
                                okText={t('delete')}
                                cancelText={t('cancel')}
                                onConfirm={async () => {
                                  const msg = await remove(g.groupId);
                                  if (msg?.success) await refetchHosts();
                                }}
                              >
                                <Button
                                  size="small"
                                  type="text"
                                  danger
                                  icon={<DeleteOutlined aria-hidden />}
                                  aria-label={t('delete')}
                                />
                              </Popconfirm>
                            </div>
                          ))}
                        </div>
                      )}
                    </>
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
