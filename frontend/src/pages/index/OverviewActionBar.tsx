import { Fragment } from 'react';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Tag, Tooltip } from 'antd';
import {
  ApiOutlined,
  ArrowUpOutlined,
  AreaChartOutlined,
  BarsOutlined,
  CloudDownloadOutlined,
  CloudServerOutlined,
  ControlOutlined,
  FileTextOutlined,
  PoweroffOutlined,
  ReloadOutlined,
} from '@ant-design/icons';

import { formatPanelVersion } from '@/lib/panel-version';
import { activateOnKey } from '@/utils/a11y';
import type { Status } from '@/models/status';

interface OverviewActionBarProps {
  status: Status;
  isMobile: boolean;
  accessLogEnable: boolean;
  panelVersion: string;
  latestVersion: string;
  updateAvailable: boolean;
  onStopXray: () => void;
  onRestartXray: () => void;
  onOpenLogs: () => void;
  onOpenXrayLogs: () => void;
  onOpenAmneziaWGLogs: () => void;
  onOpenConfig: () => void;
  onOpenBackup: () => void;
  onOpenSystemHistory: () => void;
  onOpenXrayMetrics: () => void;
  onOpenPanelUpdate: () => void;
  onOpenVersionSwitch: () => void;
}

interface BarAction {
  key: string;
  icon: ReactNode;
  text: string;
  onClick: () => void;
  primary?: boolean;
}

const XRAY_STATE_KEYS: Record<string, string> = {
  running: 'pages.index.xrayStatusRunning',
  stop: 'pages.index.xrayStatusStop',
  error: 'pages.index.xrayStatusError',
};

export default function OverviewActionBar({
  status,
  isMobile,
  accessLogEnable,
  panelVersion,
  latestVersion,
  updateAvailable,
  onStopXray,
  onRestartXray,
  onOpenLogs,
  onOpenXrayLogs,
  onOpenAmneziaWGLogs,
  onOpenConfig,
  onOpenBackup,
  onOpenSystemHistory,
  onOpenXrayMetrics,
  onOpenPanelUpdate,
  onOpenVersionSwitch,
}: OverviewActionBarProps) {
  const { t } = useTranslation();
  const stateText = t(XRAY_STATE_KEYS[status.xray.state] ?? 'pages.index.xrayStatusUnknown');
  const hasVersion = !!status.xray.version && status.xray.version !== 'Unknown';
  const size = isMobile ? ('small' as const) : ('middle' as const);

  const actionGroups: BarAction[][] = [
    [
      {
        key: 'restart',
        icon: <ReloadOutlined aria-hidden />,
        text: t('pages.index.restartXray'),
        onClick: onRestartXray,
        primary: true,
      },
      {
        key: 'stop',
        icon: <PoweroffOutlined aria-hidden />,
        text: t('pages.index.stopXray'),
        onClick: onStopXray,
      },
    ],
    [
      {
        key: 'logs',
        icon: <BarsOutlined aria-hidden />,
        text: t('pages.index.logs'),
        onClick: onOpenLogs,
      },
      ...(accessLogEnable
        ? [
            {
              key: 'accessLogs',
              icon: <FileTextOutlined aria-hidden />,
              text: t('pages.index.accessLogs'),
              onClick: onOpenXrayLogs,
            },
          ]
        : []),
      ...(status.amneziawg.configured
        ? [
            {
              key: 'amneziawgLogs',
              icon: <ApiOutlined aria-hidden />,
              text: t('pages.index.amneziawgLogs'),
              onClick: onOpenAmneziaWGLogs,
            },
          ]
        : []),
      {
        key: 'config',
        icon: <ControlOutlined aria-hidden />,
        text: t('pages.index.config'),
        onClick: onOpenConfig,
      },
      {
        key: 'backup',
        icon: <CloudServerOutlined aria-hidden />,
        text: t('pages.index.backupTitle'),
        onClick: onOpenBackup,
      },
    ],
    [
      {
        key: 'history',
        icon: <AreaChartOutlined aria-hidden />,
        text: t('pages.index.systemHistoryTitle'),
        onClick: onOpenSystemHistory,
      },
      {
        key: 'metrics',
        icon: <ArrowUpOutlined aria-hidden />,
        text: t('pages.index.xrayMetricsTitle'),
        onClick: onOpenXrayMetrics,
      },
    ],
  ];

  const statePill = (
    <span className="ov-state" data-state={status.xray.state}>
      <span className="ov-state-dot" style={{ color: status.xray.color }} />
      <span>{`${t('pages.index.xrayStatus')} · ${stateText}`}</span>
      {hasVersion && (
        <Tooltip title={t('pages.index.xraySwitch')}>
          <button type="button" className="ov-state-version" onClick={onOpenVersionSwitch}>
            {`v${status.xray.version}`}
          </button>
        </Tooltip>
      )}
    </span>
  );

  return (
    <div className="ov-bar">
      {status.xray.errorMsg ? (
        <Tooltip title={<span className="ov-error-detail">{status.xray.errorMsg}</span>}>
          {statePill}
        </Tooltip>
      ) : (
        statePill
      )}

      {status.xray.state === 'running' && status.xray.errorMsg ? (
        <Tooltip title={<span className="ov-error-detail">{status.xray.errorMsg}</span>}>
          <Tag color="error">{t('pages.index.xrayStatusError')}</Tag>
        </Tooltip>
      ) : null}

      {updateAvailable ? (
        <Tag
          className="ov-update-tag"
          color="warning"
          icon={<CloudDownloadOutlined aria-hidden />}
          role="button"
          tabIndex={0}
          onClick={onOpenPanelUpdate}
          onKeyDown={activateOnKey(onOpenPanelUpdate)}
        >
          {`${t('update')} ${formatPanelVersion(latestVersion)}`}
        </Tag>
      ) : (
        <Tooltip title={t('pages.index.updatePanel')}>
          <button type="button" className="ov-panel-version ov-mono" onClick={onOpenPanelUpdate}>
            {formatPanelVersion(panelVersion)}
          </button>
        </Tooltip>
      )}

      <div className="ov-bar-actions">
        {actionGroups.map((group, groupIndex) => (
          <Fragment key={group[0].key}>
            {groupIndex > 0 && <span className="ov-bar-sep" />}
            {group.map((action) => (
              <Button
                key={action.key}
                type={action.primary ? undefined : 'text'}
                color={action.primary ? 'primary' : undefined}
                variant={action.primary ? 'outlined' : undefined}
                size={size}
                icon={action.icon}
                aria-label={action.text}
                onClick={action.onClick}
              >
                {isMobile ? undefined : action.text}
              </Button>
            ))}
          </Fragment>
        ))}
      </div>
    </div>
  );
}
