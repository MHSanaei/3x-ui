import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Checkbox, Form, Input, Modal, Select, Tag } from 'antd';
import { DownloadOutlined, SyncOutlined } from '@ant-design/icons';

import { HttpUtil, FileManager, IntlUtil, PromiseUtil } from '@/utils';
import { activateOnKey } from '@/utils/a11y';
import { useDatepicker } from '@/hooks/useDatepicker';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import './XrayLogModal.css';

interface XrayLogModalProps {
  open: boolean;
  onClose: () => void;
}

interface XrayLogEntry {
  DateTime?: string | number;
  FromAddress?: string;
  ToAddress?: string;
  Inbound?: string;
  Outbound?: string;
  Email?: string;
  Event?: number;
}

const EVENT_LABELS: Record<number, string> = { 0: 'DIRECT', 1: 'BLOCKED', 2: 'PROXY' };
const EVENT_COLORS: Record<number, string> = { 0: 'green', 1: 'red', 2: 'blue' };

function eventLabel(ev?: number): string {
  return EVENT_LABELS[ev ?? -1] ?? String(ev ?? '');
}

function eventColor(ev?: number): string {
  return EVENT_COLORS[ev ?? -1] ?? 'default';
}

function shortTime(value?: string | number): string {
  if (!value) return '';
  const d = new Date(value);
  if (isNaN(d.getTime())) return '';
  const hh = String(d.getHours()).padStart(2, '0');
  const mm = String(d.getMinutes()).padStart(2, '0');
  const ss = String(d.getSeconds()).padStart(2, '0');
  return `${hh}:${mm}:${ss}`;
}

const AUTO_UPDATE_INTERVAL = 5000;

