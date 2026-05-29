import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Input, Modal, Space, Table, Tag, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';

import type { ClientRecord } from '@/hooks/useClients';

interface GroupRemoveClientsModalProps {
  open: boolean;
  groupName: string | null;
  members: ClientRecord[];
  onClose: () => void;
  onSubmit: (emails: string[]) => Promise<{ affected?: number } | null>;
}

interface ClientRow {
  email: string;
  comment: string;
  enable: boolean;
}

export default function GroupRemoveClientsModal({
  open,
  groupName,
  members,
  onClose,
  onSubmit,
}: GroupRemoveClientsModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [saving, setSaving] = useState(false);
  const [selectedEmails, setSelectedEmails] = useState<string[]>([]);
  const [search, setSearch] = useState('');

  const rows = useMemo<ClientRow[]>(
    () =>
      (members || [])
        .map((c) => ({
          email: (c.email || '').trim(),
          comment: (c.comment || '').trim(),
          enable: c.enable !== false,
        }))
        .filter((r) => r.email),
    [members],
  );

  useEffect(() => {
    if (!open) return;
    setSelectedEmails([]);
    setSearch('');
  }, [open, rows]);

  const filteredRows = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return rows;
    return rows.filter(
      (r) => r.email.toLowerCase().includes(q) || r.comment.toLowerCase().includes(q),
    );
  }, [rows, search]);

  const columns: ColumnsType<ClientRow> = useMemo(
    () => [
      { title: t('pages.inbounds.email'), dataIndex: 'email', key: 'email', ellipsis: true },
      { title: t('comment'), dataIndex: 'comment', key: 'comment', ellipsis: true },
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
    if (!groupName || selectedEmails.length === 0) return;
    setSaving(true);
    try {
      const result = await onSubmit(selectedEmails);
      if (!result) return;
      const affected = result.affected ?? selectedEmails.length;
      messageApi.success(
        t('pages.groups.removeFromGroupResult', { count: affected, name: groupName }),
      );
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
      okButtonProps={{ danger: true, disabled: selectedEmails.length === 0, loading: saving }}
      okText={t('remove')}
      cancelText={t('cancel')}
      title={t('pages.groups.removeFromGroupTitle', { name: groupName ?? '' })}
      width={680}
    >
      {messageContextHolder}
      <Typography.Paragraph type="secondary">
        {t('pages.groups.removeFromGroupDesc')}
      </Typography.Paragraph>

      <Space direction="vertical" size="small" style={{ width: '100%' }}>
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
              total: rows.length,
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
