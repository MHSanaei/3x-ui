import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Card,
  Col,
  Collapse,
  ConfigProvider,
  Empty,
  Input,
  InputNumber,
  Layout,
  Popconfirm,
  Progress,
  Row,
  Segmented,
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
  DownloadOutlined,
  ReloadOutlined,
  SaveOutlined,
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
import type { BulkAddHostValues, HostRecord } from '@/schemas/api/host';
import {
  ISP_SEP,
  groupDomain,
  groupPayload,
  hostAddress,
  ispGroupsOf,
  joinHost,
  defaultSelection,
  mainGroups,
  nearbyIps,
  parseTargets,
  probeIp,
  resultsCsv,
  scanRanges,
  scanTargets,
  withFragment,
  type FragmentSetting,
} from './scan';
import FragmentStep, { fragmentKey, type CleanIpCommand, type CleanIpReport } from './FragmentStep';
import { CLOUDFLARE_ASNS, StepTitle, fetchCdns, fetchNetwork } from './common';
import { coloPlace, flagOf } from './colos';
import RangePicker from './RangePicker';
import { reportFromOtherIsp, resultRows, saveIsp, type IpRow } from './report';
import './CleanIpPage.css';

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };
// the server checks at most this many per request (cleanIpVerifyMax)
const VERIFY_MAX = 50;
const TICKED_KEY = 'cleanIp.tickedRanges';

function readTickedRanges(): Record<string, string[]> {
  try {
    const saved = JSON.parse(localStorage.getItem(TICKED_KEY) || '{}');
    return saved && typeof saved === 'object' ? saved : {};
  } catch {
    return {};
  }
}

function isIpv4(value: string): boolean {
  return /^\d{1,3}(\.\d{1,3}){3}$/.test(value);
}

function VerdictTag({ status }: { status?: string }) {
  const { t } = useTranslation();
  if (status === 'ok') return <Tag color="success">{t('pages.cleanIp.verdictOk')}</Tag>;
  if (status === 'restricted')
    return <Tag color="error">{t('pages.cleanIp.verdictRestricted')}</Tag>;
  if (status === 'unreachable') return <Tag>{t('pages.cleanIp.verdictUnreachable')}</Tag>;
  if (status === 'skipped') return <Tag>{t('pages.cleanIp.verdictSkipped')}</Tag>;
  if (status === 'limited') return <Tag color="warning">{t('pages.cleanIp.verdictLimited')}</Tag>;
  return <Typography.Text type="secondary">—</Typography.Text>;
}

function SavedIps({ ips, verdicts }: { ips: string[]; verdicts: Record<string, string> }) {
  return (
    <>
      {ips.map((ip) => (
        <span key={ip} className={'clean-ip-saved clean-ip-saved-' + (verdicts[ip] ?? 'unknown')}>
          {ip}
        </span>
      ))}
    </>
  );
}

