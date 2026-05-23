import { useTranslation } from 'react-i18next';
import { Button, Modal } from 'antd';
import { DownloadOutlined, UploadOutlined } from '@ant-design/icons';

import { HttpUtil, PromiseUtil } from '@/utils';
import './BackupModal.css';

interface BusyEvent {
  busy: boolean;
  tip?: string;
}

interface BackupModalProps {
  open: boolean;
  basePath: string;
  onClose: () => void;
  onBusy: (e: BusyEvent) => void;
}

export default function BackupModal({ open, basePath: _basePath, onClose, onBusy }: BackupModalProps) {
  const { t } = useTranslation();

  function exportDb() {
    window.location.href = (window.X_UI_BASE_PATH || '') + 'panel/api/server/getDb';
  }

  function importDb() {
    const fileInput = document.createElement('input');
    fileInput.type = 'file';
    fileInput.accept = '.db';
    fileInput.addEventListener('change', async (e) => {
      const dbFile = (e.target as HTMLInputElement).files?.[0];
      if (!dbFile) return;

      const formData = new FormData();
      formData.append('db', dbFile);

      onClose();
      onBusy({ busy: true, tip: `${t('pages.index.importDatabase')}…` });

      const upload = await HttpUtil.post('/panel/api/server/importDB', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      });
      if (!upload?.success) {
        onBusy({ busy: false });
        return;
      }

      onBusy({ busy: true, tip: `${t('pages.settings.restartPanel')}…` });
      const restart = await HttpUtil.post('/panel/setting/restartPanel');
      if (restart?.success) {
        await PromiseUtil.sleep(5000);
        window.location.reload();
      } else {
        onBusy({ busy: false });
      }
    });
    fileInput.click();
  }

  return (
    <Modal
      open={open}
      title={t('pages.index.backupTitle')}
      footer={null}
      onCancel={onClose}
    >
      <div className="backup-list">
        <div className="backup-item">
          <div className="backup-meta">
            <div className="backup-title">{t('pages.index.exportDatabase')}</div>
            <div className="backup-description">{t('pages.index.exportDatabaseDesc')}</div>
          </div>
          <Button type="primary" onClick={exportDb} icon={<DownloadOutlined />} />
        </div>

        <div className="backup-item">
          <div className="backup-meta">
            <div className="backup-title">{t('pages.index.importDatabase')}</div>
            <div className="backup-description">{t('pages.index.importDatabaseDesc')}</div>
          </div>
          <Button type="primary" onClick={importDb} icon={<UploadOutlined />} />
        </div>
      </div>
    </Modal>
  );
}
