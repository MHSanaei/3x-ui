import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Checkbox, Empty, Form, Input, Modal, Select, Tag } from 'antd';
import { DownloadOutlined, SyncOutlined } from '@ant-design/icons';

import { HttpUtil, FileManager, IntlUtil, PromiseUtil, SizeFormatter } from '@/utils';
import { activateOnKey } from '@/utils/a11y';
import { useDatepicker } from '@/hooks/useDatepicker';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import './XrayLogModal.css';
import './AmneziaWGLogModal.css';

interface AmneziaWGLogModalProps {
  open: boolean;
  onClose: () => void;
}

interface AmneziaWGPeerActivity {
  interface?: string;
  tag?: string;
  inboundId?: number;
  email?: string;
  endpoint?: string;
  allowedIPs?: string;
  handshake?: number;
  up?: number;
  down?: number;
  online?: boolean;
}

interface AmneziaWGLogs {
  peers?: AmneziaWGPeerActivity[];
  events?: string[];
  running?: boolean;
}

const AUTO_UPDATE_INTERVAL = 5000;

function shortTime(value?: number): string {
  if (!value) return '';
  const d = new Date(value);
  if (isNaN(d.getTime())) return '';
  const hh = String(d.getHours()).padStart(2, '0');
  const mm = String(d.getMinutes()).padStart(2, '0');
  const ss = String(d.getSeconds()).padStart(2, '0');
  return `${hh}:${mm}:${ss}`;
}

