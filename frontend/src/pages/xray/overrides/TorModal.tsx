import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Divider, Modal, Popconfirm, Tag, message } from 'antd';
import { PlayCircleOutlined, PauseCircleOutlined, ReloadOutlined, DownloadOutlined, DeleteOutlined, GlobalOutlined, SwapOutlined } from '@ant-design/icons';

import { HttpUtil } from '@/utils';

interface TorModalProps {
  open: boolean;
  templateSettings: { outbounds?: { tag?: string }[] } | null;
  onClose: () => void;
  onAddOutbound: (outbound: Record<string, unknown>) => void;
  onRemoveOutbound: (tag: string) => void;
}

interface TorStatus {
  available: boolean;
  running: boolean;
  port: number;
  lastLog?: string;
}

interface TorExitIP {
  ip: string;
  isTor: boolean;
}

const TOR_TAG = 'tor';

export default function TorModal({ open, templateSettings, onClose, onAddOutbound, onRemoveOutbound }: TorModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState<TorStatus | null>(null);
  const [ipLoading, setIpLoading] = useState(false);
  const [exitIp, setExitIp] = useState<TorExitIP | null>(null);

  const torOutboundIndex = useMemo(() => {
    const list = templateSettings?.outbounds;
    if (!list) return -1;
    return list.findIndex((o) => o?.tag === TOR_TAG);
  }, [templateSettings?.outbounds]);

  const fetchStatus = useCallback(async () => {
    const msg = await HttpUtil.post<TorStatus>('/panel/api/xray/tor/status');
    if (msg?.success && msg.obj) setStatus(msg.obj);
  }, []);

  useEffect(() => {
    if (open) fetchStatus();
  }, [open, fetchStatus]);

  async function start() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/xray/tor/start');
      if (msg?.success) {
        messageApi.success(t('pages.xray.tor.started'));
      } else {
        messageApi.error(msg?.msg || t('pages.xray.tor.startFailed'));
      }
      await fetchStatus();
    } finally {
      setLoading(false);
    }
  }

  async function stop() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/xray/tor/stop');
      if (msg?.success) messageApi.success(t('pages.xray.tor.stopped'));
      await fetchStatus();
    } finally {
      setLoading(false);
    }
  }

  async function install() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/xray/tor/install');
      if (msg?.success) {
        messageApi.success(t('pages.xray.tor.installed'));
      } else {
        messageApi.error(msg?.msg || t('pages.xray.tor.installFailed'));
      }
      await fetchStatus();
    } finally {
      setLoading(false);
    }
  }

  async function uninstall() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/xray/tor/uninstall');
      if (msg?.success) {
        messageApi.success(t('pages.xray.tor.uninstalled'));
        if (torOutboundIndex >= 0) onRemoveOutbound(TOR_TAG);
      } else {
        messageApi.error(msg?.msg || t('pages.xray.tor.uninstallFailed'));
      }
      await fetchStatus();
    } finally {
      setLoading(false);
    }
  }

  async function checkIp() {
    setIpLoading(true);
    setExitIp(null);
    try {
      const msg = await HttpUtil.post<TorExitIP>('/panel/api/xray/tor/ip');
      if (msg?.success && msg.obj) {
        setExitIp(msg.obj);
        if (!msg.obj.isTor) messageApi.warning(t('pages.xray.tor.checkIpNotTor'));
      } else {
        messageApi.error(msg?.msg || t('pages.xray.tor.checkIpFailed'));
      }
    } finally {
      setIpLoading(false);
    }
  }

  async function newIdentity() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/xray/tor/newIdentity');
      if (msg?.success) {
        messageApi.success(t('pages.xray.tor.newIdentitySent'));
        setExitIp(null);
      } else {
        messageApi.error(msg?.msg || t('pages.xray.tor.newIdentityFailed'));
      }
    } finally {
      setLoading(false);
    }
  }

  function addOutbound() {
    if (!status) return;
    onAddOutbound({
      tag: TOR_TAG,
      protocol: 'socks',
      settings: { servers: [{ address: '127.0.0.1', port: status.port }] },
    });
    messageApi.success(t('pages.xray.tor.outboundAdded'));
    onClose();
  }

  function removeOutbound() {
    onRemoveOutbound(TOR_TAG);
    messageApi.success(t('pages.xray.tor.outboundRemoved'));
  }

  return (
    <>
      {messageContextHolder}
      <Modal open={open} title={t('pages.xray.tor.title')} footer={null} onCancel={onClose}>
        {status && !status.available ? (
          <>
            <Tag color="red">{t('pages.xray.tor.notInstalled')}</Tag>
            <p style={{ marginTop: 12 }}>{t('pages.xray.tor.installHint')}</p>
            <div style={{ display: 'flex', gap: 8 }}>
              <Button type="primary" icon={<DownloadOutlined />} loading={loading} onClick={install}>
                {t('pages.xray.tor.installButton')}
              </Button>
              <Button icon={<ReloadOutlined />} loading={loading} onClick={fetchStatus}>
                {t('refresh')}
              </Button>
            </div>
            <p style={{ marginTop: 12, fontSize: 12, color: '#888' }}>{t('pages.xray.tor.installManualHint')}</p>
            <pre style={{ background: 'var(--ant-color-fill-tertiary, #f5f5f5)', padding: 8, borderRadius: 4 }}>
              apt install tor{'\n'}dnf install tor{'\n'}pacman -S tor
            </pre>
          </>
        ) : (
          <>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              {status?.running ? (
                <Tag color="green">{t('pages.xray.tor.running')}</Tag>
              ) : (
                <Tag color="orange">{t('pages.xray.tor.stoppedState')}</Tag>
              )}
              {status?.running ? (
                <Button danger icon={<PauseCircleOutlined />} loading={loading} onClick={stop}>
                  {t('pages.xray.tor.stopButton')}
                </Button>
              ) : (
                <Button type="primary" icon={<PlayCircleOutlined />} loading={loading} onClick={start}>
                  {t('pages.xray.tor.startButton')}
                </Button>
              )}
              <Button aria-label={t('refresh')} icon={<ReloadOutlined />} onClick={fetchStatus} />
              <Popconfirm
                title={t('pages.xray.tor.uninstallConfirm')}
                okText={t('delete')}
                okType="danger"
                cancelText={t('cancel')}
                onConfirm={uninstall}
              >
                <Button danger icon={<DeleteOutlined />} loading={loading}>
                  {t('pages.xray.tor.uninstallButton')}
                </Button>
              </Popconfirm>
            </div>
            {status?.lastLog && <div style={{ fontSize: 12, color: '#888', marginTop: 8 }}>{status.lastLog}</div>}

            {status?.running && (
              <>
                <Divider className="my-10">{t('pages.xray.tor.exitIpTitle')}</Divider>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                  <Button icon={<GlobalOutlined />} loading={ipLoading} onClick={checkIp}>
                    {t('pages.xray.tor.checkIpButton')}
                  </Button>
                  <Button icon={<SwapOutlined />} loading={loading} onClick={newIdentity}>
                    {t('pages.xray.tor.newIdentityButton')}
                  </Button>
                  {exitIp && (
                    <Tag color={exitIp.isTor ? 'green' : 'red'}>
                      {exitIp.ip}
                    </Tag>
                  )}
                </div>
                <p style={{ marginTop: 8, fontSize: 12, color: '#888' }}>{t('pages.xray.tor.newIdentityHint')}</p>
              </>
            )}

            <Divider className="my-10">{t('pages.xray.outbound.outboundStatus')}</Divider>
            {torOutboundIndex >= 0 ? (
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
            <p style={{ marginTop: 12, fontSize: 12, color: '#888' }}>{t('pages.xray.tor.testButtonHint')}</p>
          </>
        )}
      </Modal>
    </>
  );
}
