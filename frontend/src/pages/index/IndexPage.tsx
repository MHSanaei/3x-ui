import { lazy, useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Col,
  ConfigProvider,
  Layout,
  message,
  Modal,
  Result,
  Row,
  Space,
  Spin,
  Statistic,
  Tag,
  Tooltip,
} from 'antd';
import {
  BarsOutlined,
  ControlOutlined,
  CloudServerOutlined,
  CloudDownloadOutlined,
  CloudUploadOutlined,
  ArrowUpOutlined,
  ArrowDownOutlined,
  AreaChartOutlined,
  GlobalOutlined,
  SwapOutlined,
  EyeOutlined,
  EyeInvisibleOutlined,
  ThunderboltOutlined,
  DesktopOutlined,
  DatabaseOutlined,
  ForkOutlined,
  CopyOutlined,
} from '@ant-design/icons';

import { HttpUtil, SizeFormatter, TimeFormatter, ClipboardManager, FileManager } from '@/utils';
import { formatPanelVersion } from '@/lib/panel-version';
import { useTheme } from '@/hooks/useTheme';
import { useStatusQuery } from '@/api/queries/useStatusQuery';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import AppSidebar from '@/layouts/AppSidebar';
import { LazyMount } from '@/components/utility';
import { setMessageInstance } from '@/utils/messageBus';
import StatusCard from './StatusCard';
import XrayStatusCard from './XrayStatusCard';
import type { PanelUpdateInfo } from './PanelUpdateModal';
const JsonEditor = lazy(() => import('@/components/form/JsonEditor'));
const PanelUpdateModal = lazy(() => import('./PanelUpdateModal'));
const LogModal = lazy(() => import('./LogModal'));
const BackupModal = lazy(() => import('./BackupModal'));
const SystemHistoryModal = lazy(() => import('./SystemHistoryModal'));
const XrayMetricsModal = lazy(() => import('./XrayMetricsModal'));
const XrayLogModal = lazy(() => import('./XrayLogModal'));
const VersionModal = lazy(() => import('./VersionModal'));
import './IndexPage.css';