export default function AmneziaWGLogModal({ open, onClose }: AmneziaWGLogModalProps) {
  const { t } = useTranslation();
  const { datepicker } = useDatepicker();
  const { isMobile } = useMediaQuery();
  const [rows, setRows] = useState('50');
  const [filter, setFilter] = useState('');
  const [autoUpdate, setAutoUpdate] = useState(false);
  const [loading, setLoading] = useState(false);
  const [logs, setLogs] = useState<AmneziaWGLogs>({});

  const peers = useMemo(() => logs.peers ?? [], [logs.peers]);
  const events = useMemo(() => logs.events ?? [], [logs.events]);

  const runRefresh = useCallback(async () => {
    try {
      const msg = await HttpUtil.post<AmneziaWGLogs>(`/panel/api/server/amneziawglogs/${rows}`, {
        filter,
      });
      if (msg?.success) setLogs(msg.obj || {});
      await PromiseUtil.sleep(300);
    } finally {
      setLoading(false);
    }
  }, [rows, filter]);

  const refresh = useCallback(() => {
    setLoading(true);
    void runRefresh();
  }, [runRefresh]);

  const refreshRef = useRef(refresh);
  useEffect(() => {
    refreshRef.current = refresh;
  });

  // The spinner is raised during render so the fetch effect stays side-effect
  // free until its response lands.
  const refreshKey = open ? `${rows}|${filter}` : null;
  const [loadingKey, setLoadingKey] = useState<string | null>(null);
  if (refreshKey !== loadingKey) {
    setLoadingKey(refreshKey);
    if (refreshKey) setLoading(true);
  }

  useEffect(() => {
    if (open) void runRefresh();
  }, [open, rows, filter, runRefresh]);

  useEffect(() => {
    if (!open || !autoUpdate) return;
    const id = setInterval(() => refreshRef.current(), AUTO_UPDATE_INTERVAL);
    return () => clearInterval(id);
  }, [open, autoUpdate]);

  function fullDate(value?: number): string {
    return value ? IntlUtil.formatDate(value, datepicker) : '';
  }

  function download() {
    const peerLines = peers.map((p) => {
      const at = p.handshake ? new Date(p.handshake).toISOString() : 'never';
      return `${at} IFACE=${p.interface || ''} INBOUND=${p.tag || ''} EMAIL=${p.email || ''} ENDPOINT=${p.endpoint || '-'} ALLOWEDIPS=${p.allowedIPs || ''} UP=${p.up ?? 0} DOWN=${p.down ?? 0} ONLINE=${p.online ? 'yes' : 'no'}`;
    });
    FileManager.downloadTextFile([...peerLines, '', ...events].join('\n'), 'amneziawg.log');
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
          {t('pages.index.amneziawgLogs')}
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
          <Checkbox checked={autoUpdate} onChange={(e) => setAutoUpdate(e.target.checked)}>
            {t('pages.index.autoUpdate')}
          </Checkbox>
        </Form.Item>
        <Form.Item className="download-item">
          <Button
            type="primary"
            onClick={download}
            icon={<DownloadOutlined />}
            aria-label={t('download')}
          />
        </Form.Item>
      </Form>

      <div className={`log-container ${isMobile ? 'log-container-mobile' : ''}`}>
        {peers.length === 0 ? (
          <div className="log-empty">
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description={t('pages.index.amneziawgNoPeers')}
            />
          </div>
        ) : isMobile ? (
          peers.map((peer, idx) => (
            <div key={idx} className="log-card">
              <div className="log-card-head">
                <span className="log-time" title={fullDate(peer.handshake)}>
                  {shortTime(peer.handshake) || '—'}
                </span>
                <Tag color={peer.online ? 'green' : 'default'} className="log-event-tag">
                  {peer.online ? t('online') : t('pages.index.amneziawgIdle')}
                </Tag>
              </div>
              <div className="log-route">
                <span className="log-addr">{peer.endpoint || '—'}</span>
                <span className="log-arrow">→</span>
                <span className="log-addr">{peer.allowedIPs}</span>
              </div>
              <div className="log-meta">
                <span className="log-meta-pair">
                  <span className="log-meta-key">iface</span>
                  <span className="log-meta-val">{peer.interface}</span>
                </span>
                <span className="log-meta-pair">
                  <span className="log-meta-key">inbound</span>
                  <span className="log-meta-val">{peer.tag}</span>
                </span>
                {peer.email && (
                  <span className="log-meta-pair">
                    <span className="log-meta-key">email</span>
                    <span className="log-meta-val">{peer.email}</span>
                  </span>
                )}
                <span className="log-meta-pair">
                  <span className="log-meta-key">↑↓</span>
                  <span className="log-meta-val">
                    {`${SizeFormatter.sizeFormat(peer.up ?? 0)} / ${SizeFormatter.sizeFormat(peer.down ?? 0)}`}
                  </span>
                </span>
              </div>
            </div>
          ))
        ) : (
          <table className="xraylog-table">
            <thead>
              <tr>
                <th>{t('pages.index.amneziawgHandshake')}</th>
                <th>{t('pages.index.amneziawgInterface')}</th>
                <th>{t('pages.index.amneziawgInbound')}</th>
                <th>Email</th>
                <th>{t('pages.index.amneziawgEndpoint')}</th>
                <th>{t('pages.clients.amneziaWgAllowedIPs')}</th>
                <th>↑ / ↓</th>
              </tr>
            </thead>
            <tbody>
              {peers.map((peer, idx) => (
                <tr key={idx} className={peer.online ? undefined : 'log-row-offline'}>
                  <td>
                    <b>{fullDate(peer.handshake) || '—'}</b>
                  </td>
                  <td>{peer.interface}</td>
                  <td>{peer.tag}</td>
                  <td>{peer.email}</td>
                  <td>{peer.endpoint || '—'}</td>
                  <td>{peer.allowedIPs}</td>
                  <td>{`${SizeFormatter.sizeFormat(peer.up ?? 0)} / ${SizeFormatter.sizeFormat(peer.down ?? 0)}`}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div className="awglog-events-title">{t('pages.index.amneziawgEvents')}</div>
      <div className={`log-container ${isMobile ? 'log-container-mobile' : ''}`}>
        {events.length === 0 ? (
          <div className="log-empty">{t('pages.index.amneziawgNoEvents')}</div>
        ) : (
          events.map((line, idx) => (
            <div key={idx} className="awglog-event-line">
              {line}
            </div>
          ))
        )}
      </div>
    </Modal>
  );
}
