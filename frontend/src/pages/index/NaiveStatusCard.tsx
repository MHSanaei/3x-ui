import { useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Button, Card, Modal, Skeleton, Space, Tag, Tooltip } from 'antd';
import {
  CloudServerOutlined,
  DeleteOutlined,
  DownloadOutlined,
  FileTextOutlined,
  PoweroffOutlined,
  ReloadOutlined,
  ToolOutlined,
} from '@ant-design/icons';

import { keys } from '@/api/queryKeys';
import { HttpUtil } from '@/utils';
import './NaiveStatusCard.css';
import NaiveLogModal from './NaiveLogModal';

interface BusyEvent {
  busy: boolean;
  tip?: string;
}

interface NaiveInstance {
  tag: string;
  running: boolean;
  uptimeSeconds: number;
  error?: string;
}

interface NaiveStatusResponse {
  installed: boolean;
  version?: string;
  instances: NaiveInstance[];
}

interface NaiveStatusCardProps {
  isMobile: boolean;
  onOpenVersionModal: () => void;
  onBusy: (event: BusyEvent) => void;
}

async function fetchNaiveStatus(): Promise<NaiveStatusResponse> {
  const msg = await HttpUtil.get<NaiveStatusResponse>(
    '/panel/api/naive/status',
    undefined,
    { silent: true },
  );
  if (!msg.success || !msg.obj) {
    throw new Error(msg.msg || 'Naive status unavailable');
  }
  return msg.obj;
}

function formatVersion(raw: string): string {
  const value = raw.trim().replace(/^naive\s+/i, '');
  if (!value) return '';
  return value.startsWith('v') ? value : `v${value}`;
}

