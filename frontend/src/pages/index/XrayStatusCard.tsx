import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Badge, Card, Col, Popover, Row, Space, Tag } from 'antd';
import {
  BarsOutlined,
  PoweroffOutlined,
  ReloadOutlined,
  ToolOutlined,
} from '@ant-design/icons';

import type { Status } from '@/models/status';
import './XrayStatusCard.css';

interface XrayStatusCardProps {
  status: Status;
  isMobile: boolean;
  ipLimitEnable: boolean;
  onStopXray: () => void;
  onRestartXray: () => void;
  onOpenLogs: () => void;
  onOpenXrayLogs: () => void;
  onOpenVersionSwitch: () => void;
}

const XRAY_STATE_KEYS: Record<string, string> = {
  running: 'pages.index.xrayStatusRunning',
  stop: 'pages.index.xrayStatusStop',
  error: 'pages.index.xrayStatusError',
};

export default function XrayStatusCard({
  status,
  isMobile,
  ipLimitEnable,
  onStopXray,
  onRestartXray,
  onOpenLogs,
  onOpenXrayLogs,
  onOpenVersionSwitch,
}: XrayStatusCardProps) {
  const { t } = useTranslation();

  const stateText = t(XRAY_STATE_KEYS[status.xray.state] ?? 'pages.index.xrayStatusUnknown');

  const title = (
    <Space>
      <span>{t('pages.index.xrayStatus')}</span>
      {isMobile && status.xray.version && status.xray.version !== 'Unknown' && (
        <Tag color="green">v{status.xray.version}</Tag>
      )}
    </Space>
  );

  const errorLines = useMemo(
    () => (status.xray.errorMsg || '').split('\n'),
    [status.xray.errorMsg],
  );

  const extra =
    status.xray.state !== 'error' ? (
      <Badge status="processing" text={stateText} color={status.xray.color} />
    ) : (
      <Popover
        title={
          <Row align="middle" justify="space-between">
            <Col>
              <span>{t('pages.index.xrayStatusError')}</span>
            </Col>
            <Col>
              <BarsOutlined className="cursor-pointer" onClick={onOpenLogs} />
            </Col>
          </Row>
        }
        content={
          <>
            {errorLines.map((line, i) => (
              <span key={i} className="error-line">
                {line}
              </span>
            ))}
          </>
        }
      >
        <Badge status="processing" text={stateText} color={status.xray.color} />
      </Popover>
    );

  const actions = [
    ...(ipLimitEnable
      ? [
          <Space className="action" key="xraylogs" onClick={onOpenXrayLogs}>
            <BarsOutlined />
            {!isMobile && <span>{t('pages.index.logs')}</span>}
          </Space>,
        ]
      : []),
    <Space className="action" key="stop" onClick={onStopXray}>
      <PoweroffOutlined />
      {!isMobile && <span>{t('pages.index.stopXray')}</span>}
    </Space>,
    <Space className="action" key="restart" onClick={onRestartXray}>
      <ReloadOutlined />
      {!isMobile && <span>{t('pages.index.restartXray')}</span>}
    </Space>,
    <Space className="action" key="switch" onClick={onOpenVersionSwitch}>
      <ToolOutlined />
      {!isMobile && (
        <span>
          {status.xray.version && status.xray.version !== 'Unknown'
            ? `v${status.xray.version}`
            : t('pages.index.xraySwitch')}
        </span>
      )}
    </Space>,
  ];

  return (
    <Card hoverable title={title} extra={extra} actions={actions} className="xray-status-card" />
  );
}
