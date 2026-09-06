import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Modal, Radio, Space, Switch, Tag } from 'antd';
import { CloudDownloadOutlined, RollbackOutlined } from '@ant-design/icons';

import { HttpUtil, PromiseUtil } from '@/utils';
import { formatPanelVersion } from '@/lib/panel-version';
import type { PanelUpdateStatus } from '@/generated/types';
import './PanelUpdateModal.css';

type UpdateOutcome = 'success' | 'failed' | 'timeout';

export interface PanelUpdateInfo {
  channel?: string;
  currentVersion: string;
  latestVersion: string;
  currentCommit?: string;
  latestCommit?: string;
  updateAvailable: boolean;
  localSnapshotVersion?: string;
  latestStableVersion?: string;
  hasLocalSnapshot?: boolean;
}

interface BusyEvent {
  busy: boolean;
  tip?: string;
}

interface PanelUpdateModalProps {
  open: boolean;
  info: PanelUpdateInfo;
  devChannelEnable?: boolean;
  onChannelChange?: (dev: boolean) => void | Promise<void>;
  onClose: () => void;
  onBusy: (e: BusyEvent) => void;
}

function RollbackConfirmContent({
  info,
  onModeChange,
}: {
  info: PanelUpdateInfo;
  onModeChange: (mode: string) => void;
}) {
  const { t } = useTranslation();
  const [selected, setSelected] = useState(info.hasLocalSnapshot ? 'local' : 'online');
  const targetVer =
    (selected === 'local' ? info.localSnapshotVersion : info.latestStableVersion) ||
    info.latestStableVersion ||
    '';

  return (
    <div className="rollback-dialog-choices">
      <p className="mb-12">{t('pages.index.rollbackDialogDesc').replace('#version#', targetVer)}</p>
      <Radio.Group
        value={selected}
        onChange={(e) => {
          const next = e.target.value;
          setSelected(next);
          onModeChange(next);
        }}
      >
        <Space orientation="vertical">
          <Radio value="local">
            {t('pages.index.rollbackOptionLocal').replace(
              '#version#',
              info.localSnapshotVersion || '',
            )}
          </Radio>
          <Radio value="online">
            {t('pages.index.rollbackOptionOnline').replace(
              '#version#',
              info.latestStableVersion || '',
            )}
          </Radio>
        </Space>
      </Radio.Group>
    </div>
  );
}

