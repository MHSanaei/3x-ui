import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Modal, Switch, Tag } from 'antd';
import { CloudDownloadOutlined } from '@ant-design/icons';
import axios from 'axios';

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
        const r = await axios.get('/panel/api/server/getUpdateStatus', { timeout: 2000 });
        const status = r?.data?.obj as PanelUpdateStatus | undefined;
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
      content: t('pages.index.panelUpdateDialogDesc').replace('#version#', info.latestVersion || ''),
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
          title: t(outcome === 'failed' ? 'pages.index.panelUpdateFailedTitle' : 'pages.index.panelUpdateUnknownTitle'),
          content: t(outcome === 'failed' ? 'pages.index.panelUpdateFailedDesc' : 'pages.index.panelUpdateUnknownDesc'),
          okText: t('refresh'),
          onOk: () => window.location.reload(),
        });
      },
    });
  }

  return (
    <>
      {contextHolder}
      <Modal
        open={open}
        title={t('pages.index.updatePanel')}
        footer={null}
        onCancel={onClose}
      >
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
            <Switch
              checked={!!devChannelEnable}
              loading={channelBusy}
              onChange={handleChannel}
            />
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
            <span>{isDev ? t('pages.index.currentCommit') : t('pages.index.currentPanelVersion')}</span>
            {isDev ? (
              <Tag color="green">{info.currentCommit || '?'}</Tag>
            ) : (
              <Tag color="green">{formatPanelVersion(window.X_UI_CUR_VER || info.currentVersion) || '?'}</Tag>
            )}
          </div>
          {info.updateAvailable ? (
            <div className="version-list-item">
              <span>{isDev ? t('pages.index.latestCommit') : t('pages.index.latestPanelVersion')}</span>
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