export default function XrayLogModal({ open, onClose }: XrayLogModalProps) {
  const { t } = useTranslation();
  const { datepicker } = useDatepicker();
  const { isMobile } = useMediaQuery();
  const [rows, setRows] = useState('20');
  const [filter, setFilter] = useState('');
  const [showDirect, setShowDirect] = useState(true);
  const [showBlocked, setShowBlocked] = useState(true);
  const [showProxy, setShowProxy] = useState(true);
  const [autoUpdate, setAutoUpdate] = useState(false);
  const [loading, setLoading] = useState(false);
  const [logs, setLogs] = useState<XrayLogEntry[]>([]);
  const openRef = useRef(open);

  const orderedLogs = useMemo(() => [...logs].reverse(), [logs]);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const msg = await HttpUtil.post<XrayLogEntry[]>(`/panel/api/server/xraylogs/${rows}`, {
        filter,
        showDirect,
        showBlocked,
        showProxy,
      });
      if (msg?.success) setLogs(msg.obj || []);
      await PromiseUtil.sleep(300);
    } finally {
      setLoading(false);
    }
  }, [rows, filter, showDirect, showBlocked, showProxy]);

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
  }, [rows, showDirect, showBlocked, showProxy, refresh]);

  useEffect(() => {
    if (!open || !autoUpdate) return;
    const id = setInterval(() => refreshRef.current(), AUTO_UPDATE_INTERVAL);
    return () => clearInterval(id);
  }, [open, autoUpdate]);

  function fullDate(value?: string | number): string {
    return IntlUtil.formatDate(value, datepicker);
  }

  function download() {
    if (!Array.isArray(logs) || logs.length === 0) {
      FileManager.downloadTextFile('', 'x-ui.log');
      return;
    }
    const lines = logs.map((l) => {
      try {
        const dt = l.DateTime ? new Date(l.DateTime) : null;
        const dateStr = dt && !isNaN(dt.getTime()) ? dt.toISOString() : '';
        const eventText = eventLabel(l.Event);
        const emailPart = l.Email ? ` Email=${l.Email}` : '';
        return `${dateStr} FROM=${l.FromAddress || ''} TO=${l.ToAddress || ''} INBOUND=${l.Inbound || ''} OUTBOUND=${l.Outbound || ''}${emailPart} EVENT=${eventText}`.trim();
      } catch {
        return JSON.stringify(l);
      }
    }).join('\n');
    FileManager.downloadTextFile(lines, 'x-ui.log');
  }

  return (
    <Modal
      open={open}
      footer={null}
      width={isMobile ? '100vw' : '80vw'}
      style={isMobile ? { top: 0, paddingBottom: 0, maxWidth: '100vw' } : undefined}
      className={isMobile ? 'xraylog-modal-mobile' : undefined}
      onCancel={onClose}
      title={
        <>
          {t('pages.index.accessLogs')}
          <SyncOutlined spin={loading} className="reload-icon" role="button" tabIndex={0} aria-label={t('refresh')} onClick={refresh} onKeyDown={activateOnKey(refresh)} />
        </>
      }
    >
      <Form layout="inline" className="log-toolbar">
        <Form.Item>
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
        </Form.Item>
        <Form.Item label={t('filter')} className="filter-item">
          <Input
            value={filter}
            size="small"
            onChange={(e) => setFilter(e.target.value)}
            onKeyUp={(e) => {
              if (e.key === 'Enter') refresh();
            }}
          />
        </Form.Item>
        <Form.Item>
          <Checkbox checked={showDirect} onChange={(e) => setShowDirect(e.target.checked)}>
            Direct
          </Checkbox>
          <Checkbox checked={showBlocked} onChange={(e) => setShowBlocked(e.target.checked)}>
            Blocked
          </Checkbox>
          <Checkbox checked={showProxy} onChange={(e) => setShowProxy(e.target.checked)}>
            Proxy
          </Checkbox>
          <Checkbox checked={autoUpdate} onChange={(e) => setAutoUpdate(e.target.checked)}>
            {t('pages.index.autoUpdate')}
          </Checkbox>
        </Form.Item>
        <Form.Item className="download-item">
          <Button type="primary" onClick={download} icon={<DownloadOutlined />} aria-label={t('download')} />
        </Form.Item>
      </Form>

      <div className={`log-container ${isMobile ? 'log-container-mobile' : ''}`}>
        {orderedLogs.length === 0 ? (
          <div className="log-empty">No Record...</div>
        ) : isMobile ? (
          orderedLogs.map((log, idx) => (
            <div key={idx} className="log-card">
              <div className="log-card-head">
                <span className="log-time" title={fullDate(log.DateTime)}>
                  {shortTime(log.DateTime)}
                </span>
                <Tag color={eventColor(log.Event)} className="log-event-tag">
                  {eventLabel(log.Event)}
                </Tag>
              </div>
              <div className="log-route">
                <span className="log-addr">{log.FromAddress}</span>
                <span className="log-arrow">→</span>
                <span className="log-addr">{log.ToAddress}</span>
              </div>
              <div className="log-meta">
                {log.Inbound && (
                  <span className="log-meta-pair">
                    <span className="log-meta-key">in</span>
                    <span className="log-meta-val">{log.Inbound}</span>
                  </span>
                )}
                {log.Outbound && (
                  <span className="log-meta-pair">
                    <span className="log-meta-key">out</span>
                    <span className="log-meta-val">{log.Outbound}</span>
                  </span>
                )}
                {log.Email && (
                  <span className="log-meta-pair">
                    <span className="log-meta-key">email</span>
                    <span className="log-meta-val">{log.Email}</span>
                  </span>
                )}
              </div>
            </div>
          ))
        ) : (
          <table className="xraylog-table">
            <thead>
              <tr>
                <th>Date</th>
                <th>From</th>
                <th>To</th>
                <th>Inbound</th>
                <th>Outbound</th>
                <th>Email</th>
              </tr>
            </thead>
            <tbody>
              {orderedLogs.map((log, idx) => (
                <tr key={idx} className={`log-row-${log.Event}`}>
                  <td>
                    <b>{fullDate(log.DateTime)}</b>
                  </td>
                  <td>{log.FromAddress}</td>
                  <td>{log.ToAddress}</td>
                  <td>{log.Inbound}</td>
                  <td>{log.Outbound}</td>
                  <td>{log.Email}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </Modal>
  );
}
