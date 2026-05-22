import { useTranslation } from 'react-i18next';
import { Alert, Button, Modal, Tag } from 'antd';
import { CloudDownloadOutlined } from '@ant-design/icons';
import axios from 'axios';

import { HttpUtil, PromiseUtil } from '@/utils';
import './PanelUpdateModal.css';

export interface PanelUpdateInfo {
  currentVersion: string;
  latestVersion: string;
  updateAvailable: boolean;
}

interface BusyEvent {
  busy: boolean;
  tip?: string;
}

interface PanelUpdateModalProps {
  open: boolean;
  info: PanelUpdateInfo;
  onClose: () => void;
  onBusy: (e: BusyEvent) => void;
}

export default function PanelUpdateModal({ open, info, onClose, onBusy }: PanelUpdateModalProps) {
  const { t } = useTranslation();
  const [modal, contextHolder] = Modal.useModal();

  async function pollUntilBack(): Promise<boolean> {
    await PromiseUtil.sleep(5000);
    const deadline = Date.now() + 90_000;
    while (Date.now() < deadline) {
      try {
        const r = await axios.get('/panel/api/server/status', { timeout: 2000 });
        if (r?.data?.success) return true;
      } catch {
        /* still restarting */
      }
      await PromiseUtil.sleep(2000);
    }
    return false;
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
        const result = await HttpUtil.post('/panel/api/server/updatePanel');
        if (!result?.success) {
          onBusy({ busy: false });
          return;
        }
        const back = await pollUntilBack();
        if (back) await PromiseUtil.sleep(800);
        window.location.reload();
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
            <span>{t('pages.index.currentPanelVersion')}</span>
            <Tag color="green">v{info.currentVersion || '?'}</Tag>
          </div>
          {info.updateAvailable ? (
            <div className="version-list-item">
              <span>{t('pages.index.latestPanelVersion')}</span>
              <Tag color="purple">{info.latestVersion || '-'}</Tag>
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