export default function CleanIpPage() {
  const { t, i18n } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);

  // 1. the visitor's internet
  const network = useQuery({ queryKey: ['cleanip', 'network'], queryFn: fetchNetwork });
  const net = network.data;
  const [ispTyped, setIsp] = useState<string | null>(null);
  const viaCloudflare = !!net && CLOUDFLARE_ASNS.has(net.asn);

  // 2. the CDN domain: a host group and the domain its configs present to Cloudflare
  const { hosts, refetch: refetchHosts } = useHostsQuery();
  const { bulkCreate, update, remove } = useHostMutations();
  const groups = useMemo(() => mainGroups(hosts), [hosts]);
  const [pickedGroupId, setGroupId] = useState<string>();
  const group = groups.find((g) => g.groupId === pickedGroupId) ?? groups[0];
  const groupId = group?.groupId;
  const ispGroups = useMemo(
    () => (group ? ispGroupsOf(group, hosts) : new Map<string, HostRecord>()),
    [group, hosts],
  );
  const [portTyped, setPortTyped] = useState<{ groupId?: string; port: number } | null>(null);
  const port = portTyped && portTyped.groupId === groupId ? portTyped.port : group?.port || 443;
  const [domainTyped, setDomainTyped] = useState<{ groupId?: string; domain: string } | null>(null);
  const domain =
    domainTyped && domainTyped.groupId === groupId
      ? domainTyped.domain
      : group
        ? groupDomain(group)
        : '';
  const { allSetting } = useAllSettings();
  const namesShowHost = String(allSetting.remarkTemplate ?? '').includes('{{HOST}}');

  // 3. fragment and upload test (a command run on a device on the ISP)
  const [size, setSize] = useState(5000);
  const [cmdPerRange, setCmdPerRange] = useState(8);
  const [command, setCommand] = useState<CleanIpCommand | null>(null);
  const [sentIps, setSentIps] = useState<string[]>([]);
  const [making, setMaking] = useState(false);
  const reports = useQuery({
    queryKey: ['cleanip', 'reports', command?.token],
    enabled: !!command,
    refetchInterval: 5000,
    queryFn: async () => {
      const msg = await HttpUtil.get(`/panel/api/cleanip/command/${command?.token}`, undefined, {
        silent: true,
      });
      if (!msg?.success) throw new Error(msg?.msg || 'expired');
      return (msg.obj as CleanIpReport[] | null) ?? [];
    },
  });
  const report = reports.data?.length ? reports.data[reports.data.length - 1] : undefined;
  const browserAsn = net?.asn ?? 0;
  const otherIspReport = reportFromOtherIsp(report, browserAsn);
  const isp = saveIsp({ browserIsp: net?.isp ?? '', browserAsn, report, typedIsp: ispTyped });
  // null = no fragment; undefined = the group's current fragment stays as it is
  const [fragPick, setFragPick] = useState<{ at: number; key: string } | null>(null);
  const suggestedKey = useMemo(() => {
    if (!report) return '';
    const none = report.fragments.find((f) => !f.packets);
    const best = report.fragments[0];
    return best && best.packets && best.ok > (none?.ok ?? 0) ? fragmentKey(best) : '';
  }, [report]);
  const fragKey = report ? (fragPick?.at === report.at ? fragPick.key : suggestedKey) : '';
  const fragment: FragmentSetting | null | undefined = report
    ? (report.fragments.find((f) => f.packets && fragmentKey(f) === fragKey) ?? null)
    : undefined;

  // the CDN the host group's domain is behind: its edge ranges are what both tests sample
  const cdns = useQuery({ queryKey: ['cleanip', 'cdns'], queryFn: fetchCdns });
  const [cdnId, setCdnId] = useState('cloudflare');
  const cdn = cdns.data?.find((c) => c.id === cdnId);
  const [tickedByCdn, setTickedByCdn] = useState<Record<string, string[]>>(readTickedRanges);
  const [extraRanges, setExtraRanges] = useState('');
  const ticked = useMemo(
    () => (cdn ? (tickedByCdn[cdn.id] ?? defaultSelection(cdn)) : []),
    [cdn, tickedByCdn],
  );
  const cdnRanges = useMemo(() => scanRanges(ticked, extraRanges), [ticked, extraRanges]);
  const setTicked = (ranges: string[]) => {
    const next = { ...tickedByCdn, [cdnId]: ranges };
    setTickedByCdn(next);
    try {
      localStorage.setItem(TICKED_KEY, JSON.stringify(next));
    } catch {
      // private mode: the choice lasts for this visit only
    }
  };

  // 4. quick scan in this browser
  const [perRange, setPerRange] = useState(8);
  const [timeoutMs, setTimeoutMs] = useState(3000);
  const [parallel, setParallel] = useState(8);
  const [running, setRunning] = useState(false);
  const [done, setDone] = useState(0);
  const [total, setTotal] = useState(0);
  const [hits, setHits] = useState<{ ip: string; ms: number }[]>([]);
  const stopRef = useRef(false);
  const testedRef = useRef(new Set<string>());
  const [source, setSource] = useState<'ranges' | 'list'>('ranges');
  const [listText, setListText] = useState('');
  const [picked, setPicked] = useState<string[] | null>(null);

  const rows = useMemo(
    () => resultRows(report, hits, sentIps, browserAsn),
    [report, hits, sentIps, browserAsn],
  );
  const placeOf = useCallback(
    (colo: string) => coloPlace(colo, i18n.language || 'en'),
    [i18n.language],
  );
  const countries = useMemo(() => {
    const seen = new Map<string, string>();
    for (const r of rows) {
      const place = placeOf(r.colo);
      if (place) seen.set(place.country, place.countryName);
    }
    return [...seen].map(([value, name]) => ({ value, label: `${flagOf(value)} ${name}` }));
  }, [rows, placeOf]);
  const [country, setCountry] = useState<string>();
  const shown = useMemo(
    () => (country ? rows.filter((r) => placeOf(r.colo)?.country === country) : rows),
    [rows, country, placeOf],
  );
  // What the panel's server saw on each IP: the browser only sees TLS, which every edge answers.
  const [verdicts, setVerdicts] = useState<Record<string, string>>({});
  const [verifying, setVerifying] = useState(false);
  const verifyAbort = useRef<AbortController | null>(null);
  useEffect(() => () => verifyAbort.current?.abort(), []);
  // Only on the admin's request, a short list at a time: many outgoing connections from the
  // panel's server can read as scanning to its hosting provider.
  const verifyIps = useCallback(
    async (ips: string[]) => {
      if (!domain || !ips.length) return {};
      const abort = new AbortController();
      verifyAbort.current = abort;
      setVerifying(true);
      try {
        const msg = await HttpUtil.post(
          '/panel/api/cleanip/verify',
          { host: domain, cdn: cdnId, ips: ips.slice(0, VERIFY_MAX) },
          { ...JSON_HEADERS, signal: abort.signal, silent: true },
        );
        if (abort.signal.aborted) return {};
        if (!msg?.success) {
          messageApi.error(msg?.msg || t('pages.cleanIp.toasts.verify'));
          return {};
        }
        const got: Record<string, string> = {};
        for (const v of (msg.obj as { ip: string; status: string }[] | null) ?? [])
          got[v.ip] = v.status;
        setVerdicts((prev) => ({ ...prev, ...got }));
        if (Object.values(got).includes('limited'))
          messageApi.warning(t('pages.cleanIp.toasts.verifyLimited'));
        return got;
      } finally {
        if (verifyAbort.current === abort) verifyAbort.current = null;
        setVerifying(false);
      }
    },
    [domain, cdnId, messageApi, t],
  );
  const selected =
    picked ??
    shown
      .filter((r) => verdicts[r.ip] !== 'restricted' && r.test !== 'failed')
      .slice(0, 5)
      .map((r) => r.ip);

  const runScan = useCallback(
    async (targets: string[], keep: boolean) => {
      stopRef.current = false;
      setRunning(true);
      if (!keep) {
        testedRef.current = new Set();
        setDone(0);
        setTotal(0);
        setHits([]);
        setPicked(null);
      }
      setTotal((n) => n + targets.length);
      const found: { ip: string; ms: number }[] = keep ? [...hits] : [];
      let next = 0;
      const worker = async () => {
        while (!stopRef.current && next < targets.length) {
          const ip = targets[next++];
          testedRef.current.add(ip);
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
    },
    [hits, timeoutMs, parallel],
  );
  const startScan = useCallback(() => {
    const targets =
      source === 'list' ? parseTargets(listText, perRange) : scanTargets(cdnRanges, perRange);
    if (!targets.length) {
      messageApi.warning(t('pages.cleanIp.noTargets'));
      return;
    }
    void runScan(targets, false);
  }, [source, listText, perRange, cdnRanges, runScan, messageApi, t]);
  const findMore = useCallback(() => {
    const targets = nearbyIps(
      hits.map((h) => h.ip),
      testedRef.current,
      8,
    );
    if (targets.length) void runScan(targets, true);
  }, [hits, runScan]);

  const makeCommand = useCallback(async () => {
    setMaking(true);
    const ips = hits.slice(0, 100).map((h) => h.ip);
    try {
      const msg = await HttpUtil.post(
        '/panel/api/cleanip/command',
        {
          host: domain,
          path: group?.path || '/',
          size,
          perRange: cmdPerRange,
          ips,
          cdn: cdnId,
          ranges: cdnRanges,
        },
        JSON_HEADERS,
      );
      if (msg?.success) {
        setCommand(msg.obj as CleanIpCommand);
        setSentIps(ips);
      } else messageApi.error(msg?.msg || t('pages.cleanIp.toasts.command'));
    } finally {
      setMaking(false);
    }
  }, [domain, group?.path, size, cmdPerRange, hits, cdnId, cdnRanges, messageApi, t]);

  // 5. save for the ISP
  const [saving, setSaving] = useState(false);
  const targetName = group ? `${group.remark ?? ''}${ISP_SEP}${isp || '?'}` : '';
  const fragmentLabel = (f: FragmentSetting | null | undefined) =>
    f === undefined
      ? t('pages.cleanIp.keepFragment')
      : f
        ? `${f.packets === 'tlshello' ? 'tlshello' : 'TCP'} · ${f.length} · ${f.delay} ms`
        : t('pages.cleanIp.noFragment');

  const savedGroups = useMemo(
    () => (group ? [group, ...ispGroups.values()] : []),
    [group, ispGroups],
  );
  const mainIps = useMemo(
    () => (group ? group.hosts.map(hostAddress).filter(isIpv4) : []),
    [group],
  );
  const restrictedSaved = useMemo(
    () =>
      [...new Set(savedGroups.flatMap((g) => g.hosts.map(hostAddress)))].filter(
        (ip) => verdicts[ip] === 'restricted',
      ),
    [savedGroups, verdicts],
  );
  const savedIps = useMemo(
    () =>
      [...new Set(savedGroups.flatMap((g) => g.hosts.map(hostAddress)))]
        .filter(isIpv4)
        .slice(0, VERIFY_MAX),
    [savedGroups],
  );
  const checkSaved = useCallback(async () => {
    const got = await verifyIps(savedIps);
    const bad = Object.values(got).filter((v) => v === 'restricted').length;
    const count = Object.keys(got).length;
    if (count) messageApi.info(t('pages.cleanIp.checkedSaved', { count, bad }));
  }, [savedIps, verifyIps, messageApi, t]);
  const removeRestricted = useCallback(async () => {
    setSaving(true);
    try {
      for (const g of savedGroups) {
        const kept = g.hosts.filter((h) => verdicts[hostAddress(h)] !== 'restricted');
        if (kept.length === g.hosts.length) continue;
        const values = groupPayload(g, g.remark ?? '', kept) as unknown as BulkAddHostValues;
        // an ISP group left with no IP has no reason to exist; the main group always stays
        const msg =
          kept.length || g === group ? await update(g.groupId, values) : await remove(g.groupId);
        if (!msg?.success) {
          messageApi.error(msg?.msg || t('somethingWentWrong'));
          return;
        }
      }
      messageApi.success(t('pages.cleanIp.removedRestricted', { count: restrictedSaved.length }));
      await refetchHosts();
    } finally {
      setSaving(false);
    }
  }, [
    savedGroups,
    verdicts,
    group,
    update,
    remove,
    refetchHosts,
    restrictedSaved.length,
    messageApi,
    t,
  ]);

  const save = useCallback(async () => {
    if (!group) return;
    if (!isp) {
      messageApi.warning(t('pages.cleanIp.needIsp'));
      return;
    }
    if (!selected.length) {
      messageApi.warning(t('pages.cleanIp.needIps'));
      return;
    }
    const remark = `${group.remark ?? ''}${ISP_SEP}${isp}`.slice(0, 256);
    const entries = selected.map((ip) => joinHost(ip, port));
    const existing = ispGroups.get(isp);
    const payload = groupPayload(existing ?? group, remark, entries);
    if (fragment !== undefined) payload.finalMask = withFragment(payload.finalMask, fragment);
    setSaving(true);
    try {
      const values = payload as unknown as BulkAddHostValues;
      const msg = existing ? await update(existing.groupId, values) : await bulkCreate(values);
      if (msg?.success) {
        messageApi.success(t('pages.cleanIp.saved', { count: entries.length, name: remark }));
        await refetchHosts();
      } else {
        messageApi.error(msg?.msg || t('somethingWentWrong'));
      }
    } finally {
      setSaving(false);
    }
  }, [
    group,
    isp,
    selected,
    port,
    ispGroups,
    fragment,
    update,
    bulkCreate,
    refetchHosts,
    messageApi,
    t,
  ]);

  const downloadCsv = useCallback(() => {
    const blob = new Blob([resultsCsv(shown, i18n.language || 'en')], { type: 'text/csv' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = `clean-ips${isp ? '-' + isp : ''}.csv`;
    a.click();
    URL.revokeObjectURL(a.href);
  }, [shown, isp, i18n.language]);
  const copyResults = useCallback(async () => {
    const text = rows.map((r) => (isp ? `${r.ip} ${isp}` : r.ip)).join('\n');
    try {
      await navigator.clipboard.writeText(text);
      messageApi.success(t('pages.cleanIp.copied'));
    } catch {
      messageApi.error(t('somethingWentWrong'));
    }
  }, [rows, isp, messageApi, t]);

  const pageClass = ['clean-ip-page', isDark ? 'is-dark' : '', isUltra ? 'is-ultra' : '']
    .filter(Boolean)
    .join(' ');
  const percent = total ? Math.round((done / total) * 100) : 0;
  const gutter: [number, number] = [isMobile ? 8 : 16, isMobile ? 8 : 16];

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
                    {t('pages.cleanIp.intro', { sep: ISP_SEP.trim() })}
                  </Typography.Paragraph>
                  <Alert type="info" showIcon title={t('pages.cleanIp.vpnHint')} />
                </Card>
              </Col>

              <Col xs={24} lg={12}>
                <Card
                  size="small"
                  className="clean-ip-card"
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
                      {viaCloudflare ? (
                        <Alert type="error" showIcon title={t('pages.cleanIp.vpnWarning')} />
                      ) : !net.isp ? (
                        <Alert type="warning" showIcon title={t('pages.cleanIp.noIsp')} />
                      ) : null}
                    </div>
                  ) : null}
                  <label className="clean-ip-field">
                    <span>{t('pages.cleanIp.ispName')}</span>
                    <Input
                      value={ispTyped ?? net?.isp ?? ''}
                      maxLength={24}
                      placeholder="MCI"
                      onChange={(e) => setIsp(e.target.value.replace(/\s+/g, ''))}
                    />
                  </label>
                </Card>
              </Col>

              <Col xs={24} lg={12}>
                <Card
                  size="small"
                  className="clean-ip-card"
                  title={<StepTitle n={2}>{t('pages.cleanIp.domainStep')}</StepTitle>}
                >
                  {!groups.length ? (
                    <Alert type="info" showIcon title={t('pages.cleanIp.noGroups')} />
                  ) : (
                    <>
                      <Typography.Paragraph type="secondary">
                        {t('pages.cleanIp.domainHint')}
                      </Typography.Paragraph>
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
                          <span>{t('pages.cleanIp.cdn')}</span>
                          <Select
                            value={cdnId}
                            onChange={setCdnId}
                            loading={cdns.isLoading}
                            options={(cdns.data ?? []).map((c) => ({ value: c.id, label: c.name }))}
                            popupMatchSelectWidth={false}
                          />
                        </label>
                        <label className="clean-ip-field">
                          <span>{t('pages.cleanIp.port')}</span>
                          <InputNumber
                            min={1}
                            max={65535}
                            value={port}
                            onChange={(v) => setPortTyped({ groupId, port: Number(v) || 443 })}
                          />
                        </label>
                        <label className="clean-ip-field clean-ip-grow">
                          <span>{t('pages.cleanIp.domain')}</span>
                          <Input
                            className="clean-ip-mono"
                            value={domain}
                            placeholder="cdn.example.com"
                            onChange={(e) =>
                              setDomainTyped({ groupId, domain: e.target.value.trim() })
                            }
                          />
                        </label>
                      </div>
                      {cdn && (
                        <Collapse
                          className="clean-ip-note"
                          size="small"
                          ghost
                          items={[
                            {
                              key: 'ranges',
                              label: t('pages.cleanIp.rangesTitle', { count: cdnRanges.length }),
                              children: (
                                <RangePicker
                                  cdn={cdn}
                                  ticked={ticked}
                                  onTicked={setTicked}
                                  extra={extraRanges}
                                  onExtra={setExtraRanges}
                                  disabled={running}
                                />
                              ),
                            },
                          ]}
                        />
                      )}
                      {!domain && (
                        <Alert
                          className="clean-ip-note"
                          type="warning"
                          showIcon
                          title={t('pages.cleanIp.noDomain')}
                        />
                      )}
                    </>
                  )}
                </Card>
              </Col>

              <Col span={24}>
                <Card
                  size="small"
                  title={<StepTitle n={3}>{t('pages.cleanIp.fullTest')}</StepTitle>}
                >
                  <FragmentStep
                    domain={domain}
                    size={size}
                    perRange={cmdPerRange}
                    onSize={setSize}
                    onPerRange={setCmdPerRange}
                    command={command}
                    making={making}
                    onMakeCommand={makeCommand}
                    expired={reports.isError}
                    report={report}
                    fragKey={fragKey}
                    onFragKey={(key) => report && setFragPick({ at: report.at, key })}
                  />
                </Card>
              </Col>

              <Col span={24}>
                <Card
                  size="small"
                  title={
                    <StepTitle
                      n={4}
                      extra={
                        <Space size={6}>
                          <Button
                            size="small"
                            icon={<CopyOutlined aria-hidden />}
                            disabled={!rows.length}
                            aria-label={t('pages.cleanIp.copy')}
                            onClick={copyResults}
                          >
                            {isMobile ? null : t('pages.cleanIp.copy')}
                          </Button>
                          <Button
                            size="small"
                            icon={<DownloadOutlined aria-hidden />}
                            disabled={!shown.length}
                            aria-label="CSV"
                            onClick={downloadCsv}
                          >
                            {isMobile ? null : 'CSV'}
                          </Button>
                        </Space>
                      }
                    >
                      {t('pages.cleanIp.scan')}
                    </StepTitle>
                  }
                >
                  <div className="clean-ip-settings">
                    <Segmented
                      value={source}
                      disabled={running}
                      onChange={(v) => setSource(v as 'ranges' | 'list')}
                      options={[
                        { value: 'ranges', label: t('pages.cleanIp.sourceRanges') },
                        { value: 'list', label: t('pages.cleanIp.sourceList') },
                      ]}
                    />
                  </div>
                  {source === 'list' && (
                    <Input.TextArea
                      aria-label={t('pages.cleanIp.sourceList')}
                      className="clean-ip-mono clean-ip-note"
                      rows={4}
                      value={listText}
                      disabled={running}
                      placeholder={'104.16.1.1\n172.64.0.0/24'}
                      onChange={(e) => setListText(e.target.value)}
                    />
                  )}
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
                        <>
                          <Button type="primary" onClick={startScan}>
                            {t('pages.cleanIp.start')}
                          </Button>
                          <Button disabled={!hits.length} onClick={findMore}>
                            {t('pages.cleanIp.findMore')}
                          </Button>
                        </>
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
                  {countries.length > 0 && (
                    <label className="clean-ip-field clean-ip-note clean-ip-country">
                      <span>{t('pages.cleanIp.countryFilter')}</span>
                      <Select
                        allowClear
                        value={country}
                        placeholder={t('pages.cleanIp.allCountries')}
                        options={countries}
                        onChange={(v) => {
                          setCountry(v);
                          setPicked(null);
                        }}
                      />
                    </label>
                  )}
                  <Table
                    size="small"
                    rowKey="ip"
                    dataSource={shown}
                    pagination={shown.length > 50 ? { pageSize: 50 } : false}
                    locale={{ emptyText: <Empty description={t('pages.cleanIp.empty')} /> }}
                    rowSelection={{
                      selectedRowKeys: selected,
                      onChange: (keys) => setPicked(keys as string[]),
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
                        width: 110,
                        render: (ms: number) => `${ms} ms`,
                      },
                      {
                        title: t('pages.cleanIp.datacenter'),
                        dataIndex: 'colo',
                        responsive: ['sm'],
                        render: (colo: string) => {
                          const place = placeOf(colo);
                          return place
                            ? `${flagOf(place.country)} ${place.city}, ${place.countryName}`
                            : colo || '—';
                        },
                      },
                      {
                        title: t('pages.cleanIp.source'),
                        dataIndex: 'test',
                        width: 120,
                        render: (test: IpRow['test']) =>
                          test === 'upload' ? (
                            <Tag color="success">{t('pages.cleanIp.sourceUpload')}</Tag>
                          ) : test === 'failed' ? (
                            <Tag color="error">{t('pages.cleanIp.sourceFailed')}</Tag>
                          ) : (
                            <Tag>{t('pages.cleanIp.sourceQuick')}</Tag>
                          ),
                      },
                      {
                        title: t('pages.cleanIp.serverCheck'),
                        dataIndex: 'ip',
                        key: 'verdict',
                        width: 150,
                        render: (ip: string) => <VerdictTag status={verdicts[ip]} />,
                      },
                    ]}
                  />
                </Card>
              </Col>

              <Col span={24}>
                <Card
                  size="small"
                  title={<StepTitle n={5}>{t('pages.cleanIp.saveStep')}</StepTitle>}
                >
                  {!group ? (
                    <Alert type="info" showIcon title={t('pages.cleanIp.noGroups')} />
                  ) : (
                    <>
                      <div className="clean-ip-summary">
                        <div>
                          <span>{t('pages.cleanIp.summaryGroup')}</span>
                          <b>{targetName}</b>
                        </div>
                        <div>
                          <span>{t('pages.cleanIp.summaryIps')}</span>
                          <b className="clean-ip-mono">
                            {selected.length ? selected.join('  ') : '—'}
                          </b>
                        </div>
                        <div>
                          <span>{t('pages.cleanIp.fragment')}</span>
                          <b className="clean-ip-mono">{fragmentLabel(fragment)}</b>
                        </div>
                      </div>
                      {otherIspReport && report && (
                        <Alert
                          className="clean-ip-note"
                          type="info"
                          showIcon
                          title={t('pages.cleanIp.reportOtherIsp', {
                            isp: report.network.isp || report.network.org || '?',
                          })}
                        />
                      )}
                      <div className="clean-ip-actions clean-ip-save">
                        <Button
                          type="primary"
                          icon={<SaveOutlined aria-hidden />}
                          loading={saving}
                          disabled={!selected.length || !isp}
                          onClick={save}
                        >
                          {t('pages.cleanIp.saveFor', { count: selected.length, isp: isp || '?' })}
                        </Button>
                        <Popconfirm
                          title={t('pages.cleanIp.serverCheckConfirm', {
                            count: Math.min(selected.length, VERIFY_MAX),
                          })}
                          okText={t('pages.cleanIp.serverCheck')}
                          cancelText={t('cancel')}
                          disabled={!selected.length || verifying}
                          onConfirm={() => void verifyIps(selected)}
                        >
                          <Button disabled={!selected.length} loading={verifying}>
                            {t('pages.cleanIp.serverCheckSelected', { count: selected.length })}
                          </Button>
                        </Popconfirm>
                        {verifying && (
                          <Button onClick={() => verifyAbort.current?.abort()}>
                            {t('cancel')}
                          </Button>
                        )}
                      </div>
                      {!namesShowHost && (
                        <Alert
                          className="clean-ip-note"
                          type="warning"
                          showIcon
                          title={t('pages.cleanIp.templateHint', { tag: '{{HOST}}' })}
                        />
                      )}
                      <div className="clean-ip-step-title clean-ip-subtitle">
                        <Typography.Title level={5} className="clean-ip-step-text">
                          {t('pages.cleanIp.ispGroups')}
                        </Typography.Title>
                        <Popconfirm
                          title={t('pages.cleanIp.serverCheckConfirm', { count: savedIps.length })}
                          okText={t('pages.cleanIp.serverCheck')}
                          cancelText={t('cancel')}
                          disabled={!savedIps.length || verifying}
                          onConfirm={() => void checkSaved()}
                        >
                          <Button size="small" loading={verifying} disabled={!savedIps.length}>
                            {t('pages.cleanIp.checkSaved')}
                          </Button>
                        </Popconfirm>
                        {restrictedSaved.length > 0 && (
                          <Popconfirm
                            title={t('pages.cleanIp.removeRestrictedConfirm', {
                              count: restrictedSaved.length,
                            })}
                            okText={t('delete')}
                            cancelText={t('cancel')}
                            onConfirm={removeRestricted}
                          >
                            <Button size="small" danger loading={saving}>
                              {t('pages.cleanIp.removeRestricted', {
                                count: restrictedSaved.length,
                              })}
                            </Button>
                          </Popconfirm>
                        )}
                      </div>
                      {mainIps.length > 0 && (
                        <div className="clean-ip-isp-row">
                          <Tag>{group.remark}</Tag>
                          <span className="clean-ip-mono clean-ip-isp-ips">
                            <SavedIps ips={mainIps} verdicts={verdicts} />
                          </span>
                        </div>
                      )}
                      {ispGroups.size === 0 ? (
                        <Typography.Text type="secondary">
                          {t('pages.cleanIp.noIspGroups')}
                        </Typography.Text>
                      ) : (
                        <div className="clean-ip-isp-list">
                          {[...ispGroups.entries()].map(([name, g]) => (
                            <div key={g.groupId} className="clean-ip-isp-row">
                              <Tag color="processing">{name}</Tag>
                              {g.finalMask?.includes('"fragment"') && (
                                <Tag>{t('pages.cleanIp.fragment')}</Tag>
                              )}
                              <span className="clean-ip-mono clean-ip-isp-ips">
                                <SavedIps ips={g.hosts.map(hostAddress)} verdicts={verdicts} />
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
