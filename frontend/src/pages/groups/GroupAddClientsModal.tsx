import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Input, Modal, Space, Table, Tag, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';

import type { ClientRecord } from '@/hooks/useClients';

interface GroupAddClientsModalProps {
  open: boolean;
  groupName: string | null;
  candidates: ClientRecord[];
  onClose: () => void;
  onSubmit: (emails: string[]) => Promise<{ affected?: number } | null>;
}

interface ClientRow {
  email: string;
  comment: string;
  enable: boolean;
  currentGroup: string;
}

export default function GroupAddClientsModal({
  open,
  groupName,
  candidates,
  onClose,
  onSubmit,
}: GroupAddClientsModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [saving, setSaving] = useState(false);
  const [selectedEmails, setSelectedEmails] = useState<string[]>([]);
  const [search, setSearch] = useState('');

  const rows = useMemo<ClientRow[]>(
    () =>
      (candidates || [])
        .map((c) => ({
          email: (c.email || '').trim(),
          comment: (c.comment || '').trim(),
          enable: c.enable !== false,
          currentGroup: (c.group || '').trim(),
        }))
        .filter((r) => r.email),
    [candidates],
  );

  useEffect(() => {
    if (!open) return;
    setSelectedEmails([]);
    setSearch('');
  }, [open]);

  const filteredRows = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return rows;
    return rows.filter(
      (r) =>
        r.email.toLowerCase().includes(q) ||
        r.comment.toLowerCase().includes(q) ||
        r.currentGroup.toLowerCase().includes(q),
    );
  }, [rows, search]);

  const columns: ColumnsType<ClientRow> = useMemo(
    () => [
      { title: t('pages.inbounds.email'), dataIndex: 'email', key: 'email', ellipsis: true },
      { title: t('comment'), dataIndex: 'comment', key: 'comment', ellipsis: true },
      {
        title: t('pages.clients.group'),
        dataIndex: 'currentGroup',
        key: 'currentGroup',
        width: 140,
        ellipsis: true,
        render: (g: string) =>
          g ? <Tag color="geekblue">{g}</Tag> : <span style={{ color: 'rgba(0,0,0,0.45)' }}>—</span>,
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
    if (!groupName || selectedEmails.length === 0) return;
    setSaving(true);
    try {
      const result = await onSubmit(selectedEmails);
      if (!result) return;
      const affected = result.affected ?? selectedEmails.length;
      messageApi.success(t('pages.groups.addToGroupResult', { count: affected, name: groupName }));
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
      okButtonProps={{ disabled: selectedEmails.length === 0, loading: saving }}
      okText={t('add')}
      cancelText={t('cancel')}
      title={t('pages.groups.addToGroupTitle', { name: groupName ?? '' })}
      width={720}
    >
      {messageContextHolder}
      <Typography.Paragraph type="secondary">
        {t('pages.groups.addToGroupDesc')}
      </Typography.Paragraph>

      <Space orientation="vertical" size="small" style={{ width: '100%' }}>
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
        {rows.length === 0 ? (
          <Alert type="info" showIcon title={t('pages.groups.addToGroupEmpty')} />
        ) : (
          <Table<ClientRow>
            size="small"
            rowKey="email"
            columns={columns}
            dataSource={filteredRows}
            pagination={false}
            scroll={{ y: 320 }}
            rowSelection={{
              selectedRowKeys: selectedEmails,
              onChange: (keys) => setSelectedEmails(keys as string[]),
              preserveSelectedRowKeys: true,
            }}
          />
        )}
      </Space>
    </Modal>
  );
}
