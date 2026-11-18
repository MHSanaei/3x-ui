import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Modal } from 'antd';
import { UploadOutlined } from '@ant-design/icons';

import { HttpUtil } from '@/utils';

interface PanelUpdateModalProps {
  open: boolean;
  onClose: () => void;
  onBusy: (state: { busy: boolean; tip?: string }) => void;
}

type UpdateOutcome = 'success' | 'failed' | 'unknown';

const MAX_BYTES = 256 * 1024 * 1024;

async function pollUpdateStatus(runId: string): Promise<UpdateOutcome> {
  const deadline = Date.now() + 180_000;
  while (Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, 1500));
    const response = await HttpUtil.get<{ runId: string; state: string }>(
      '/panel/api/server/getUpdateStatus',
      undefined,
      { silent: true, timeout: 4000 },
    );
    if (response?.obj?.runId !== runId) continue;
    if (response.obj.state === 'success' || response.obj.state === 'failed') {
      return response.obj.state;
    }
  }
  return 'unknown';
}

export default function PanelUpdateModal({ open, onClose, onBusy }: PanelUpdateModalProps) {
  const { t } = useTranslation();
  const [modal, contextHolder] = Modal.useModal();
  const [file, setFile] = useState<File>();
  const [busy, setBusy] = useState(false);
  const valid = !!file && file.size > 0 && file.size <= MAX_BYTES && /\.zip$/i.test(file.name);

  async function applyPackage() {
    if (!file || !valid || busy) return;
    setBusy(true);
    onBusy({ busy: true, tip: t('pages.index.offlineUpdateApplying') });
    try {
      const response = await HttpUtil.post<{ runId: string }>(
        '/panel/api/server/uploadUpdate',
        file,
        { headers: { 'Content-Type': 'application/zip' }, timeout: 180_000 },
      );
      if (!response?.success || !response.obj?.runId) return;
      const outcome = await pollUpdateStatus(response.obj.runId);
      onClose();
      modal.info({
        title: t('pages.index.offlineUpdate', { defaultValue: 'Offline update' }),
        content: t(
          outcome === 'success'
            ? 'pages.index.offlineUpdateSuccess'
            : outcome === 'failed'
              ? 'pages.index.offlineUpdateFailed'
              : 'pages.index.panelUpdateUnknownDesc',
        ),
        okText: t('refresh'),
        onOk: () => window.location.reload(),
      });
    } finally {
      setBusy(false);
      onBusy({ busy: false });
    }
  }

  return (
    <>
      {contextHolder}
      <Modal
        open={open}
        title={t('pages.index.offlineUpdate', { defaultValue: 'Offline update' })}
        footer={null}
        onCancel={busy ? undefined : onClose}
        closable={!busy}
        mask={{ closable: !busy }}
      >
        <Alert type="info" showIcon title={t('pages.index.offlineUpdateOnly')} />
        <div className="actions-row">
          <input
            type="file"
            accept=".zip,application/zip"
            aria-label={t('pages.index.offlineUpdateSelect')}
            disabled={busy}
            onChange={(event) => setFile(event.target.files?.[0])}
          />
          <Button
            type="primary"
            loading={busy}
            disabled={!valid}
            onClick={applyPackage}
            icon={<UploadOutlined aria-hidden />}
          >
            {t('pages.index.offlineUpdateApply')}
          </Button>
        </div>
        {file && !valid && <Alert type="error" title={t('pages.index.offlineUpdateTooLarge')} />}
      </Modal>
    </>
  );
}
