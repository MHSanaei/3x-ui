import { useCallback, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Card,
  Empty,
  Input,
  InputNumber,
  Radio,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { CodeOutlined, SaveOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import type { HostRecord } from '@/schemas/api/host';
import { hostAddress, type FragmentSetting } from './scan';

interface CleanIpCommand {
  token: string;
  expiresAt: number;
  unix: string;
  windows: string;
}

interface CleanIpFragment extends FragmentSetting {
  ok: number;
  n: number;
  ms: number;
}

interface CleanIpReport {
  at: number;
  network: { ip: string; asn: number; org: string; isp: string };
  tested: number;
  size: number;
  ips: { ip: string; ms: number; colo: string }[];
  fragments: CleanIpFragment[];
}

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };

// The CDN domain the test uses as SNI: what the group's configs present to Cloudflare.
function groupDomain(group: HostRecord): string {
  const named = group.hosts.map(hostAddress).find((h) => !/^[\d.]+$/.test(h) && !h.includes(':'));
  return group.sni || group.hostHeader || named || '';
}

function fragmentKey(f: FragmentSetting): string {
  return f.packets ? `${f.packets} ${f.length} ${f.delay}` : '';
}

function ReportView({
  report,
  defaultIsp,
  saving,
  onSave,
}: {
  report: CleanIpReport;
  defaultIsp: string;
  saving: boolean;
  onSave: (isp: string, ips: string[], frag: FragmentSetting | null) => void;
}) {
  const { t } = useTranslation();
  const [isp, setIsp] = useState(report.network.isp || defaultIsp);
  const [ips, setIps] = useState(() => report.ips.slice(0, 5).map((h) => h.ip));
  const none = report.fragments.find((f) => !f.packets);
  const best = report.fragments[0];
  // A fragment is only worth sending when it passes more often than no fragment at all.
  const [fragKey, setFragKey] = useState(() =>
    best && best.packets && best.ok > (none?.ok ?? 0) ? fragmentKey(best) : '',
  );
  const frag = report.fragments.find((f) => fragmentKey(f) === fragKey && f.packets) ?? null;
  const label = (f: CleanIpFragment) =>
    f.packets
      ? `${f.packets === 'tlshello' ? 'tlshello' : 'TCP'} · ${f.length} · ${f.delay} ms`
      : t('pages.cleanIp.noFragment');

  return (
    <div className="clean-ip-report">
      <div className="clean-ip-report-head">
        {report.network.isp ? <Tag color="processing">{report.network.isp}</Tag> : null}
        <span>
          {t('pages.cleanIp.reportLine', {
            org: report.network.org || report.network.ip,
            ok: report.ips.length,
            tested: report.tested,
            size: report.size,
          })}
        </span>
        <Typography.Text type="secondary">
          {new Date(report.at * 1000).toLocaleTimeString()}
        </Typography.Text>
      </div>
      <div className="clean-ip-report-body">
        <Table
          size="small"
          rowKey="ip"
          dataSource={report.ips}
          pagination={report.ips.length > 20 ? { pageSize: 20 } : false}
          locale={{ emptyText: <Empty description={t('pages.cleanIp.empty')} /> }}
          rowSelection={{ selectedRowKeys: ips, onChange: (keys) => setIps(keys as string[]) }}
          columns={[
            {
              title: t('pages.cleanIp.ip'),
              dataIndex: 'ip',
              render: (ip: string) => <span className="clean-ip-mono">{ip}</span>,
            },
            {
              title: t('pages.cleanIp.latency'),
              dataIndex: 'ms',
              width: 100,
              render: (ms: number) => `${ms} ms`,
            },
            { title: 'Colo', dataIndex: 'colo', width: 70 },
          ]}
        />
        <Radio.Group value={fragKey} onChange={(e) => setFragKey(e.target.value)}>
          <Table
            size="small"
            rowKey={(f) => fragmentKey(f) || 'none'}
            dataSource={report.fragments}
            pagination={{ pageSize: 8, size: 'small' }}
            columns={[
              {
                title: t('pages.cleanIp.fragment'),
                render: (_: unknown, f: CleanIpFragment) => (
                  <Radio value={fragmentKey(f)}>
                    <span className="clean-ip-mono">{label(f)}</span>
                  </Radio>
                ),
              },
              {
                title: t('pages.cleanIp.passed'),
                width: 90,
                render: (_: unknown, f: CleanIpFragment) => `${f.ok}/${f.n}`,
              },
              {
                title: t('pages.cleanIp.latency'),
                width: 100,
                render: (_: unknown, f: CleanIpFragment) => (f.ok ? `${f.ms} ms` : '—'),
              },
            ]}
          />
        </Radio.Group>
      </div>
      <div className="clean-ip-settings">
        <label className="clean-ip-field">
          <span>{t('pages.cleanIp.ispName')}</span>
          <Input
            value={isp}
            maxLength={24}
            placeholder="MCI"
            onChange={(e) => setIsp(e.target.value.replace(/\s+/g, ''))}
          />
        </label>
        <div className="clean-ip-actions">
          <Button
            type="primary"
            icon={<SaveOutlined aria-hidden />}
            loading={saving}
            disabled={!ips.length || !isp.trim()}
            onClick={() => onSave(isp, ips, frag)}
          >
            {t(frag ? 'pages.cleanIp.saveWithFragment' : 'pages.cleanIp.saveFor', {
              count: ips.length,
              isp: isp.trim() || '?',
            })}
          </Button>
        </div>
      </div>
    </div>
  );
}

export default function FullTestCard({
  group,
  seedIps,
  defaultIsp,
  saving,
  onSave,
}: {
  group: HostRecord;
  seedIps: string[];
  defaultIsp: string;
  saving: boolean;
  onSave: (isp: string, ips: string[], frag: FragmentSetting | null) => void;
}) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [size, setSize] = useState(5000);
  const [perRange, setPerRange] = useState(8);
  const [command, setCommand] = useState<CleanIpCommand | null>(null);
  const [making, setMaking] = useState(false);
  const domain = groupDomain(group);

  const makeCommand = useCallback(async () => {
    setMaking(true);
    try {
      const msg = await HttpUtil.post(
        '/panel/api/cleanip/command',
        { host: domain, path: group.path || '/', size, perRange, ips: seedIps },
        JSON_HEADERS,
      );
      if (msg?.success) setCommand(msg.obj as CleanIpCommand);
      else messageApi.error(msg?.msg || t('pages.cleanIp.toasts.command'));
    } finally {
      setMaking(false);
    }
  }, [domain, group.path, size, perRange, seedIps, messageApi, t]);

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
  const newestFirst = useMemo(() => [...(reports.data ?? [])].reverse(), [reports.data]);

  return (
    <Card
      size="small"
      title={
        <Space>
          <CodeOutlined aria-hidden />
          {t('pages.cleanIp.fullTest')}
        </Space>
      }
    >
      {messageContextHolder}
      <Typography.Paragraph type="secondary">{t('pages.cleanIp.fullIntro')}</Typography.Paragraph>
      {!domain ? (
        <Alert type="warning" showIcon title={t('pages.cleanIp.noDomain')} />
      ) : (
        <div className="clean-ip-settings">
          <label className="clean-ip-field">
            <span>{t('pages.cleanIp.domain')}</span>
            <Input value={domain} readOnly className="clean-ip-mono" />
          </label>
          <label className="clean-ip-field">
            <span>{t('pages.cleanIp.size')}</span>
            <InputNumber
              min={1000}
              max={64000}
              step={1000}
              value={size}
              onChange={(v) => setSize(Number(v) || 5000)}
            />
          </label>
          <label className="clean-ip-field">
            <span>{t('pages.cleanIp.perRange')}</span>
            <InputNumber
              min={1}
              max={64}
              value={perRange}
              onChange={(v) => setPerRange(Number(v) || 8)}
            />
          </label>
          <div className="clean-ip-actions">
            <Button icon={<CodeOutlined aria-hidden />} loading={making} onClick={makeCommand}>
              {t('pages.cleanIp.makeCommand')}
            </Button>
          </div>
        </div>
      )}

      {command && (
        <div className="clean-ip-command">
          <Typography.Text type="secondary">{t('pages.cleanIp.unixCmd')}</Typography.Text>
          <Typography.Paragraph copyable className="clean-ip-mono clean-ip-code">
            {command.unix}
          </Typography.Paragraph>
          <Typography.Text type="secondary">{t('pages.cleanIp.windowsCmd')}</Typography.Text>
          <Typography.Paragraph copyable className="clean-ip-mono clean-ip-code">
            {command.windows}
          </Typography.Paragraph>
          <Typography.Text type="secondary">
            {t('pages.cleanIp.expires', {
              time: new Date(command.expiresAt * 1000).toLocaleString(),
            })}
          </Typography.Text>
          <Alert
            className="clean-ip-note"
            type="info"
            showIcon
            title={t('pages.cleanIp.fragHint')}
          />
          {reports.isError ? (
            <Alert
              className="clean-ip-note"
              type="warning"
              showIcon
              title={t('pages.cleanIp.toasts.expired')}
            />
          ) : newestFirst.length === 0 ? (
            <Typography.Paragraph className="clean-ip-note" type="secondary">
              {t('pages.cleanIp.waiting')}
            </Typography.Paragraph>
          ) : (
            newestFirst.map((r) => (
              <ReportView
                key={r.at}
                report={r}
                defaultIsp={defaultIsp}
                saving={saving}
                onSave={onSave}
              />
            ))
          )}
        </div>
      )}
    </Card>
  );
}
