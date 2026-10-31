import { useTranslation } from 'react-i18next';
import { Modal, Spin, Table, Tag, Typography } from 'antd';
import dayjs from 'dayjs';

import { useClientUsageHistoryQuery } from '@/api/queries/useClientUsageHistoryQuery';
import { SizeFormatter } from '@/utils';

interface ClientUsageHistoryModalProps {
  open: boolean;
  email?: string;
  zIndex?: number;
  onClose: () => void;
}

// Reason label for each closed traffic window; unknown reasons stay raw so a
// future reason still renders instead of blanking out.
const REASON_KEYS: Record<string, string> = {
  reset: 'pages.clients.reasonReset',
  renew: 'pages.clients.reasonRenew',
  deleted: 'pages.clients.reasonDeleted',
  renamed: 'pages.clients.reasonRenamed',
};

// Human labels for the change-log field names; unknown fields fall back to the
// raw key so a future field still renders instead of blanking out.
const FIELD_KEYS: Record<string, string> = {
  name: 'pages.clients.fieldName',
  subId: 'pages.clients.fieldSubId',
  expiry: 'pages.clients.fieldExpiry',
  dataLimit: 'pages.clients.fieldDataLimit',
  enable: 'pages.clients.fieldEnable',
  renewalPlan: 'pages.clients.fieldRenewalPlan',
  limitIp: 'pages.clients.fieldLimitIp',
  limitHwid: 'pages.clients.fieldLimitHwid',
  adTag: 'pages.clients.fieldAdTag',
  group: 'pages.clients.fieldGroup',
  deleted: 'pages.clients.fieldDeleted',
};

function fieldLabel(t: (key: string) => string, field: string): string {
  const key = FIELD_KEYS[field];
  return key ? t(key) : field;
}

function changeValueText(field: string, value: string): string {
  if (value === '') return '—';
  if (field === 'expiry') {
    const n = Number(value);
    if (Number.isFinite(n) && n > 0) return dayjs(n).format('YYYY-MM-DD HH:mm');
  } else if (field === 'dataLimit') {
    const n = Number(value);
    if (Number.isFinite(n)) return SizeFormatter.sizeFormat(n);
  }
  return value;
}