export default function IndexPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { status, fetched, fetchError, refresh } = useStatusQuery();
  const { isMobile } = useMediaQuery();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => { setMessageInstance(messageApi); }, [messageApi]);

  const [accessLogEnable, setAccessLogEnable] = useState(false);
  const [isDevBuild, setIsDevBuild] = useState(false);
  const [devChannelEnable, setDevChannelEnable] = useState(false);
  const [panelUpdateInfo, setPanelUpdateInfo] = useState<PanelUpdateInfo>({
    currentVersion: '',
    latestVersion: '',
    updateAvailable: false,
  });

  const basePath = window.X_UI_BASE_PATH || '';

  const [showIp, setShowIp] = useState(false);
  const [logsOpen, setLogsOpen] = useState(false);
  const [backupOpen, setBackupOpen] = useState(false);
  const [panelUpdateOpen, setPanelUpdateOpen] = useState(false);
  const [sysHistoryOpen, setSysHistoryOpen] = useState(false);
  const [xrayMetricsOpen, setXrayMetricsOpen] = useState(false);
  const [xrayLogsOpen, setXrayLogsOpen] = useState(false);
  const [versionOpen, setVersionOpen] = useState(false);
  const [configTextOpen, setConfigTextOpen] = useState(false);
  const [configText, setConfigText] = useState('');
  const [loading, setLoading] = useState(false);
  const [loadingTip, setLoadingTip] = useState(t('loading'));

  useEffect(() => {
    HttpUtil.post<{ accessLogEnable?: boolean; isDevBuild?: boolean; devChannelEnable?: boolean }>(
      '/panel/api/setting/defaultSettings',
    ).then((msg) => {
      if (msg?.success && msg.obj) {
        setAccessLogEnable(!!msg.obj.accessLogEnable);
        setIsDevBuild(!!msg.obj.isDevBuild);
        setDevChannelEnable(!!msg.obj.devChannelEnable);
      }
    });
    HttpUtil.get<PanelUpdateInfo>('/panel/api/server/getPanelUpdateInfo').then((msg) => {
      if (msg?.success && msg.obj) setPanelUpdateInfo(msg.obj);
    });
  }, []);

  const displayVersion = useMemo(
    () => window.X_UI_CUR_VER || panelUpdateInfo.currentVersion || '?',
    [panelUpdateInfo.currentVersion],
  );

  const setBusy = useCallback(
    ({ busy, tip }: { busy: boolean; tip?: string }) => {
      setLoading(busy);
      if (tip) setLoadingTip(tip);
    },
    [],
  );

  const stopXray = useCallback(async () => {
    await HttpUtil.post('/panel/api/server/stopXrayService');
    await refresh();
  }, [refresh]);

  const restartXray = useCallback(async () => {
    await HttpUtil.post('/panel/api/server/restartXrayService');
    await refresh();
  }, [refresh]);

  function openPanelVersion() {
    if (panelUpdateInfo.updateAvailable || isDevBuild) {
      setPanelUpdateOpen(true);
    } else {
      window.open('https://github.com/MHSanaei/3x-ui/releases', '_blank', 'noopener,noreferrer');
    }
  }

  async function handleChannelChange(dev: boolean) {
    const res = await HttpUtil.post('/panel/api/server/setUpdateChannel', { dev });
    if (!res?.success) return;
    setDevChannelEnable(dev);
    const msg = await HttpUtil.get<PanelUpdateInfo>('/panel/api/server/getPanelUpdateInfo');
    if (msg?.success && msg.obj) setPanelUpdateInfo(msg.obj);
  }

  function openTelegram() {
    window.open('https://t.me/XrayUI', '_blank', 'noopener,noreferrer');
  }

  async function openConfig() {
    setLoading(true);
    try {
      const msg = await HttpUtil.get('/panel/api/server/getConfigJson');
      if (!msg?.success) return;
      setConfigText(JSON.stringify(msg.obj, null, 2));
      setConfigTextOpen(true);
    } finally {
      setLoading(false);
    }
  }

  async function copyConfig() {
    const ok = await ClipboardManager.copyText(configText || '');
    if (ok) messageApi.success('Copied');
  }

  function downloadConfig() {
    FileManager.downloadTextFile(configText, 'config.json');
  }

  const pageClass = `index-page ${isDark ? 'is-dark' : ''} ${isUltra ? 'is-ultra' : ''}`.trim();

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />

        <Layout className="content-shell">
          <Layout.Content className="content-area">
            <Spin
              spinning={loading || !fetched}
              delay={200}
              description={loading ? loadingTip : t('loading')}
              size="large"
            >
              {!fetched ? (
                <div className="loading-spacer" />
              ) : fetchError ? (
                <Result
                  status="error"
                  title={t('somethingWentWrong')}
                  subTitle={fetchError}
                  extra={<Button type="primary" onClick={refresh}>{t('refresh')}</Button>}
                />
              ) : (
                <Row gutter={[isMobile ? 8 : 16, 12]}>
                  <Col span={24}>
                    <StatusCard status={status} isMobile={isMobile} />
                  </Col>

                  <Col xs={24} lg={12}>
                    <XrayStatusCard
                      status={status}
                      isMobile={isMobile}
                      accessLogEnable={accessLogEnable}
                      onStopXray={stopXray}
                      onRestartXray={restartXray}
                      onOpenXrayLogs={() => setXrayLogsOpen(true)}
                      onOpenLogs={() => setLogsOpen(true)}
                      onOpenVersionSwitch={() => setVersionOpen(true)}
                    />
                  </Col>

                  <Col xs={24} lg={12}>
                    <Card
                      title={t('menu.link')}
                      hoverable
                      actions={[
                        <Space className="action" key="logs" onClick={() => setLogsOpen(true)}>
                          <BarsOutlined />
                          {!isMobile && <span>{t('pages.index.logs')}</span>}
                        </Space>,
                        <Space className="action" key="config" onClick={openConfig}>
                          <ControlOutlined />
                          {!isMobile && <span>{t('pages.index.config')}</span>}
                        </Space>,
                        <Space className="action" key="backup" onClick={() => setBackupOpen(true)}>
                          <CloudServerOutlined />
                          {!isMobile && <span>{t('pages.index.backupTitle')}</span>}
                        </Space>,
                      ]}
                    />
                  </Col>

                  <Col xs={24} lg={12}>
                    <Card
                      title={
                        <Space>
                          <span>3X-UI</span>
                          {isMobile && displayVersion && (
                            <Tag color={panelUpdateInfo.updateAvailable ? 'orange' : 'green'}>
                              {panelUpdateInfo.updateAvailable
                                ? formatPanelVersion(panelUpdateInfo.latestVersion)
                                : formatPanelVersion(displayVersion)}
                            </Tag>
                          )}
                        </Space>
                      }
                      hoverable
                      actions={[
                        <Space className="action" key="tg" onClick={openTelegram}>
                          <svg
                            viewBox="0 0 24 24"
                            width="14"
                            height="14"
                            fill="currentColor"
                            className="tg-icon"
                            aria-hidden="true"
                          >
                            <path d="M21.93 4.34a1.5 1.5 0 0 0-2.05-1.6L2.97 9.6c-.92.36-.91 1.66.02 1.99l4.32 1.53 1.7 5.23a1 1 0 0 0 1.68.36l2.43-2.43 4.36 3.21a1.5 1.5 0 0 0 2.36-.91l3.09-13.86a1.5 1.5 0 0 0 0-.38ZM9.97 14.66l-.55 3.36-1.36-4.2 9.8-7.05-7.89 7.89Z" />
                          </svg>
                          {!isMobile && <span>@XrayUI</span>}
                        </Space>,
                        <Space
                          key="panel-version"
                          className={`action ${panelUpdateInfo.updateAvailable ? 'action-update' : ''}`}
                          onClick={openPanelVersion}
                        >
                          <CloudDownloadOutlined />
                          {!isMobile && (
                            <span>
                              {panelUpdateInfo.updateAvailable
                                ? `${t('update')} ${formatPanelVersion(panelUpdateInfo.latestVersion)}`
                                : formatPanelVersion(displayVersion)}
                            </span>
                          )}
                        </Space>,
                      ]}
                    />
                  </Col>

                  <Col xs={24} lg={12}>
                    <Card
                      title={t('pages.index.charts')}
                      hoverable
                      actions={[
                        <Space
                          className="action"
                          key="sys-history"
                          onClick={() => setSysHistoryOpen(true)}
                        >
                          <AreaChartOutlined />
                          {!isMobile && <span>{t('pages.index.systemHistoryTitle')}</span>}
                        </Space>,
                        <Space
                          className="action"
                          key="xray-metrics"
                          onClick={() => setXrayMetricsOpen(true)}
                        >
                          <AreaChartOutlined />
                          {!isMobile && <span>{t('pages.index.xrayMetricsTitle')}</span>}
                        </Space>,
                      ]}
                    />
                  </Col>

                  <Col xs={24} lg={12}>
                    <Card title={t('pages.index.operationHours')} hoverable>
                      <Row gutter={isMobile ? [8, 8] : 0}>
                        <Col span={12}>
                          <Statistic
                            title="Xray"
                            value={TimeFormatter.formatSecond(status.appStats.uptime)}
                            prefix={<ThunderboltOutlined />}
                          />
                        </Col>
                        <Col span={12}>
                          <Statistic
                            title="OS"
                            value={TimeFormatter.formatSecond(status.uptime)}
                            prefix={<DesktopOutlined />}
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col xs={24} lg={12}>
                    <Card title={t('usage')} hoverable>
                      <Row gutter={isMobile ? [8, 8] : 0}>
                        <Col span={12}>
                          <Statistic
                            title={t('pages.index.memory')}
                            value={SizeFormatter.sizeFormat(status.appStats.mem)}
                            prefix={<DatabaseOutlined />}
                          />
                        </Col>
                        <Col span={12}>
                          <Statistic
                            title={t('pages.index.threads')}
                            value={status.appStats.threads}
                            prefix={<ForkOutlined />}
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col xs={24} lg={12}>
                    <Card title={t('pages.index.overallSpeed')} hoverable>
                      <Row gutter={isMobile ? [8, 8] : 0}>
                        <Col span={12}>
                          <Statistic
                            title={t('pages.index.upload')}
                            value={SizeFormatter.sizeFormat(status.netIO.up)}
                            prefix={<ArrowUpOutlined />}
                            suffix="/s"
                          />
                        </Col>
                        <Col span={12}>
                          <Statistic
                            title={t('pages.index.download')}
                            value={SizeFormatter.sizeFormat(status.netIO.down)}
                            prefix={<ArrowDownOutlined />}
                            suffix="/s"
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col xs={24} lg={12}>
                    <Card title={t('pages.index.totalData')} hoverable>
                      <Row gutter={isMobile ? [8, 8] : 0}>
                        <Col span={12}>
                          <Statistic
                            title={t('pages.index.sent')}
                            value={SizeFormatter.sizeFormat(status.netTraffic.sent)}
                            prefix={<CloudUploadOutlined />}
                          />
                        </Col>
                        <Col span={12}>
                          <Statistic
                            title={t('pages.index.received')}
                            value={SizeFormatter.sizeFormat(status.netTraffic.recv)}
                            prefix={<CloudDownloadOutlined />}
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col xs={24} lg={12}>
                    <Card
                      title={t('pages.index.ipAddresses')}
                      hoverable
                      extra={
                        <Tooltip
                          title={t('pages.index.toggleIpVisibility')}
                          placement={isMobile ? 'topRight' : 'top'}
                        >
                          {showIp ? (
                            <EyeOutlined
                              className="ip-toggle-icon"
                              onClick={() => setShowIp(false)}
                            />
                          ) : (
                            <EyeInvisibleOutlined
                              className="ip-toggle-icon"
                              onClick={() => setShowIp(true)}
                            />
                          )}
                        </Tooltip>
                      }
                    >
                      <Row className={showIp ? 'ip-visible' : 'ip-hidden'} gutter={isMobile ? [8, 8] : 0}>
                        <Col span={isMobile ? 24 : 12}>
                          <Statistic
                            title="IPv4"
                            value={status.publicIP.ipv4}
                            prefix={<GlobalOutlined />}
                          />
                        </Col>
                        <Col span={isMobile ? 24 : 12}>
                          <Statistic
                            title="IPv6"
                            value={status.publicIP.ipv6}
                            prefix={<GlobalOutlined />}
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col xs={24} lg={12}>
                    <Card title={t('pages.index.connectionCount')} hoverable>
                      <Row gutter={isMobile ? [8, 8] : 0}>
                        <Col span={12}>
                          <Statistic
                            title="TCP"
                            value={status.tcpCount}
                            prefix={<SwapOutlined />}
                          />
                        </Col>
                        <Col span={12}>
                          <Statistic
                            title="UDP"
                            value={status.udpCount}
                            prefix={<SwapOutlined />}
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>
                </Row>
              )}
            </Spin>
          </Layout.Content>
        </Layout>

        <LazyMount when={panelUpdateOpen}>
          <PanelUpdateModal
            open={panelUpdateOpen}
            info={panelUpdateInfo}
            isDevBuild={isDevBuild}
            devChannelEnable={devChannelEnable}
            onChannelChange={handleChannelChange}
            onClose={() => setPanelUpdateOpen(false)}
            onBusy={setBusy}
          />
        </LazyMount>
        <LazyMount when={logsOpen}>
          <LogModal open={logsOpen} onClose={() => setLogsOpen(false)} />
        </LazyMount>
        <LazyMount when={backupOpen}>
          <BackupModal
            open={backupOpen}
            basePath={basePath}
            onClose={() => setBackupOpen(false)}
            onBusy={setBusy}
          />
        </LazyMount>
        <LazyMount when={sysHistoryOpen}>
          <SystemHistoryModal
            open={sysHistoryOpen}
            status={status}
            onClose={() => setSysHistoryOpen(false)}
          />
        </LazyMount>
        <LazyMount when={xrayMetricsOpen}>
          <XrayMetricsModal open={xrayMetricsOpen} onClose={() => setXrayMetricsOpen(false)} />
        </LazyMount>
        <LazyMount when={xrayLogsOpen}>
          <XrayLogModal open={xrayLogsOpen} onClose={() => setXrayLogsOpen(false)} />
        </LazyMount>
        <LazyMount when={versionOpen}>
          <VersionModal
            open={versionOpen}
            status={status}
            onClose={() => setVersionOpen(false)}
            onBusy={setBusy}
          />
        </LazyMount>

        <LazyMount when={configTextOpen}>
          <Modal
            open={configTextOpen}
            title={t('pages.index.config')}
            width={isMobile ? '100%' : 900}
            style={isMobile
              ? { top: 20, maxWidth: 'calc(100vw - 16px)' }
              : { top: 20 }}
            onCancel={() => setConfigTextOpen(false)}
            footer={[
              <Button
                key="download"
                onClick={downloadConfig}
                size={isMobile ? 'small' : 'middle'}
                icon={<CloudDownloadOutlined />}
              >
                {isMobile ? 'Download' : 'config.json'}
              </Button>,
              <Button
                key="copy"
                type="primary"
                onClick={copyConfig}
                size={isMobile ? 'small' : 'middle'}
                icon={<CopyOutlined />}
              >
                Copy
              </Button>,
            ]}
          >
            <JsonEditor
              value={configText}
              onChange={setConfigText}
              minHeight={isMobile ? '300px' : 'calc(100vh - 220px)'}
              maxHeight={isMobile ? '70vh' : 'calc(100vh - 220px)'}
              readOnly
            />
          </Modal>
        </LazyMount>
      </Layout>
    </ConfigProvider>
  );
}
