/* oxlint-disable react/set-state-in-effect -- same fetch-on-open shape as
   PsiphonModal.tsx/TorModal.tsx: reacts to the open prop flipping, at
   modal-open frequency, not a hot path. */
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Divider, Modal, Popconfirm, Tag, Typography, message } from 'antd';
import {
  PlayCircleOutlined,
  PauseCircleOutlined,
  ReloadOutlined,
  DownloadOutlined,
  DeleteOutlined,
  SyncOutlined,
  WarningOutlined,
} from '@ant-design/icons';

import { HttpUtil } from '@/utils';

interface WireproxyModalProps {
  open: boolean;
  templateSettings: { outbounds?: { tag?: string }[] } | null;
  onClose: () => void;
  onAddOutbound: (outbound: Record<string, unknown>) => void;
  onRemoveOutbound: (tag: string) => void;
}

interface WireproxyStatus {
  installed: boolean;
  healthy: boolean;
  scheduler: string;
  service: { active: boolean };
  socks5: { port: number; listening: boolean };
  warp: { on: boolean; ip: string; colo: string; loc: string };
  routingGuard: { danger: boolean };
}

const WIREPROXY_TAG = 'wireproxy';

export default function WireproxyModal({
  open,
  templateSettings,
  onClose,
  onAddOutbound,
  onRemoveOutbound,
}: WireproxyModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);
  const [repairLoading, setRepairLoading] = useState(false);
  const [status, setStatus] = useState<WireproxyStatus | null>(null);

  const wireproxyOutboundIndex = useMemo(() => {
    const list = templateSettings?.outbounds;
    if (!list) return -1;
    return list.findIndex((o) => o?.tag === WIREPROXY_TAG);
  }, [templateSettings?.outbounds]);

  const fetchStatus = useCallback(async () => {
    const msg = await HttpUtil.post<WireproxyStatus>('/panel/api/xray/wireproxy/status');
    if (msg?.success && msg.obj) setStatus(msg.obj);
  }, []);

  useEffect(() => {
    if (!open) return;
    fetchStatus();
  }, [open, fetchStatus]);

  async function install() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/xray/wireproxy/install');
      if (msg?.success) {
        messageApi.success(t('pages.xray.wireproxy.installed'));
      } else {
        messageApi.error(msg?.msg || t('pages.xray.wireproxy.installFailed'));
      }
      await fetchStatus();
    } finally {
      setLoading(false);
    }
  }

  async function uninstall() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/xray/wireproxy/uninstall');
      if (msg?.success) {
        messageApi.success(t('pages.xray.wireproxy.uninstalled'));
        if (wireproxyOutboundIndex >= 0) onRemoveOutbound(WIREPROXY_TAG);
      } else {
        messageApi.error(msg?.msg || t('pages.xray.wireproxy.uninstallFailed'));
      }
      await fetchStatus();
    } finally {
      setLoading(false);
    }
  }

  async function start() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/xray/wireproxy/start');
      if (msg?.success) {
        messageApi.success(t('pages.xray.wireproxy.started'));
      } else {
        messageApi.error(msg?.msg || t('pages.xray.wireproxy.startFailed'));
      }
      await fetchStatus();
    } finally {
      setLoading(false);
    }
  }

  async function stop() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/xray/wireproxy/stop');
      if (msg?.success) messageApi.success(t('pages.xray.wireproxy.stopped'));
      await fetchStatus();
    } finally {
      setLoading(false);
    }
  }

  async function repair() {
    setRepairLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/xray/wireproxy/repair', { mode: 'check' });
      if (msg?.success) {
        messageApi.success(t('pages.xray.wireproxy.repaired'));
      } else {
        messageApi.error(msg?.msg || t('pages.xray.wireproxy.repairFailed'));
      }
      await fetchStatus();
    } finally {
      setRepairLoading(false);
    }
  }

  async function fixRouting() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/xray/wireproxy/fixRouting');
      if (msg?.success) {
        messageApi.success(t('pages.xray.wireproxy.routingFixed'));
      } else {
        messageApi.error(msg?.msg || t('pages.xray.wireproxy.routingFixFailed'));
      }
      await fetchStatus();
    } finally {
      setLoading(false);
    }
  }

  function addOutbound() {
    if (!status) return;
    onAddOutbound({
      tag: WIREPROXY_TAG,
      protocol: 'socks',
      settings: { servers: [{ address: '127.0.0.1', port: status.socks5.port }] },
    });
    messageApi.success(t('pages.xray.wireproxy.outboundAdded'));
    onClose();
  }

  function removeOutbound() {
    onRemoveOutbound(WIREPROXY_TAG);
    messageApi.success(t('pages.xray.wireproxy.outboundRemoved'));
  }

  return (
    <>
      {messageContextHolder}
      <Modal open={open} title={t('pages.xray.wireproxy.title')} footer={null} onCancel={onClose}>
        {status && !status.installed ? (
          <>
            <Tag color="red">{t('pages.xray.wireproxy.notInstalled')}</Tag>
            <p style={{ marginTop: 12 }}>{t('pages.xray.wireproxy.installHint')}</p>
            <div style={{ display: 'flex', gap: 8 }}>
              <Popconfirm
                title={t('pages.xray.wireproxy.installConfirm')}
                okText={t('pages.xray.wireproxy.installButton')}
                cancelText={t('cancel')}
                onConfirm={install}
              >
                <Button type="primary" icon={<DownloadOutlined />} loading={loading}>
                  {t('pages.xray.wireproxy.installButton')}
                </Button>
              </Popconfirm>
              <Button icon={<ReloadOutlined />} loading={loading} onClick={fetchStatus}>
                {t('refresh')}
              </Button>
            </div>
          </>
        ) : (
          <>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
              {status?.service.active ? (
                <Tag color="green">{t('pages.xray.wireproxy.running')}</Tag>
              ) : (
                <Tag color="orange">{t('pages.xray.wireproxy.stoppedState')}</Tag>
              )}
              {status?.service.active ? (
                <Button danger icon={<PauseCircleOutlined />} loading={loading} onClick={stop}>
                  {t('pages.xray.wireproxy.stopButton')}
                </Button>
              ) : (
                <Button
                  type="primary"
                  icon={<PlayCircleOutlined />}
                  loading={loading}
                  onClick={start}
                >
                  {t('pages.xray.wireproxy.startButton')}
                </Button>
              )}
              <Button icon={<SyncOutlined />} loading={repairLoading} onClick={repair}>
                {t('pages.xray.wireproxy.repairButton')}
              </Button>
              <Button aria-label={t('refresh')} icon={<ReloadOutlined />} onClick={fetchStatus} />
              <Popconfirm
                title={t('pages.xray.wireproxy.uninstallConfirm')}
                okText={t('delete')}
                okType="danger"
                cancelText={t('cancel')}
                onConfirm={uninstall}
              >
                <Button danger icon={<DeleteOutlined />} loading={loading}>
                  {t('pages.xray.wireproxy.uninstallButton')}
                </Button>
              </Popconfirm>
            </div>

            {status?.routingGuard.danger && (
              <div
                style={{
                  marginTop: 8,
                  display: 'flex',
                  alignItems: 'center',
                  gap: 8,
                  flexWrap: 'wrap',
                }}
              >
                <Tag icon={<WarningOutlined />} color="red">
                  {t('pages.xray.wireproxy.routingDanger')}
                </Tag>
                <Button danger size="small" loading={loading} onClick={fixRouting}>
                  {t('pages.xray.wireproxy.fixRoutingButton')}
                </Button>
              </div>
            )}

            <div style={{ marginTop: 8, display: 'flex', gap: 8, flexWrap: 'wrap' }}>
              {status?.warp.on ? (
                <Tag color="green">
                  {t('pages.xray.wireproxy.warpOn', {
                    ip: status.warp.ip,
                    colo: status.warp.colo,
                    loc: status.warp.loc,
                  })}
                </Tag>
              ) : (
                <Tag color="orange">{t('pages.xray.wireproxy.warpOff')}</Tag>
              )}
              {status?.scheduler && status.scheduler !== 'none' && (
                <Tag color="blue">
                  {t('pages.xray.wireproxy.scheduler', { name: status.scheduler })}
                </Tag>
              )}
            </div>

            <Divider className="my-10">{t('pages.xray.outbound.outboundStatus')}</Divider>
            {wireproxyOutboundIndex >= 0 ? (
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
              {t('pages.xray.wireproxy.outboundHint', { port: status?.socks5.port })}
            </Typography.Paragraph>
          </>
        )}
      </Modal>
    </>
  );
}
