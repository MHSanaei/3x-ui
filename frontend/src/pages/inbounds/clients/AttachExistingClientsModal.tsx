import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Input, Modal, Select, Space, Spin, Table, Tag, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';

import { HttpUtil } from '@/utils';
import type { DBInbound } from '@/models/dbinbound';

interface AttachExistingClientsModalProps {
  open: boolean;
  target: DBInbound | null;
  onClose: () => void;
  onAttached?: () => void;
}

interface BulkAttachResult {
  attached?: string[];
  skipped?: string[];
  errors?: string[];
}

interface ClientRow {
  email: string;
  group: string;
  enable: boolean;
  alreadyAttached: boolean;
}

interface RawClient {
  email?: string;
  group?: string;
  enable?: boolean;
  inboundIds?: number[] | null;
}

export default function AttachExistingClientsModal({
  open,
  target,
  onClose,
  onAttached,
}: AttachExistingClientsModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [clientRows, setClientRows] = useState<ClientRow[]>([]);
  const [selectedEmails, setSelectedEmails] = useState<string[]>([]);
  const [search, setSearch] = useState('');
  const [groupFilter, setGroupFilter] = useState<string | undefined>(undefined);

  useEffect(() => {
    if (!open || !target) return;
    let cancelled = false;
    setLoading(true);
    setSearch('');
    setGroupFilter(undefined);
    HttpUtil.get('/panel/api/clients/list', undefined, { silent: true })
      .then((msg) => {
        if (cancelled) return;
        const list = Array.isArray(msg?.obj) ? (msg.obj as RawClient[]) : [];
        const rows: ClientRow[] = list
          .map((c) => ({
            email: (c?.email || '').trim(),
            group: (c?.group || '').trim(),
            enable: c?.enable !== false,
            alreadyAttached: Array.isArray(c?.inboundIds) && c.inboundIds.includes(target.id),
          }))
          .filter((r) => r.email);
        setClientRows(rows);
        setSelectedEmails(rows.filter((r) => !r.alreadyAttached).map((r) => r.email));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, target]);

  const groupOptions = useMemo(() => {
    const set = new Set<string>();
    for (const r of clientRows) if (r.group) set.add(r.group);
    return [...set].sort((a, b) => a.localeCompare(b)).map((g) => ({ value: g, label: g }));
  }, [clientRows]);

  const attachableCount = useMemo(
    () => clientRows.filter((r) => !r.alreadyAttached).length,
    [clientRows],
  );

  const filteredRows = useMemo(() => {
    const q = search.trim().toLowerCase();
    return clientRows.filter((r) => {
      if (groupFilter && r.group !== groupFilter) return false;
      if (!q) return true;
      return r.email.toLowerCase().includes(q) || r.group.toLowerCase().includes(q);
    });
  }, [clientRows, search, groupFilter]);

  const columns: ColumnsType<ClientRow> = useMemo(
    () => [
      {
        title: t('pages.inbounds.email'),
        dataIndex: 'email',
        key: 'email',
        ellipsis: true,
      },
      {
        title: t('pages.clients.group'),
        dataIndex: 'group',
        key: 'group',
        width: 150,
        ellipsis: true,
        render: (group: string) =>
          group ? <Tag color="geekblue">{group}</Tag> : <span style={{ color: 'rgba(0,0,0,0.45)' }}>—</span>,
      },
      {
        title: t('enable'),
        key: 'status',
        width: 140,
        render: (_v, row) => {
          if (row.alreadyAttached) return <Tag color="default">{t('pages.inbounds.attachExistingStatusAttached')}</Tag>;
          return row.enable ? (
            <Tag color="success">{t('enable')}</Tag>
          ) : (
            <Tag>{t('pages.inbounds.attachClientsStatusDisabled')}</Tag>
          );
        },
      },
    ],
    [t],
  );

  async function submit() {
    if (!target || selectedEmails.length === 0) return;
    setSaving(true);
    try {
      const msg = await HttpUtil.post(
        '/panel/api/clients/bulkAttach',
        { emails: selectedEmails, inboundIds: [target.id] },
        { headers: { 'Content-Type': 'application/json' } },
      );
      if (!msg?.success) {
        messageApi.error(msg?.msg || t('somethingWentWrong'));
        return;
      }
      const result = (msg.obj || {}) as BulkAttachResult;
      const attached = result.attached?.length ?? 0;
      const skipped = result.skipped?.length ?? 0;
      const errors = result.errors?.length ?? 0;
      if (errors > 0) {
        messageApi.warning(t('pages.inbounds.attachClientsResultMixed', { attached, skipped, errors }));
      } else {
        messageApi.success(t('pages.inbounds.attachClientsResult', { attached, skipped }));
      }
      onAttached?.();
      onClose();
    } finally {
      setSaving(false);
    }
  }

  const noClients = !loading && clientRows.length === 0;

  return (
    <Modal
      open={open}
      onCancel={onClose}
      onOk={submit}
      okButtonProps={{ disabled: selectedEmails.length === 0, loading: saving }}
      okText={t('pages.inbounds.attachClients')}
      cancelText={t('cancel')}
      title={t('pages.inbounds.attachExistingTitle', { remark: target?.remark?.trim() || target?.tag || '' })}
      width={680}
    >
      {messageContextHolder}
      <Typography.Paragraph type="secondary">
        {t('pages.inbounds.attachExistingDesc', { count: attachableCount })}
      </Typography.Paragraph>

      {noClients ? (
        <Alert type="info" showIcon message={t('pages.inbounds.attachExistingNoClients')} />
      ) : (
        <Spin spinning={loading}>
          <Space direction="vertical" size="small" style={{ width: '100%' }}>
            <Space style={{ width: '100%', justifyContent: 'space-between' }} wrap>
              <Space wrap>
                <Input.Search
                  allowClear
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder={t('pages.inbounds.attachClientsSearchPlaceholder')}
                  style={{ width: 260 }}
                />
                {groupOptions.length > 0 && (
                  <Select
                    allowClear
                    value={groupFilter}
                    onChange={(v) => setGroupFilter(v)}
                    options={groupOptions}
                    placeholder={t('pages.clients.group')}
                    style={{ minWidth: 160 }}
                    optionFilterProp="label"
                  />
                )}
              </Space>
              <Typography.Text type="secondary">
                {t('pages.inbounds.attachClientsSelectedCount', {
                  selected: selectedEmails.length,
                  total: attachableCount,
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
                getCheckboxProps: (row) => ({ disabled: row.alreadyAttached }),
                preserveSelectedRowKeys: true,
              }}
            />
          </Space>
        </Spin>
      )}
    </Modal>
  );
}
