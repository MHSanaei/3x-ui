/* oxlint-disable react/set-state-in-effect -- same fetch-on-open/sync-from-response
   shape as TorModal.tsx: both setState calls react to a real external input (the
   open prop flipping, a status response landing), at modal-open frequency, not a hot path. */
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Divider, Modal, Popconfirm, Select, Tag, Typography, message } from 'antd';
import {
  PlayCircleOutlined,
  PauseCircleOutlined,
  ReloadOutlined,
  DownloadOutlined,
  DeleteOutlined,
  UploadOutlined,
  GlobalOutlined,
  CheckCircleOutlined,
} from '@ant-design/icons';

import { HttpUtil } from '@/utils';

interface PsiphonModalProps {
  open: boolean;
  templateSettings: { outbounds?: { tag?: string }[] } | null;
  onClose: () => void;
  onAddOutbound: (outbound: Record<string, unknown>) => void;
  onRemoveOutbound: (tag: string) => void;
}

interface TunnelStatus {
  connected: boolean;
  tunnelCount: number;
  serverRegion?: string;
  clientRegion?: string;
}

interface PsiphonStatus {
  installed: boolean;
  configured: boolean;
  running: boolean;
  port: number;
  tunnel: TunnelStatus;
  lastLog?: string;
}

interface ExitInfo {
  ip: string;
  country: string;
}

interface Region {
  code: string;
  name: string;
}

const PSIPHON_TAG = 'psiphon';

