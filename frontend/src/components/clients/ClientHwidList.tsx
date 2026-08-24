import { Button, Modal, Popconfirm, Tag, Typography } from 'antd';
import { DeleteOutlined, ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { ClientHwidInfo } from '@/lib/clients/hwid-log';

interface ClientHwidListModalProps {
  open: boolean;
  email?: string;
  zIndex?: number;
  hwids: ClientHwidInfo[];
  loading: boolean;
  clearing: boolean;
  deletingId: number | null;
  formatDate: (ts: number) => string;
  onRefresh: () => void;
  onClearAll: () => void;
  onDelete: (id: number) => void;
  onClose: () => void;
}

// The single place the HWID device list is rendered — the edit form and the
// info card share it so date format and row layout can't drift apart again.
export default function ClientHwidListModal({
  open,
  email,
  zIndex,
  hwids,
  loading,
  clearing,
  deletingId,
  formatDate,
  onRefresh,
  onClearAll,
  onDelete,
  onClose,
}: ClientHwidListModalProps) {
  const { t } = useTranslation();

  return (
    <Modal
      open={open}
      title={`${t('pages.clients.hwidLog')}${email ? ` — ${email}` : ''}`}
      width={520}
      zIndex={zIndex}
      onCancel={onClose}
      footer={[
        <Button key="refresh" icon={<ReloadOutlined />} loading={loading} onClick={onRefresh}>
          {t('refresh')}
        </Button>,
        <Popconfirm
          key="clear"
          title={t('pages.clients.clearHwidsConfirm')}
          onConfirm={onClearAll}
          okType="danger"
          okText={t('delete')}
          cancelText={t('cancel')}
        >
          <Button danger loading={clearing} disabled={hwids.length === 0}>
            {t('pages.clients.clearAll')}
          </Button>
        </Popconfirm>,
        <Button key="close" type="primary" onClick={onClose}>
          {t('close')}
        </Button>,
      ]}
    >
      {hwids.length > 0 ? (
        <div style={{ maxHeight: 360, overflowY: 'auto' }}>
          {hwids.map((entry) => (
            <div
              key={entry.id}
              style={{
                display: 'flex',
                alignItems: 'flex-start',
                gap: 8,
                borderBottom: '1px solid var(--ant-color-border-secondary)',
                padding: '8px 0',
              }}
            >
              <div style={{ flex: 1, minWidth: 0 }}>
                <Typography.Text strong>
                  {entry.deviceModel || entry.userAgent || t('pages.clients.hwidDevice')}
                </Typography.Text>
                <br />
                <Typography.Text type="secondary">
                  {[entry.deviceOs, entry.osVersion].filter(Boolean).join(' ')}
                </Typography.Text>
                <br />
                <Typography.Text type="secondary">
                  {t('pages.clients.firstSeen')}: {formatDate(entry.firstSeen)}
                </Typography.Text>
                <br />
                <Typography.Text type="secondary">
                  {t('pages.clients.lastSeen')}: {formatDate(entry.lastSeen)}
                </Typography.Text>
                {entry.userAgent && (
                  <>
                    <br />
                    <Typography.Text type="secondary" style={{ wordBreak: 'break-all' }}>
                      {entry.userAgent}
                    </Typography.Text>
                  </>
                )}
              </div>
              <Popconfirm
                title={t('pages.clients.deleteHwidConfirm')}
                onConfirm={() => onDelete(entry.id)}
                okType="danger"
                okText={t('delete')}
                cancelText={t('cancel')}
              >
                <Button
                  danger
                  type="text"
                  size="small"
                  aria-label={t('pages.clients.deleteHwid')}
                  icon={<DeleteOutlined />}
                  loading={deletingId === entry.id}
                />
              </Popconfirm>
            </div>
          ))}
        </div>
      ) : (
        <Tag>{t('pages.clients.noHwids')}</Tag>
      )}
    </Modal>
  );
}
