import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Checkbox, Form, Modal, Select, Space } from 'antd';
import { DownloadOutlined, SyncOutlined } from '@ant-design/icons';

import { HttpUtil, FileManager, PromiseUtil } from '@/utils';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { parseLogLine } from './logParse';
import './LogModal.css';

interface LogModalProps {
  open: boolean;
  onClose: () => void;
}

const AUTO_UPDATE_INTERVAL = 5000;

export default function LogModal({ open, onClose }: LogModalProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();
  const [rows, setRows] = useState('20');
  const [level, setLevel] = useState('info');
  const [syslog, setSyslog] = useState(false);
  const [autoUpdate, setAutoUpdate] = useState(false);
  const [loading, setLoading] = useState(false);
  const [logs, setLogs] = useState<string[]>([]);
  const openRef = useRef(open);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const msg = await HttpUtil.post<string[]>(`/panel/api/server/logs/${rows}`, {
        level,
        syslog,
      });
      if (msg?.success) {
        setLogs(msg.obj || []);
      }
      await PromiseUtil.sleep(300);
    } finally {
      setLoading(false);
    }
  }, [rows, level, syslog]);

  const refreshRef = useRef(refresh);
  useEffect(() => {
    refreshRef.current = refresh;
  }, [refresh]);

  useEffect(() => {
    openRef.current = open;
    if (open) refresh();
  }, [open, refresh]);

  useEffect(() => {
    if (openRef.current) refresh();
  }, [rows, level, syslog, refresh]);

  useEffect(() => {
    if (!open || !autoUpdate) return;
    const id = setInterval(() => refreshRef.current(), AUTO_UPDATE_INTERVAL);
    return () => clearInterval(id);
  }, [open, autoUpdate]);

  const parsedLogs = useMemo(() => logs.map(parseLogLine), [logs]);

  function download() {
    FileManager.downloadTextFile(logs.join('\n'), 'x-ui.log');
  }

  const titleNode = (
    <>
      {t('pages.index.logs')}
      <SyncOutlined spin={loading} className="reload-icon" onClick={refresh} />
    </>
  );

  return (
    <Modal
      open={open}
      footer={null}
      width={isMobile ? '100vw' : 800}
      style={isMobile ? { top: 0, paddingBottom: 0, maxWidth: '100vw' } : undefined}
      className={isMobile ? 'logmodal-mobile' : undefined}
      onCancel={onClose}
      title={titleNode}
    >
      <Form layout="inline" className="log-toolbar">
        <Form.Item>
          <Space.Compact>
            <Select
              value={rows}
              size="small"
              style={{ width: 70 }}
              onChange={setRows}
              options={[
                { value: '20', label: '20' },
                { value: '50', label: '50' },
                { value: '100', label: '100' },
                { value: '500', label: '500' },
                { value: '1000', label: '1000' },
              ]}
            />
            <Select
              value={level}
              size="small"
              style={{ width: 95 }}
              onChange={setLevel}
              options={[
                { value: 'debug', label: 'Debug' },
                { value: 'info', label: 'Info' },
                { value: 'notice', label: 'Notice' },
                { value: 'warning', label: 'Warning' },
                { value: 'err', label: 'Error' },
              ]}
            />
          </Space.Compact>
        </Form.Item>
        <Form.Item>
          <Checkbox checked={syslog} onChange={(e) => setSyslog(e.target.checked)}>
            SysLog
          </Checkbox>
          <Checkbox checked={autoUpdate} onChange={(e) => setAutoUpdate(e.target.checked)}>
            {t('pages.index.autoUpdate')}
          </Checkbox>
        </Form.Item>
        <Form.Item className="download-item">
          <Button type="primary" onClick={download} icon={<DownloadOutlined />} />
        </Form.Item>
      </Form>

      <div className={`log-container ${isMobile ? 'log-container-mobile' : ''}`}>
        {parsedLogs.length === 0 ? (
          <div className="log-empty">No Record...</div>
        ) : isMobile ? (
          parsedLogs.map((log, idx) => (
            <div key={idx} className="log-card">
              <div className="log-card-head">
                {log.stamp && (
                  <span className="log-time">
                    {log.time && <span>{log.time}</span>}
                    {log.time && log.date ? ' ' : ''}
                    {log.date && <span className="log-date">{log.date}</span>}
                  </span>
                )}
                {log.levelText && (
                  <span className={`log-level-badge ${log.levelClass}`}>{log.levelText}</span>
                )}
              </div>
              {(log.body || log.service) && (
                <div className="log-body">
                  {log.service && <b>{log.service}</b>}
                  {log.service && log.body ? ' ' : ''}
                  {log.body && <span className="log-body-text">{log.body}</span>}
                </div>
              )}
            </div>
          ))
        ) : (
          parsedLogs.map((log, idx) => (
            <div key={idx} className="log-line">
              {log.stamp && <span className="log-stamp">{log.stamp}</span>}
              {log.stamp && log.levelText ? ' ' : ''}
              {log.levelText && <span className={`log-level ${log.levelClass}`}>{log.levelText}</span>}
              {(log.body || log.service) && (
                <>
                  {(log.stamp || log.levelText) && <span> - </span>}
                  {log.service && <b>{log.service}</b>}
                  {log.service && log.body ? ' ' : ''}
                  <span>{log.body}</span>
                </>
              )}
            </div>
          ))
        )}
      </div>
    </Modal>
  );
}
