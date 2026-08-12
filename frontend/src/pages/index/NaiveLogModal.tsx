import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Checkbox, Modal, Select, Space } from 'antd';
import { DownloadOutlined, SyncOutlined } from '@ant-design/icons';

import { HttpUtil, FileManager } from '@/utils';
import { activateOnKey } from '@/utils/a11y';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import './LogModal.css';

interface NaiveLogModalProps {
  open: boolean;
  tag: string;
  onClose: () => void;
}

const AUTO_UPDATE_INTERVAL = 5000;

export default function NaiveLogModal({ open, tag, onClose }: NaiveLogModalProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();
  const [rows, setRows] = useState('100');
  const [autoUpdate, setAutoUpdate] = useState(false);
  const [loading, setLoading] = useState(false);
  const [logs, setLogs] = useState<string[]>([]);
  const refreshRef = useRef<() => Promise<void>>(async () => undefined);

  const refresh = useCallback(async () => {
    if (!open || !tag) return;
    setLoading(true);
    try {
      const msg = await HttpUtil.get<string[]>(
        `/panel/api/naive/logs/${encodeURIComponent(tag)}/${rows}`,
        undefined,
        { silent: true },
      );
      if (msg?.success) setLogs(msg.obj || []);
    } finally {
      setLoading(false);
    }
  }, [open, rows, tag]);

  useEffect(() => {
    refreshRef.current = refresh;
  }, [refresh]);

  useEffect(() => {
    if (open) refresh();
  }, [open, refresh]);

  useEffect(() => {
    if (!open || !autoUpdate) return;
    const id = setInterval(() => refreshRef.current(), AUTO_UPDATE_INTERVAL);
    return () => clearInterval(id);
  }, [open, autoUpdate]);

  function download() {
    FileManager.downloadTextFile(logs.join('\n'), `naive-${tag}.log`);
  }

  return (
    <Modal
      open={open}
      footer={null}
      width={isMobile ? '100vw' : 900}
      style={isMobile ? { top: 0, paddingBottom: 0, maxWidth: '100vw' } : undefined}
      className={isMobile ? 'logmodal-mobile' : undefined}
      onCancel={onClose}
      title={(
        <>
          {`Naive · ${tag} · ${t('pages.index.logs')}`}
          <SyncOutlined
            spin={loading}
            className="reload-icon"
            role="button"
            tabIndex={0}
            aria-label={t('refresh')}
            onClick={refresh}
            onKeyDown={activateOnKey(refresh)}
          />
        </>
      )}
    >
      <Space wrap style={{ width: '100%', marginBottom: 12 }}>
        <Select
          value={rows}
          size="small"
          style={{ width: 90 }}
          onChange={setRows}
          options={[
            { value: '20', label: '20' },
            { value: '50', label: '50' },
            { value: '100', label: '100' },
            { value: '500', label: '500' },
            { value: '1000', label: '1000' },
          ]}
        />
        <Checkbox checked={autoUpdate} onChange={(e) => setAutoUpdate(e.target.checked)}>
          {t('pages.index.autoUpdate')}
        </Checkbox>
        <Button type="primary" onClick={download} icon={<DownloadOutlined />} aria-label={t('download')} />
      </Space>

      <div className={`log-container ${isMobile ? 'log-container-mobile' : ''}`}>
        {logs.length === 0 ? (
          <div className="log-empty">No Record...</div>
        ) : (
          logs.map((line, index) => (
            <div key={`${index}-${line}`} className="log-line">
              <span>{line}</span>
            </div>
          ))
        )}
      </div>
    </Modal>
  );
}
