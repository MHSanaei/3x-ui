import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Input, Modal, Space, Table, Tag, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';

import { HttpUtil } from '@/utils';
import { coerceInboundJsonField, type DBInbound } from '@/models/dbinbound';

interface DetachClientsModalProps {
  open: boolean;
  source: DBInbound | null;
  onClose: () => void;
  onDetached?: () => void;
}

interface BulkDetachResult {
  detached?: string[];
  skipped?: string[];
  errors?: string[];
}

interface ClientRow {
  email: string;
  comment: string;
  enable: boolean;
}

function readClientRows(settings: unknown): ClientRow[] {
  const parsed = coerceInboundJsonField(settings) as {
    clients?: Array<{ email?: string; comment?: string; enable?: boolean }>;
  };
  const clients = Array.isArray(parsed?.clients) ? parsed.clients : [];
  return clients
    .map((c) => ({
      email: (c?.email || '').trim(),
      comment: (c?.comment || '').trim(),
      enable: c?.enable !== false,
    }))
    .filter((r) => r.email);
}

export default function DetachClientsModal({
  open,
  source,
  onClose,
  onDetached,
}: DetachClientsModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [saving, setSaving] = useState(false);
  const [clientRows, setClientRows] = useState<ClientRow[]>([]);
  const [selectedEmails, setSelectedEmails] = useState<string[]>([]);
  const [search, setSearch] = useState('');

  useEffect(() => {
    if (!open) return;
    const rows = source ? readClientRows(source.settings) : [];
    setClientRows(rows);
    setSelectedEmails([]);
    setSearch('');
  }, [open, source]);

  const filteredRows = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return clientRows;
    return clientRows.filter(
      (r) => r.email.toLowerCase().includes(q) || r.comment.toLowerCase().includes(q),
    );
  }, [clientRows, search]);

  const columns: ColumnsType<ClientRow> = useMemo(
    () => [
      {
        title: t('pages.inbounds.email'),
        dataIndex: 'email',
        key: 'email',
        ellipsis: true,
      },
      {
        title: t('comment'),
        dataIndex: 'comment',
        key: 'comment',
        ellipsis: true,
      },
      {
        title: t('enable'),
        dataIndex: 'enable',
        key: 'enable',
        width: 90,
        render: (enabled: boolean) =>
          enabled ? (
            <Tag color="success">{t('enable')}</Tag>
          ) : (
            <Tag>{t('pages.inbounds.attachClientsStatusDisabled')}</Tag>
          ),
      },
    ],
    [t],
  );

  async function submit() {
    if (!source || selectedEmails.length === 0) return;
    setSaving(true);
    try {
      const msg = await HttpUtil.post(
        '/panel/api/clients/bulkDetach',
        { emails: selectedEmails, inboundIds: [source.id] },
        { headers: { 'Content-Type': 'application/json' } },
      );
      if (!msg?.success) {
        messageApi.error(msg?.msg || t('somethingWentWrong'));
        return;
      }
      const result = (msg.obj || {}) as BulkDetachResult;
      const detached = result.detached?.length ?? 0;
      const skipped = result.skipped?.length ?? 0;
      const errors = result.errors?.length ?? 0;
      if (errors > 0) {
        messageApi.warning(t('pages.inbounds.detachClientsResultMixed', { detached, skipped, errors }));
      } else {
        messageApi.success(t('pages.inbounds.detachClientsResult', { detached, skipped }));
      }
      onDetached?.();
      onClose();
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal
      open={open}
      onCancel={onClose}
      onOk={submit}
      okButtonProps={{
        danger: true,
        disabled: selectedEmails.length === 0,
        loading: saving,
      }}
      okText={t('pages.inbounds.detachClients')}
      cancelText={t('cancel')}
      title={t('pages.inbounds.detachClientsTitle', { remark: source?.tag ?? '' })}
      width={680}
    >
      {messageContextHolder}
      <Typography.Paragraph type="secondary">
        {t('pages.inbounds.detachClientsDesc', { count: clientRows.length })}
      </Typography.Paragraph>

      <Space orientation="vertical" size="small" style={{ width: '100%' }}>
        <Typography.Text strong>{t('pages.inbounds.detachClientsSelectLabel')}</Typography.Text>
        <Space style={{ width: '100%', justifyContent: 'space-between' }} wrap>
          <Input.Search
            allowClear
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t('pages.inbounds.attachClientsSearchPlaceholder')}
            style={{ maxWidth: 320 }}
          />
          <Typography.Text type="secondary">
            {t('pages.inbounds.attachClientsSelectedCount', {
              selected: selectedEmails.length,
              total: clientRows.length,
            })}
          </Typography.Text>
        </Space>
        <Table<ClientRow>
          size="small"
          rowKey="email"
          columns={columns}
          dataSource={filteredRows}
          pagination={false}
          scroll={{ y: 280 }}
          rowSelection={{
            selectedRowKeys: selectedEmails,
            onChange: (keys) => setSelectedEmails(keys as string[]),
            preserveSelectedRowKeys: true,
          }}
        />
      </Space>
    </Modal>
  );
}
