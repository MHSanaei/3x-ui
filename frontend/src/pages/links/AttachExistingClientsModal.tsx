import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Input, Modal, Select, Space, Spin, Table, Tag, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';

import { HttpUtil, type Msg } from '@/utils';
import type { LinkAssignResult } from '@/schemas/api/link';
import type { ManagedLinkRecord } from '@/api/queries/useLinksQuery';

interface AttachExistingClientsModalProps {
  open: boolean;
  targets: ManagedLinkRecord[];
  loading?: boolean;
  assign: (linkIds: number[], emails: string[]) => Promise<Msg<LinkAssignResult> | undefined>;
  onClose: () => void;
  onAttached?: () => void;
}

interface ClientRow {
  email: string;
  group: string;
  enable: boolean;
  alreadyAttached: boolean;
}

interface RawExternalLink {
  kind?: string;
  value?: string;
}

interface RawClient {
  email?: string;
  group?: string;
  enable?: boolean;
  externalLinks?: RawExternalLink[] | null;
}

function shortValue(value: string): string {
  if (!value) return '';
  if (value.length <= 72) return value;
  return `${value.slice(0, 48)}...${value.slice(-14)}`;
}

function linkKey(link: Pick<ManagedLinkRecord, 'kind' | 'value'>): string {
  return `${link.kind}\x00${link.value}`;
}

function targetLabel(targets: ManagedLinkRecord[]): string {
  if (targets.length === 0) return '';
  if (targets.length === 1) return targets[0].remark || shortValue(targets[0].value);
  return `${targets.length} link(s)`;
}

export default function AttachExistingClientsModal({
  open,
  targets,
  loading: saving = false,
  assign,
  onClose,
  onAttached,
}: AttachExistingClientsModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);
  const [clientRows, setClientRows] = useState<ClientRow[]>([]);
  const [selectedEmails, setSelectedEmails] = useState<string[]>([]);
  const [search, setSearch] = useState('');
  const [groupFilter, setGroupFilter] = useState<string | undefined>(undefined);

  useEffect(() => {
    if (!open || targets.length === 0) return;
    let cancelled = false;
    const targetKeys = new Set(targets.map(linkKey));
    setLoading(true);
    setSearch('');
    setGroupFilter(undefined);
    HttpUtil.get('/panel/api/clients/list', undefined, { silent: true })
      .then((msg) => {
        if (cancelled) return;
        const list = Array.isArray(msg?.obj) ? (msg.obj as RawClient[]) : [];
        const rows: ClientRow[] = list
          .map((c) => {
            const attachedKeys = new Set(
              (Array.isArray(c?.externalLinks) ? c.externalLinks : [])
                .filter((row) => row?.kind && row?.value)
                .map((row) => `${row.kind}\x00${row.value}`),
            );
            const alreadyAttached = [...targetKeys].every((key) => attachedKeys.has(key));
            return {
              email: (c?.email || '').trim(),
              group: (c?.group || '').trim(),
              enable: c?.enable !== false,
              alreadyAttached,
            };
          })
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
  }, [open, targets]);

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
          group ? <Tag color="geekblue">{group}</Tag> : <span style={{ color: 'rgba(0,0,0,0.45)' }}>-</span>,
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
    if (targets.length === 0 || selectedEmails.length === 0) return;
    const msg = await assign(targets.map((link) => link.id), selectedEmails);
    if (!msg?.success) {
      messageApi.error(msg?.msg || t('somethingWentWrong'));
      return;
    }
    const result = msg.obj;
    messageApi.success(t('pages.links.toasts.assignResult', {
      attached: result?.attached ?? 0,
      skipped: result?.skipped ?? 0,
    }));
    onAttached?.();
    onClose();
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
      title={t('pages.links.attachExistingTitle', { remark: targetLabel(targets) })}
      width={680}
      destroyOnHidden
    >
      {messageContextHolder}
      <Typography.Paragraph type="secondary">
        {t('pages.links.attachExistingDesc', { count: attachableCount })}
      </Typography.Paragraph>

      {noClients ? (
        <Alert type="info" showIcon message={t('pages.inbounds.attachExistingNoClients')} />
      ) : (
        <Spin spinning={loading}>
          <Space orientation="vertical" size="small" style={{ width: '100%' }}>
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