function formatUptime(seconds: number): string {
  const safeSeconds = Number.isFinite(seconds) ? Math.max(0, Math.floor(seconds)) : 0;
  if (safeSeconds < 60) return `${safeSeconds}s`;
  const minutes = Math.floor(safeSeconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  const remainder = minutes % 60;
  return remainder > 0 ? `${hours}h ${remainder}m` : `${hours}h`;
}

export default function NaiveStatusCard({
  isMobile,
  onOpenVersionModal,
  onBusy,
}: NaiveStatusCardProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [modal, modalContextHolder] = Modal.useModal();
  const [logTag, setLogTag] = useState('');
  const statusQuery = useQuery({
    queryKey: keys.naive.status(),
    queryFn: fetchNaiveStatus,
    refetchInterval: 5000,
  });

  const data = statusQuery.data ?? { installed: false, version: '', instances: [] };
  const runningCount = data.instances.filter((instance) => instance.running).length;
  const longestUptime = data.instances.reduce((longest, instance) => {
    if (!instance.running || !Number.isFinite(instance.uptimeSeconds)) return longest;
    return Math.max(longest, instance.uptimeSeconds);
  }, 0);
  const version = formatVersion(data.version ?? '');
  const versionFamily = version.match(/^v\d+/i)?.[0] ?? version;
  const status = useMemo(() => {
    if (statusQuery.isLoading) {
      return { color: 'processing', text: t('loading') };
    }
    if (statusQuery.isError) {
      return { color: 'error', text: t('somethingWentWrong') };
    }
    if (!data.installed) {
      return { color: 'default', text: t('pages.xray.naive.notInstalled') };
    }
    if (runningCount === 0) {
      return { color: 'warning', text: t('pages.index.xrayStatusStop') };
    }
    return {
      color: runningCount === data.instances.length ? 'success' : 'processing',
      text: `${t('pages.index.xrayStatusRunning')} ${runningCount}/${data.instances.length}`,
    };
  }, [
    data.installed,
    data.instances.length,
    runningCount,
    statusQuery.isError,
    statusQuery.isLoading,
    t,
  ]);

  async function mutate(path: string) {
    onBusy({ busy: true, tip: t('pages.index.dontRefresh') });
    try {
      const msg = await HttpUtil.post(path, {}, {
        headers: { 'Content-Type': 'application/json' },
      });
      if (msg.success) {
        await queryClient.invalidateQueries({ queryKey: keys.naive.status() });
      }
    } finally {
      onBusy({ busy: false });
    }
  }

  function uninstall() {
    modal.confirm({
      title: t('pages.xray.naive.uninstall'),
      content: t('pages.xray.naive.uninstallConfirm'),
      okText: t('confirm'),
      cancelText: t('cancel'),
      okButtonProps: { danger: true },
      onOk: async () => {
        await mutate('/panel/api/naive/binary/delete');
      },
    });
  }

  const compactAction = (label: string, icon: ReactNode, onClick: () => void, danger = false) => (
    <Tooltip title={isMobile ? label : undefined}>
      <Button
        type="text"
        danger={danger}
        size={isMobile ? 'small' : 'middle'}
        icon={icon}
        aria-label={label}
        onClick={onClick}
      >
        {isMobile ? undefined : label}
      </Button>
    </Tooltip>
  );

  return (
    <>
      {modalContextHolder}
      <Card hoverable className="ov-naive-card" styles={{ body: { padding: 0 } }}>
        <div className="ov-naive-head">
          <div className="ov-naive-title">
            <span className="ov-naive-icon"><CloudServerOutlined /></span>
            <div>
              <div className="ov-kicker">NaïveProxy Client</div>
              <div className="ov-sub">
                {statusQuery.isLoading
                  ? t('loading')
                  : statusQuery.isError
                    ? t('somethingWentWrong')
                    : data.installed
                      ? version || 'NaïveProxy Client'
                      : t('pages.xray.naive.notInstalled')}
              </div>
            </div>
          </div>

          <Tag color={status.color} className="ov-naive-status">{status.text}</Tag>

          <Space size={2} wrap className="ov-naive-actions">
            {!statusQuery.isLoading && !statusQuery.isError && (
              <>
                {compactAction(
                  data.installed ? t('update') : t('pages.xray.naive.install'),
                  data.installed ? <ToolOutlined /> : <DownloadOutlined />,
                  onOpenVersionModal,
                )}
                {data.installed && compactAction(
                  t('pages.xray.naive.restartAll'),
                  <ReloadOutlined />,
                  () => { void mutate('/panel/api/naive/restart-all'); },
                )}
                {data.installed && compactAction(
                  t('pages.xray.naive.stop'),
                  <PoweroffOutlined />,
                  () => { void mutate('/panel/api/naive/stop-all'); },
                )}
                {data.installed && compactAction(
                  t('pages.xray.naive.uninstall'),
                  <DeleteOutlined />,
                  uninstall,
                  true,
                )}
              </>
            )}
          </Space>
        </div>

        {statusQuery.isLoading ? (
          <div className="ov-naive-message">
            <Skeleton active paragraph={{ rows: 2 }} />
          </div>
        ) : statusQuery.isError ? (
          <div className="ov-naive-message">
            <Alert
              type="error"
              showIcon
              title={
                statusQuery.error instanceof Error
                  ? statusQuery.error.message
                  : t('somethingWentWrong')
              }
              action={(
                <Button size="small" onClick={() => { void statusQuery.refetch(); }}>
                  {t('refresh')}
                </Button>
              )}
            />
          </div>
        ) : data.installed ? (
          <div className="ov-naive-body">
            <div className="ov-naive-summary">
              <div>
                <span className="ov-kicker">{t('pages.index.xraySwitch')}</span>
                <strong className="ov-mono">{versionFamily || '—'}</strong>
              </div>
              <div>
                <span className="ov-kicker">{t('pages.index.uptime')}</span>
                <strong>{runningCount > 0 ? formatUptime(longestUptime) : '—'}</strong>
              </div>
              <div>
                <span className="ov-kicker">{t('pages.index.xrayStatusRunning')}</span>
                <strong>{runningCount}</strong>
              </div>
            </div>

            <div className="ov-naive-instances">
              {data.instances.map((instance) => (
                <div className="ov-naive-instance" key={instance.tag}>
                  <span className="ov-naive-instance-state" data-running={instance.running} />
                  <span className="ov-naive-instance-tag ov-mono">{instance.tag}</span>
                  <span className="ov-naive-instance-uptime">
                    {instance.running ? formatUptime(instance.uptimeSeconds) : '—'}
                  </span>
                  <Button
                    type="text"
                    size="small"
                    icon={<FileTextOutlined />}
                    aria-label={t('pages.index.logs')}
                    onClick={() => setLogTag(instance.tag)}
                  />
                  {instance.error && <Tooltip title={instance.error}><span className="ov-naive-error">!</span></Tooltip>}
                </div>
              ))}
              {data.instances.length === 0 && <span className="ov-sub">{t('pages.xray.naive.instances', { count: 0 })}</span>}
            </div>
          </div>
        ) : (
          <div className="ov-naive-empty">
            <CloudServerOutlined />
            <div>
              <strong>{t('pages.xray.naive.notInstalled')}</strong>
              <span>{t('pages.xray.naive.versionWarning')}</span>
            </div>
          </div>
        )}
      </Card>
      <NaiveLogModal open={!!logTag} tag={logTag} onClose={() => setLogTag('')} />
    </>
  );
}