export default function ClientUsageHistoryModal({
  open,
  email,
  zIndex,
  onClose,
}: ClientUsageHistoryModalProps) {
  const { t } = useTranslation();
  const { data: history, isFetching, isError } = useClientUsageHistoryQuery(email, open);

  const bytesCell = (value: number) => SizeFormatter.sizeFormat(value);

  const nodeColumns = [
    {
      title: t('pages.clients.node'),
      dataIndex: 'nodeName',
      key: 'nodeName',
      render: (value: string, row: { nodeId: number }) => value || `#${row.nodeId}`,
    },
    {
      title: t('pages.clients.upload'),
      dataIndex: 'up',
      key: 'up',
      width: 140,
      align: 'right' as const,
      render: bytesCell,
    },
    {
      title: t('pages.clients.download'),
      dataIndex: 'down',
      key: 'down',
      width: 140,
      align: 'right' as const,
      render: bytesCell,
    },
  ];

  const windowColumns = [
    {
      title: t('pages.clients.time'),
      dataIndex: 'at',
      key: 'at',
      width: 160,
      render: (value: number) => dayjs(value).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: t('pages.clients.upload'),
      dataIndex: 'up',
      key: 'up',
      width: 110,
      align: 'right' as const,
      render: bytesCell,
    },
    {
      title: t('pages.clients.download'),
      dataIndex: 'down',
      key: 'down',
      width: 110,
      align: 'right' as const,
      render: bytesCell,
    },
    {
      title: t('pages.clients.reason'),
      dataIndex: 'reason',
      key: 'reason',
      width: 90,
      render: (value: string) => <Tag>{REASON_KEYS[value] ? t(REASON_KEYS[value]) : value}</Tag>,
    },
  ];

  const changeColumns = [
    {
      title: t('pages.clients.time'),
      dataIndex: 'at',
      key: 'at',
      width: 160,
      render: (value: number) => dayjs(value).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: t('pages.clients.field'),
      dataIndex: 'field',
      key: 'field',
      width: 100,
      render: (value: string) => fieldLabel(t, value),
    },
    {
      title: t('pages.clients.oldValue'),
      dataIndex: 'oldValue',
      key: 'oldValue',
      render: (value: string, row: { field: string }) => changeValueText(row.field, value),
    },
    {
      title: t('pages.clients.newValue'),
      dataIndex: 'newValue',
      key: 'newValue',
      render: (value: string, row: { field: string }) => changeValueText(row.field, value),
    },
    {
      title: t('pages.clients.source'),
      dataIndex: 'source',
      key: 'source',
      width: 90,
      render: (value: string) => <Tag>{value}</Tag>,
    },
  ];

  const hasContent =
    history !== undefined &&
    (history.lifetimeUp > 0 ||
      history.lifetimeDown > 0 ||
      history.windows.length > 0 ||
      history.nodes.length > 0 ||
      history.changes.length > 0);

  return (
    <Modal
      open={open}
      title={`${t('pages.clients.usageHistory')}${email ? ` — ${email}` : ''}`}
      width={720}
      zIndex={zIndex}
      onCancel={onClose}
      footer={null}
      destroyOnHidden
    >
      {isFetching ? (
        <div style={{ padding: 48, textAlign: 'center' }}>
          <Spin />
        </div>
      ) : isError ? (
        <Typography.Text type="secondary">{t('pages.clients.historyError')}</Typography.Text>
      ) : !hasContent ? (
        <Typography.Text type="secondary">{t('pages.clients.noUsageHistory')}</Typography.Text>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
          <section>
            <Typography.Title level={5} style={{ marginBottom: 8 }}>
              {t('pages.clients.lifetimeTraffic')}
            </Typography.Title>
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <tbody>
                <tr>
                  <td style={{ padding: '2px 12px 2px 0' }}>{t('pages.clients.upload')}</td>
                  <td style={{ textAlign: 'right' }}>
                    {SizeFormatter.sizeFormat(history!.lifetimeUp)}
                  </td>
                  <td style={{ padding: '2px 12px' }}>{t('pages.clients.download')}</td>
                  <td style={{ textAlign: 'right' }}>
                    {SizeFormatter.sizeFormat(history!.lifetimeDown)}
                  </td>
                </tr>
                <tr>
                  <td colSpan={4} style={{ color: 'var(--ant-color-text-secondary)' }}>
                    {t('pages.clients.currentCycle')}: ↑{' '}
                    {SizeFormatter.sizeFormat(history!.currentUp)} / ↓{' '}
                    {SizeFormatter.sizeFormat(history!.currentDown)}
                  </td>
                </tr>
              </tbody>
            </table>
          </section>
          {history!.nodes.length > 0 && (
            <section>
              <Typography.Title level={5} style={{ marginBottom: 8 }}>
                {t('pages.clients.usageByNode')}
              </Typography.Title>
              <Table
                rowKey="nodeId"
                size="small"
                columns={nodeColumns}
                dataSource={history!.nodes}
                pagination={false}
              />
            </section>
          )}
          {history!.windows.length > 0 && (
            <section>
              <Typography.Title level={5} style={{ marginBottom: 8 }}>
                {t('pages.clients.pastCycles')}
              </Typography.Title>
              <div style={{ maxHeight: 200, overflowY: 'auto' }}>
                <Table
                  rowKey="id"
                  size="small"
                  columns={windowColumns}
                  dataSource={history!.windows}
                  pagination={false}
                />
              </div>
            </section>
          )}
          {history!.changes.length > 0 && (
            <section>
              <Typography.Title level={5} style={{ marginBottom: 8 }}>
                {t('pages.clients.changes')}
              </Typography.Title>
              <div style={{ maxHeight: 240, overflowY: 'auto' }}>
                <Table
                  rowKey="id"
                  size="small"
                  columns={changeColumns}
                  dataSource={history!.changes}
                  pagination={false}
                />
              </div>
            </section>
          )}
        </div>
      )}
    </Modal>
  );
}
