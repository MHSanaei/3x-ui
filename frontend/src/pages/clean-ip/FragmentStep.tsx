import { useTranslation } from 'react-i18next';
import { Alert, Button, InputNumber, Radio, Table, Tag, Typography } from 'antd';
import { CodeOutlined } from '@ant-design/icons';

import type { FragmentSetting } from './scan';

export interface CleanIpCommand {
  token: string;
  expiresAt: number;
  unix: string;
  windows: string;
}

export interface CleanIpFragment extends FragmentSetting {
  ok: number;
  n: number;
  ms: number;
}

export interface CleanIpReport {
  at: number;
  network: { ip: string; asn: number; org: string; isp: string };
  tested: number;
  size: number;
  ips: { ip: string; ms: number; colo: string }[];
  fragments: CleanIpFragment[];
}

export function fragmentKey(f: FragmentSetting): string {
  return f.packets ? `${f.packets} ${f.length} ${f.delay}` : '';
}

export default function FragmentStep({
  domain,
  size,
  perRange,
  onSize,
  onPerRange,
  command,
  making,
  onMakeCommand,
  expired,
  report,
  fragKey,
  onFragKey,
}: {
  domain: string;
  size: number;
  perRange: number;
  onSize: (v: number) => void;
  onPerRange: (v: number) => void;
  command: CleanIpCommand | null;
  making: boolean;
  onMakeCommand: () => void;
  expired: boolean;
  report: CleanIpReport | undefined;
  fragKey: string;
  onFragKey: (key: string) => void;
}) {
  const { t } = useTranslation();
  const label = (f: CleanIpFragment) =>
    f.packets
      ? `${f.packets === 'tlshello' ? 'tlshello' : 'TCP'} · ${f.length} · ${f.delay} ms`
      : t('pages.cleanIp.noFragment');

  return (
    <>
      <Typography.Paragraph type="secondary">{t('pages.cleanIp.fullIntro')}</Typography.Paragraph>
      <div className="clean-ip-settings">
        <label className="clean-ip-field">
          <span>{t('pages.cleanIp.size')}</span>
          <InputNumber
            min={1000}
            max={64000}
            step={1000}
            value={size}
            onChange={(v) => onSize(Number(v) || 5000)}
          />
        </label>
        <label className="clean-ip-field">
          <span>{t('pages.cleanIp.perRange')}</span>
          <InputNumber
            min={1}
            max={64}
            value={perRange}
            onChange={(v) => onPerRange(Number(v) || 8)}
          />
        </label>
        <div className="clean-ip-actions">
          <Button
            icon={<CodeOutlined aria-hidden />}
            loading={making}
            disabled={!domain}
            onClick={onMakeCommand}
          >
            {t('pages.cleanIp.makeCommand')}
          </Button>
        </div>
      </div>
      {!domain && (
        <Alert
          className="clean-ip-note"
          type="warning"
          showIcon
          title={t('pages.cleanIp.noDomain')}
        />
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
          {expired ? (
            <Alert
              className="clean-ip-note"
              type="warning"
              showIcon
              title={t('pages.cleanIp.toasts.expired')}
            />
          ) : !report ? (
            <Typography.Paragraph className="clean-ip-note" type="secondary">
              {t('pages.cleanIp.waiting')}
            </Typography.Paragraph>
          ) : (
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
              <Radio.Group
                className="clean-ip-frag-group"
                value={fragKey}
                onChange={(e) => onFragKey(e.target.value)}
              >
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
              <Alert
                className="clean-ip-note"
                type="info"
                showIcon
                title={t('pages.cleanIp.fragHint')}
              />
            </div>
          )}
        </div>
      )}
    </>
  );
}