export default function PanelUpdateModal({
  open,
  info,
  devChannelEnable,
  onChannelChange,
  onClose,
  onBusy,
}: PanelUpdateModalProps) {
  const { t } = useTranslation();
  const [modal, contextHolder] = Modal.useModal();
  const [channelBusy, setChannelBusy] = useState(false);

  const isDev = info.channel === 'dev';

  async function pollUpdateStatus(expectedRunId: string): Promise<UpdateOutcome> {
    await PromiseUtil.sleep(5000);
    const deadline = Date.now() + 90_000;
    while (Date.now() < deadline) {
      try {
        const msg = await HttpUtil.get<PanelUpdateStatus>(
          '/panel/api/server/getUpdateStatus',
          undefined,
          { silent: true, timeout: 2000 },
        );
        const status = msg?.obj ?? undefined;
        if (status?.runId === expectedRunId) {
          if (status.state === 'success') return 'success';
          if (status.state === 'failed') return 'failed';
        }
      } catch {
        /* still restarting */
      }
      await PromiseUtil.sleep(2000);
    }
    return 'timeout';
  }

  async function handleChannel(checked: boolean) {
    if (!onChannelChange) return;
    setChannelBusy(true);
    try {
      await onChannelChange(checked);
    } finally {
      setChannelBusy(false);
    }
  }

  function updatePanel() {
    modal.confirm({
      title: t('pages.index.panelUpdateDialog'),
      content: t('pages.index.panelUpdateDialogDesc').replace(
        '#version#',
        info.latestVersion || '',
      ),
      okText: t('confirm'),
      cancelText: t('cancel'),
      onOk: async () => {
        const baseTip = t('pages.index.dontRefresh');
        const tip = info.latestVersion ? `${baseTip} (${info.latestVersion})` : baseTip;
        onClose();
        onBusy({ busy: true, tip });
        const result = await HttpUtil.post<{ runId: string }>('/panel/api/server/updatePanel');
        if (!result?.success) {
          onBusy({ busy: false });
          return;
        }
        const outcome = await pollUpdateStatus(result.obj?.runId ?? '');
        onBusy({ busy: false });
        if (outcome === 'success') {
          await PromiseUtil.sleep(800);
          window.location.reload();
          return;
        }
        modal[outcome === 'failed' ? 'error' : 'warning']({
          title: t(
            outcome === 'failed'
              ? 'pages.index.panelUpdateFailedTitle'
              : 'pages.index.panelUpdateUnknownTitle',
          ),
          content: t(
            outcome === 'failed'
              ? 'pages.index.panelUpdateFailedDesc'
              : 'pages.index.panelUpdateUnknownDesc',
          ),
          okText: t('refresh'),
          onOk: () => window.location.reload(),
        });
      },
    });
  }

  function handleRollback() {
    let selectedMode = info.hasLocalSnapshot ? 'local' : 'online';
    const hasDiffChoices =
      info.hasLocalSnapshot &&
      !!info.localSnapshotVersion &&
      !!info.latestStableVersion &&
      info.localSnapshotVersion !== info.latestStableVersion;

    const singleTargetVer =
      (selectedMode === 'local' ? info.localSnapshotVersion : info.latestStableVersion) ||
      info.latestStableVersion ||
      '';

    const dialogContent = hasDiffChoices ? (
      <RollbackConfirmContent
        info={info}
        onModeChange={(m) => {
          selectedMode = m;
        }}
      />
    ) : (
      t('pages.index.rollbackDialogDesc').replace('#version#', singleTargetVer)
    );

    modal.confirm({
      title: t('pages.index.rollbackDialogTitle'),
      content: dialogContent,
      okText: t('confirm'),
      cancelText: t('cancel'),
      onOk: async () => {
        const baseTip = t('pages.index.dontRefresh');
        onClose();
        onBusy({ busy: true, tip: baseTip });
        const result = await HttpUtil.post<{ runId: string }>('/panel/api/server/rollbackPanel', {
          mode: selectedMode,
        });
        if (!result?.success) {
          onBusy({ busy: false });
          return;
        }
        const outcome = await pollUpdateStatus(result.obj?.runId ?? '');
        onBusy({ busy: false });
        if (outcome === 'success') {
          await PromiseUtil.sleep(800);
          window.location.reload();
          return;
        }
        modal[outcome === 'failed' ? 'error' : 'warning']({
          title: t(
            outcome === 'failed'
              ? 'pages.index.panelUpdateFailedTitle'
              : 'pages.index.panelUpdateUnknownTitle',
          ),
          content: t(
            outcome === 'failed'
              ? 'pages.index.panelUpdateFailedDesc'
              : 'pages.index.panelUpdateUnknownDesc',
          ),
          okText: t('refresh'),
          onOk: () => window.location.reload(),
        });
      },
    });
  }

  return (
    <>
      {contextHolder}
      <Modal open={open} title={t('pages.index.updatePanel')} footer={null} onCancel={onClose}>
        {info.updateAvailable && (
          <Alert
            type="warning"
            className="mb-12"
            title={t('pages.index.panelUpdateDesc')}
            showIcon
          />
        )}

        <div className="version-list">
          <div className="version-list-item">
            <span>{t('pages.index.devChannel')}</span>
            <Switch checked={!!devChannelEnable} loading={channelBusy} onChange={handleChannel} />
          </div>
        </div>

        {devChannelEnable && (
          <Alert
            type="info"
            className="mb-12"
            title={t('pages.index.devChannelWarning')}
            showIcon
          />
        )}

        <div className="version-list">
          <div className="version-list-item">
            <span>
              {isDev ? t('pages.index.currentCommit') : t('pages.index.currentPanelVersion')}
            </span>
            {isDev ? (
              <Tag color="green">{info.currentCommit || '?'}</Tag>
            ) : (
              <Tag color="green">
                {formatPanelVersion(window.X_UI_CUR_VER || info.currentVersion) || '?'}
              </Tag>
            )}
          </div>
          {info.updateAvailable ? (
            <div className="version-list-item">
              <span>
                {isDev ? t('pages.index.latestCommit') : t('pages.index.latestPanelVersion')}
              </span>
              <Tag color="purple">{(isDev ? info.latestCommit : info.latestVersion) || '-'}</Tag>
            </div>
          ) : (
            <div className="version-list-item">
              <span>{t('pages.index.panelUpToDate')}</span>
              <Tag color="green">{t('pages.index.panelUpToDate')}</Tag>
            </div>
          )}
        </div>

        <div className="actions-row">
          {isDev && (
            <Button danger onClick={handleRollback} icon={<RollbackOutlined />} className="mr-auto">
              {t('pages.index.rollbackToRelease')}
            </Button>
          )}
          <Button
            type="primary"
            disabled={!info.updateAvailable}
            onClick={updatePanel}
            icon={<CloudDownloadOutlined />}
          >
            {t('pages.index.updatePanel')}
          </Button>
        </div>
      </Modal>
    </>
  );
}