export default function PsiphonModal({
  open,
  templateSettings,
  onClose,
  onAddOutbound,
  onRemoveOutbound,
}: PsiphonModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState<PsiphonStatus | null>(null);
  const [regions, setRegions] = useState<Region[]>([]);
  const [selectedRegion, setSelectedRegion] = useState<string>('');
  const [verifyLoading, setVerifyLoading] = useState(false);
  const [exitInfo, setExitInfo] = useState<ExitInfo | null>(null);

  const psiphonOutboundIndex = useMemo(() => {
    const list = templateSettings?.outbounds;
    if (!list) return -1;
    return list.findIndex((o) => o?.tag === PSIPHON_TAG);
  }, [templateSettings?.outbounds]);

  const fetchStatus = useCallback(async () => {
    const msg = await HttpUtil.post<PsiphonStatus>('/panel/api/xray/psiphon/status');
    if (msg?.success && msg.obj) setStatus(msg.obj);
  }, []);

  useEffect(() => {
    if (!open) return;
    fetchStatus();
    setExitInfo(null);
  }, [open, fetchStatus]);

  const regionsFetched = useRef(false);
  useEffect(() => {
    if (!open || regionsFetched.current) return;
    regionsFetched.current = true;
    HttpUtil.get<Region[]>('/panel/api/xray/psiphon/regions').then((msg) => {
      if (msg?.success && msg.obj) setRegions(msg.obj);
    });
  }, [open]);

  useEffect(() => {
    if (status?.tunnel.serverRegion) setSelectedRegion(status.tunnel.serverRegion);
  }, [status?.tunnel.serverRegion]);

  async function install() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/xray/psiphon/install');
      if (msg?.success) {
        messageApi.success(t('pages.xray.psiphon.installed'));
      } else {
        messageApi.error(msg?.msg || t('pages.xray.psiphon.installFailed'));
      }
      await fetchStatus();
    } finally {
      setLoading(false);
    }
  }

  async function uninstall() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/xray/psiphon/uninstall');
      if (msg?.success) {
        messageApi.success(t('pages.xray.psiphon.uninstalled'));
        if (psiphonOutboundIndex >= 0) onRemoveOutbound(PSIPHON_TAG);
      } else {
        messageApi.error(msg?.msg || t('pages.xray.psiphon.uninstallFailed'));
      }
      await fetchStatus();
    } finally {
      setLoading(false);
    }
  }

  async function start() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/xray/psiphon/start');
      if (msg?.success) {
        messageApi.success(t('pages.xray.psiphon.started'));
      } else {
        messageApi.error(msg?.msg || t('pages.xray.psiphon.startFailed'));
      }
      await fetchStatus();
    } finally {
      setLoading(false);
    }
  }

  async function stop() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/xray/psiphon/stop');
      if (msg?.success) messageApi.success(t('pages.xray.psiphon.stopped'));
      setExitInfo(null);
      await fetchStatus();
    } finally {
      setLoading(false);
    }
  }

  function uploadConfig() {
    const fileInput = document.createElement('input');
    fileInput.type = 'file';
    fileInput.addEventListener('change', async (e) => {
      const file = (e.target as HTMLInputElement).files?.[0];
      if (!file) return;

      const formData = new FormData();
      formData.append('config', file);

      setLoading(true);
      try {
        const msg = await HttpUtil.post<PsiphonStatus>(
          '/panel/api/xray/psiphon/config/upload',
          formData,
          { headers: { 'Content-Type': 'multipart/form-data' } },
        );
        if (msg?.success) {
          messageApi.success(t('pages.xray.psiphon.configUploaded'));
          if (msg.obj) setStatus(msg.obj);
        } else {
          messageApi.error(msg?.msg || t('pages.xray.psiphon.configUploadFailed'));
        }
      } finally {
        setLoading(false);
      }
    });
    fileInput.click();
  }

  async function applyRegion() {
    setVerifyLoading(true);
    setExitInfo(null);
    try {
      const msg = await HttpUtil.post<ExitInfo>('/panel/api/xray/psiphon/region', {
        region: selectedRegion,
      });
      if (msg?.success) {
        messageApi.success(t('pages.xray.psiphon.regionApplied'));
        if (msg.obj) setExitInfo(msg.obj);
      } else {
        messageApi.error(msg?.msg || t('pages.xray.psiphon.regionFailed'));
      }
      await fetchStatus();
    } finally {
      setVerifyLoading(false);
    }
  }

  async function verify() {
    setVerifyLoading(true);
    setExitInfo(null);
    try {
      const msg = await HttpUtil.post<ExitInfo>('/panel/api/xray/psiphon/verify');
      if (msg?.success && msg.obj) {
        setExitInfo(msg.obj);
      } else {
        messageApi.error(msg?.msg || t('pages.xray.psiphon.verifyFailed'));
      }
    } finally {
      setVerifyLoading(false);
    }
  }

  function addOutbound() {
    if (!status) return;
    onAddOutbound({
      tag: PSIPHON_TAG,
      protocol: 'socks',
      settings: { servers: [{ address: '127.0.0.1', port: status.port }] },
    });
    messageApi.success(t('pages.xray.psiphon.outboundAdded'));
    onClose();
  }

  function removeOutbound() {
    onRemoveOutbound(PSIPHON_TAG);
    messageApi.success(t('pages.xray.psiphon.outboundRemoved'));
  }

  const regionOptions = useMemo(
    () => [
      { value: '', label: t('pages.xray.psiphon.egressRegionAuto') },
      ...regions.map((r) => ({ value: r.code, label: `${r.name} (${r.code})` })),
    ],
    [regions, t],
  );

  return (
    <>
      {messageContextHolder}
      <Modal open={open} title={t('pages.xray.psiphon.title')} footer={null} onCancel={onClose}>
        {status && !status.installed ? (
          <>
            <Tag color="red">{t('pages.xray.psiphon.notInstalled')}</Tag>
            <p style={{ marginTop: 12 }}>{t('pages.xray.psiphon.installHint')}</p>
            <div style={{ display: 'flex', gap: 8 }}>
              <Button
                type="primary"
                icon={<DownloadOutlined />}
                loading={loading}
                onClick={install}
              >
                {t('pages.xray.psiphon.installButton')}
              </Button>
              <Button icon={<ReloadOutlined />} loading={loading} onClick={fetchStatus}>
                {t('refresh')}
              </Button>
            </div>
          </>
        ) : status && !status.configured ? (
          <>
            <Tag color="orange">{t('pages.xray.psiphon.notConfigured')}</Tag>
            <p style={{ marginTop: 12 }}>{t('pages.xray.psiphon.configHint')}</p>
            <div style={{ display: 'flex', gap: 8 }}>
              <Button
                type="primary"
                icon={<UploadOutlined />}
                loading={loading}
                onClick={uploadConfig}
              >
                {t('pages.xray.psiphon.uploadConfigButton')}
              </Button>
              <Button icon={<ReloadOutlined />} loading={loading} onClick={fetchStatus}>
                {t('refresh')}
              </Button>
            </div>
          </>
        ) : (
          <>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
              {status?.running ? (
                <Tag color="green">{t('pages.xray.psiphon.running')}</Tag>
              ) : (
                <Tag color="orange">{t('pages.xray.psiphon.stoppedState')}</Tag>
              )}
              {status?.running ? (
                <Button danger icon={<PauseCircleOutlined />} loading={loading} onClick={stop}>
                  {t('pages.xray.psiphon.stopButton')}
                </Button>
              ) : (
                <Button
                  type="primary"
                  icon={<PlayCircleOutlined />}
                  loading={loading}
                  onClick={start}
                >
                  {t('pages.xray.psiphon.startButton')}
                </Button>
              )}
              <Button
                icon={<UploadOutlined />}
                loading={loading}
                onClick={uploadConfig}
                title={t('pages.xray.psiphon.uploadConfigButton')}
              />
              <Button aria-label={t('refresh')} icon={<ReloadOutlined />} onClick={fetchStatus} />
              <Popconfirm
                title={t('pages.xray.psiphon.uninstallConfirm')}
                okText={t('delete')}
                okType="danger"
                cancelText={t('cancel')}
                onConfirm={uninstall}
              >
                <Button danger icon={<DeleteOutlined />} loading={loading}>
                  {t('pages.xray.psiphon.uninstallButton')}
                </Button>
              </Popconfirm>
            </div>
            {status?.lastLog && (
              <div style={{ fontSize: 12, color: '#888', marginTop: 8 }}>{status.lastLog}</div>
            )}

            {status?.running && (
              <>
                <Divider className="my-10">{t('pages.xray.psiphon.egressRegionTitle')}</Divider>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                  <Select
                    showSearch
                    style={{ minWidth: 220 }}
                    value={selectedRegion}
                    options={regionOptions}
                    optionFilterProp="label"
                    onChange={(v) => setSelectedRegion(v)}
                  />
                  <Button type="primary" loading={verifyLoading} onClick={applyRegion}>
                    {t('pages.xray.psiphon.applyRegionButton')}
                  </Button>
                  <Button icon={<GlobalOutlined />} loading={verifyLoading} onClick={verify}>
                    {t('pages.xray.psiphon.verifyButton')}
                  </Button>
                </div>
                <div style={{ marginTop: 8 }}>
                  {status.tunnel.connected ? (
                    <Tag icon={<CheckCircleOutlined />} color="green">
                      {t('pages.xray.psiphon.tunnelConnected')}
                      {status.tunnel.serverRegion ? ` (${status.tunnel.serverRegion})` : ''}
                    </Tag>
                  ) : (
                    <Tag color="orange">{t('pages.xray.psiphon.tunnelNotConnected')}</Tag>
                  )}
                  {exitInfo && (
                    <Tag color="blue" style={{ marginLeft: 8 }}>
                      {t('pages.xray.psiphon.verifiedExit', {
                        ip: exitInfo.ip,
                        country: exitInfo.country,
                      })}
                    </Tag>
                  )}
                </div>
              </>
            )}

            <Divider className="my-10">{t('pages.xray.outbound.outboundStatus')}</Divider>
            {psiphonOutboundIndex >= 0 ? (
              <>
                <Tag color="green">{t('enabled')}</Tag>
                <Button type="primary" danger className="ml-8" onClick={removeOutbound}>
                  {t('delete')}
                </Button>
              </>
            ) : (
              <>
                <Tag color="orange">{t('disabled')}</Tag>
                <Button type="primary" className="ml-8" onClick={addOutbound}>
                  {t('pages.xray.warp.addOutbound')}
                </Button>
              </>
            )}
            <Typography.Paragraph type="secondary" style={{ marginTop: 12, fontSize: 12 }}>
              {t('pages.xray.psiphon.outboundHint', { port: status?.port })}
            </Typography.Paragraph>
          </>
        )}
      </Modal>
    </>
  );
}
