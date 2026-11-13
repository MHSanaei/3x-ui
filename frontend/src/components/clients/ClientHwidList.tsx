import { Button, Modal, Popconfirm, Tag, Typography } from 'antd';
import { DeleteOutlined, ReloadOutlined, StopOutlined, UnlockOutlined } from '@ant-design/icons';
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
  blockingId: number | null;
  formatDate: (ts: number) => string;
  onRefresh: () => void;
  onClearAll: () => void;
  onDelete: (id: number) => void;
  onSetBlocked: (id: number, blocked: boolean) => void;
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
  blockingId,
  formatDate,
  onRefresh,
  onClearAll,
  onDelete,
  onSetBlocked,
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
        <Button
          key="refresh"
          icon={<ReloadOutlined aria-hidden />}
          loading={loading}
          onClick={onRefresh}
        >
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
          <Button
            danger
            loading={clearing}
            disabled={
              loading ||
              blockingId !== null ||
              deletingId !== null ||
              !hwids.some((entry) => !entry.blocked)
            }
          >
            {t('pages.clients.clearAll')}
          </Button>
        </Popconfirm>,
        <Button key="close" type="primary" onClick={onClose}>
          {t('close')}
        </Button>,
      ]}
    >
      <Typography.Paragraph type="secondary">
        {t('pages.clients.hwidBlockHint')}
      </Typography.Paragraph>
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
                {entry.blocked && <Tag color="red">{t('pages.clients.hwidBlocked')}</Tag>}
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
                {entry.fingerprint && (
                  <>
                    <br />
                    <Typography.Text type="secondary" code>
                      {t('pages.clients.hwidFingerprint')}: {entry.fingerprint}
                    </Typography.Text>
                  </>
                )}
              </div>
              <Popconfirm
                title={t(
                  entry.blocked
                    ? 'pages.clients.unblockHwidConfirm'
                    : 'pages.clients.blockHwidConfirm',
                )}
                onConfirm={() => onSetBlocked(entry.id, !entry.blocked)}
                okType={entry.blocked ? 'primary' : 'danger'}
                okText={t(entry.blocked ? 'pages.clients.unblockHwid' : 'pages.clients.blockHwid')}
                cancelText={t('cancel')}
              >
                <Button
                  danger={!entry.blocked}
                  size="small"
                  icon={
                    entry.blocked ? <UnlockOutlined aria-hidden /> : <StopOutlined aria-hidden />
                  }
                  loading={blockingId === entry.id}
                  disabled={clearing || loading || deletingId !== null || blockingId !== null}
                >
                  {t(entry.blocked ? 'pages.clients.unblockHwid' : 'pages.clients.blockHwid')}
                </Button>
              </Popconfirm>
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
                  icon={<DeleteOutlined aria-hidden />}
                  loading={deletingId === entry.id}
                  disabled={
                    entry.blocked ||
                    clearing ||
                    loading ||
                    blockingId !== null ||
                    deletingId !== null
                  }
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
