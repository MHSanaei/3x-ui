import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Badge, Modal, Table, Tag } from 'antd';
import dayjs from 'dayjs';

import { HttpUtil, SizeFormatter } from '@/utils';

interface AwgDiagnosticsClient {
  email: string;
  connected: boolean;
  lastHandshake?: string;
  rxBytes: number;
  txBytes: number;
}

interface AwgDiagnostics {
  running: boolean;
  listenPort?: number;
  clients: AwgDiagnosticsClient[];
}

interface AwgDiagnosticsModalProps {
  open: boolean;
  onClose: () => void;
  inboundId: number;
}

// Read-only snapshot of one embedded AmneziaWG inbound's live state --
// interface up/down, listen port, and each configured client's real
// handshake/traffic status straight from the running amneziawg-go Device's
// own UAPI dump. Fetched fresh every time the modal opens (no polling --
// this is a point-in-time check, not a live dashboard).
export default function AwgDiagnosticsModal({ open, onClose, inboundId }: AwgDiagnosticsModalProps) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [diag, setDiag] = useState<AwgDiagnostics | null>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    if (!open) return;
    setLoading(true);
    setError(false);
    setDiag(null);
    HttpUtil.get<AwgDiagnostics>(`/panel/api/inbounds/${inboundId}/awgDiagnostics`)
      .then((msg) => {
        if (msg?.success && msg.obj) {
          setDiag(msg.obj);
        } else {
          setError(true);
        }
      })
      .catch(() => setError(true))
      .finally(() => setLoading(false));
  }, [open, inboundId]);

  return (
    <Modal
      title={t('pages.xray.amneziawg.diagnostics')}
      open={open}
      onCancel={onClose}
      onOk={onClose}
      cancelButtonProps={{ style: { display: 'none' } }}
      okText={t('close')}
      width={640}
      destroyOnHidden
    >
      {error && (
        <Alert type="error" showIcon message={t('pages.xray.amneziawg.diagnosticsError')} style={{ marginBottom: 16 }} />
      )}
      {!error && diag && !diag.running && (
        <Alert
          type="warning"
          showIcon
          message={t('pages.xray.amneziawg.diagnosticsNotRunning')}
          style={{ marginBottom: 16 }}
        />
      )}
      {!error && diag && diag.running && (
        <p style={{ marginBottom: 16 }}>
          {t('pages.xray.amneziawg.diagnosticsListenPort')}: <b>{diag.listenPort}</b>
        </p>
      )}
      <Table<AwgDiagnosticsClient>
        loading={loading}
        dataSource={diag?.clients ?? []}
        rowKey="email"
        size="small"
        pagination={false}
        locale={{ emptyText: t('pages.xray.amneziawg.diagnosticsNoClients') }}
        columns={[
          {
            title: t('pages.inbounds.email'),
            dataIndex: 'email',
          },
          {
            title: t('pages.xray.amneziawg.diagnosticsStatus'),
            dataIndex: 'connected',
            render: (connected: boolean) => (
              <Tag color={connected ? 'success' : 'default'}>
                <Badge status={connected ? 'success' : 'default'} />
                {' '}
                {connected
                  ? t('pages.xray.amneziawg.diagnosticsConnected')
                  : t('pages.xray.amneziawg.diagnosticsNeverConnected')}
              </Tag>
            ),
          },
          {
            title: t('pages.xray.amneziawg.diagnosticsLastHandshake'),
            dataIndex: 'lastHandshake',
            render: (v?: string) => (v ? dayjs(v).format('YYYY-MM-DD HH:mm:ss') : '—'),
          },
          {
            title: t('pages.xray.amneziawg.diagnosticsTraffic'),
            key: 'traffic',
            render: (_: unknown, row: AwgDiagnosticsClient) => (
              <>
                ↓{SizeFormatter.sizeFormat(row.rxBytes)} ↑{SizeFormatter.sizeFormat(row.txBytes)}
              </>
            ),
          },
        ]}
      />
    </Modal>
  );
}
