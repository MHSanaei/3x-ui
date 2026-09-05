/* oxlint-disable react/set-state-in-effect -- same fetch-on-open shape as
   PsiphonModal.tsx/TorModal.tsx: the setState calls react to a real external
   input (the open prop flipping), at modal-open frequency, not a hot path. */
import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Divider, Modal, Popconfirm, Radio, Switch, Tag, Typography, message } from 'antd';
import { DownloadOutlined, PlayCircleOutlined, StopOutlined } from '@ant-design/icons';

import { HttpUtil, PromiseUtil } from '@/utils';

interface RunStatus {
  runId: string;
  state: 'pending' | 'running' | 'success' | 'failed';
  startedAt: number;
  endedAt?: number;
  tail?: string;
}

interface UnattendedUpgradesStatus {
  installed: boolean;
  enabled: boolean;
  mode: 'security' | 'full';
  autoReboot: boolean;
  lastRun: RunStatus;
}

interface UnattendedUpgradesModalProps {
  open: boolean;
  onClose: () => void;
}

export default function UnattendedUpgradesModal({ open, onClose }: UnattendedUpgradesModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);
  const [runLoading, setRunLoading] = useState(false);
  const [status, setStatus] = useState<UnattendedUpgradesStatus | null>(null);
  const [mode, setMode] = useState<'security' | 'full'>('security');
  const [autoReboot, setAutoReboot] = useState(false);

  const fetchStatus = useCallback(async () => {
    const msg = await HttpUtil.get<UnattendedUpgradesStatus>(
      '/panel/api/server/unattendedUpgrades/status',
    );
    if (msg?.success && msg.obj) {
      setStatus(msg.obj);
      setMode(msg.obj.mode);
      setAutoReboot(msg.obj.autoReboot);
    }
  }, []);

  useEffect(() => {
    if (!open) return;
    fetchStatus();
  }, [open, fetchStatus]);

  async function install() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/server/unattendedUpgrades/install');
      if (msg?.success) {
        messageApi.success(t('pages.index.unattendedUpgradesInstalled'));
      } else {
        messageApi.error(msg?.msg || t('pages.index.unattendedUpgradesInstallFailed'));
      }
      await fetchStatus();
    } finally {
      setLoading(false);
    }
  }

  async function save() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/server/unattendedUpgrades/configure', {
        mode,
        autoReboot,
      });
      if (msg?.success) {
        messageApi.success(t('pages.index.unattendedUpgradesConfigured'));
      } else {
        messageApi.error(msg?.msg || t('pages.index.unattendedUpgradesConfigureFailed'));
      }
      await fetchStatus();
    } finally {
      setLoading(false);
    }
  }

  async function disable() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/server/unattendedUpgrades/disable');
      if (msg?.success) messageApi.success(t('pages.index.unattendedUpgradesDisabled'));
      await fetchStatus();
    } finally {
      setLoading(false);
    }
  }

  async function pollRun(expectedRunId: string): Promise<RunStatus | null> {
    const deadline = Date.now() + 16 * 60_000; // a bit past the backend's own 15-minute run budget
    while (Date.now() < deadline) {
      await PromiseUtil.sleep(3000);
      const msg = await HttpUtil.get<UnattendedUpgradesStatus>(
        '/panel/api/server/unattendedUpgrades/status',
        undefined,
        { silent: true },
      );
      const lastRun = msg?.obj?.lastRun;
      if (lastRun?.runId === expectedRunId && lastRun.state !== 'running') {
        return lastRun;
      }
    }
    return null;
  }

  async function runNow() {
    setRunLoading(true);
    try {
      const msg = await HttpUtil.post<{ runId: string }>(
        '/panel/api/server/unattendedUpgrades/runNow',
      );
      if (!msg?.success || !msg.obj?.runId) {
        messageApi.error(msg?.msg || t('pages.index.unattendedUpgradesRunFailed'));
        return;
      }
      messageApi.info(t('pages.index.unattendedUpgradesRunStarted'));
      const result = await pollRun(msg.obj.runId);
      await fetchStatus();
      if (result?.state === 'success') {
        messageApi.success(t('pages.index.unattendedUpgradesRunSucceeded'));
      } else if (result?.state === 'failed') {
        messageApi.error(t('pages.index.unattendedUpgradesRunFailed'));
      }
    } finally {
      setRunLoading(false);
    }
  }

  const running = status?.lastRun?.state === 'running';

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={t('pages.index.unattendedUpgradesTitle')}
        footer={null}
        onCancel={onClose}
      >
        {status && !status.installed ? (
          <>
            <Tag color="orange">{t('pages.index.unattendedUpgradesNotInstalled')}</Tag>
            <p style={{ marginTop: 12 }}>{t('pages.index.unattendedUpgradesInstallHint')}</p>
            <Popconfirm
              title={t('pages.index.unattendedUpgradesInstallConfirm')}
              okText={t('confirm')}
              cancelText={t('cancel')}
              onConfirm={install}
            >
              <Button type="primary" icon={<DownloadOutlined />} loading={loading}>
                {t('pages.index.unattendedUpgradesInstallButton')}
              </Button>
            </Popconfirm>
          </>
        ) : (
          <>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              {status?.enabled ? (
                <Tag color="green">{t('enabled')}</Tag>
              ) : (
                <Tag color="orange">{t('disabled')}</Tag>
              )}
            </div>

            <Divider className="my-10">{t('pages.index.unattendedUpgradesModeTitle')}</Divider>
            <Radio.Group value={mode} onChange={(e) => setMode(e.target.value)}>
              <Radio.Button value="security">
                {t('pages.index.unattendedUpgradesModeSecurity')}
              </Radio.Button>
              <Radio.Button value="full">
                {t('pages.index.unattendedUpgradesModeFull')}
              </Radio.Button>
            </Radio.Group>
            <Typography.Paragraph type="secondary" style={{ marginTop: 8, fontSize: 12 }}>
              {mode === 'full'
                ? t('pages.index.unattendedUpgradesModeFullHint')
                : t('pages.index.unattendedUpgradesModeSecurityHint')}
            </Typography.Paragraph>

            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 12 }}>
              <Switch checked={autoReboot} onChange={setAutoReboot} />
              <span>{t('pages.index.unattendedUpgradesAutoReboot')}</span>
            </div>
            <Typography.Paragraph type="secondary" style={{ marginTop: 4, fontSize: 12 }}>
              {t('pages.index.unattendedUpgradesAutoRebootHint')}
            </Typography.Paragraph>

            <div style={{ display: 'flex', gap: 8, marginTop: 12, flexWrap: 'wrap' }}>
              <Button type="primary" loading={loading} onClick={save}>
                {t('save')}
              </Button>
              {status?.enabled && (
                <Popconfirm
                  title={t('pages.index.unattendedUpgradesDisableConfirm')}
                  okText={t('delete')}
                  okType="danger"
                  cancelText={t('cancel')}
                  onConfirm={disable}
                >
                  <Button danger icon={<StopOutlined />} loading={loading}>
                    {t('pages.index.unattendedUpgradesDisableButton')}
                  </Button>
                </Popconfirm>
              )}
              <Popconfirm
                title={t('pages.index.unattendedUpgradesRunConfirm')}
                okText={t('confirm')}
                cancelText={t('cancel')}
                onConfirm={runNow}
              >
                <Button icon={<PlayCircleOutlined />} loading={runLoading || running}>
                  {t('pages.index.unattendedUpgradesRunButton')}
                </Button>
              </Popconfirm>
            </div>

            {status?.lastRun?.runId && (
              <>
                <Divider className="my-10">
                  {t('pages.index.unattendedUpgradesLastRunTitle')}
                </Divider>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  {status.lastRun.state === 'running' && (
                    <Tag color="blue">{t('pages.index.unattendedUpgradesRunRunning')}</Tag>
                  )}
                  {status.lastRun.state === 'success' && (
                    <Tag color="green">{t('pages.index.unattendedUpgradesRunSucceeded')}</Tag>
                  )}
                  {status.lastRun.state === 'failed' && (
                    <Tag color="red">{t('pages.index.unattendedUpgradesRunFailed')}</Tag>
                  )}
                </div>
                {status.lastRun.tail && (
                  <pre
                    style={{
                      marginTop: 8,
                      maxHeight: 200,
                      overflow: 'auto',
                      fontSize: 12,
                      background: 'var(--ant-color-fill-quaternary, rgba(0,0,0,0.04))',
                      padding: 8,
                      borderRadius: 4,
                    }}
                  >
                    {status.lastRun.tail}
                  </pre>
                )}
              </>
            )}
          </>
        )}
      </Modal>
    </>
  );
}
