import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, Space, Switch, Table, Tag, Tooltip } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  DeleteOutlined,
  EditOutlined,
  GlobalOutlined,
  PlusOutlined,
} from '@ant-design/icons';

import type { HostRecord } from '@/api/queries/useHostsQuery';
import type { InboundOption } from '@/schemas/client';
import './HostList.css';

interface HostListProps {
  hosts: HostRecord[];
  inboundOptions: InboundOption[];
  loading?: boolean;
  isMobile?: boolean;
  selectedIds: number[];
  onSelectionChange: (ids: number[]) => void;
  onAdd: () => void;
  onEdit: (host: HostRecord) => void;
  onDelete: (host: HostRecord) => void;
  onToggleEnable: (host: HostRecord, next: boolean) => void;
  onMove: (host: HostRecord, dir: 'up' | 'down') => void;
  onBulkEnable: (enable: boolean) => void;
  onBulkDelete: () => void;
}

// Sorted by inbound then sort_order then id — the same order the subscription
// renderer uses, so the list mirrors the emitted link order.
function sortHosts(hosts: HostRecord[]): HostRecord[] {
  return [...hosts].sort((a, b) => {
    if (a.inboundId !== b.inboundId) return a.inboundId - b.inboundId;
    const sa = a.sortOrder ?? 0;
    const sb = b.sortOrder ?? 0;
    if (sa !== sb) return sa - sb;
    return a.id - b.id;
  });
}

export default function HostList(props: HostListProps) {
  const { t } = useTranslation();
  const {
    hosts, inboundOptions, loading, isMobile, selectedIds, onSelectionChange,
    onAdd, onEdit, onDelete, onToggleEnable, onMove, onBulkEnable, onBulkDelete,
  } = props;

  const inboundLabel = useMemo(() => {
    const map = new Map<number, string>();
    for (const ib of inboundOptions) map.set(ib.id, ib.remark || ib.tag || `#${ib.id}`);
    return map;
  }, [inboundOptions]);

  const sorted = useMemo(() => sortHosts(hosts), [hosts]);

  // Move is bounded to neighbours within the same inbound (sort_order is per-inbound).
  const movable = useMemo(() => {
    const byInbound = new Map<number, number>();
    const idxInGroup = new Map<number, number>();
    const counters = new Map<number, number>();
    for (const h of sorted) byInbound.set(h.inboundId, (byInbound.get(h.inboundId) ?? 0) + 1);
    for (const h of sorted) {
      const c = counters.get(h.inboundId) ?? 0;
      idxInGroup.set(h.id, c);
      counters.set(h.inboundId, c + 1);
    }
    return { byInbound, idxInGroup };
  }, [sorted]);

  // Column order requested: Actions, Enable, then the rest.
  const columns: ColumnsType<HostRecord> = [
    {
      title: t('pages.hosts.fields.actions'),
      key: 'actions',
      width: 168,
      render: (_, h) => {
        const idx = movable.idxInGroup.get(h.id) ?? 0;
        const count = movable.byInbound.get(h.inboundId) ?? 1;
        return (
          <Space size={2}>
            <Tooltip title={t('pages.hosts.moveUp')}>
              <Button size="small" type="text" icon={<ArrowUpOutlined />} aria-label={t('pages.hosts.moveUp')} disabled={idx === 0} onClick={() => onMove(h, 'up')} />
            </Tooltip>
            <Tooltip title={t('pages.hosts.moveDown')}>
              <Button size="small" type="text" icon={<ArrowDownOutlined />} aria-label={t('pages.hosts.moveDown')} disabled={idx >= count - 1} onClick={() => onMove(h, 'down')} />
            </Tooltip>
            <Tooltip title={t('edit')}>
              <Button size="small" type="text" icon={<EditOutlined />} aria-label={t('edit')} onClick={() => onEdit(h)} />
            </Tooltip>
            <Tooltip title={t('delete')}>
              <Button size="small" type="text" danger icon={<DeleteOutlined />} aria-label={t('delete')} onClick={() => onDelete(h)} />
            </Tooltip>
          </Space>
        );
      },
    },
    {
      title: t('pages.hosts.fields.enable'),
      key: 'enable',
      width: 90,
      render: (_, h) => (
        <Switch size="small" checked={!h.isDisabled} onChange={(next) => onToggleEnable(h, next)} />
      ),
    },
    {
      title: t('pages.hosts.fields.remark'),
      dataIndex: 'remark',
      key: 'remark',
      render: (_, h) => (
        <div className="host-remark-cell">
          <span className="host-remark">{h.remark}</span>
          {h.serverDescription ? <span className="host-desc">{h.serverDescription}</span> : null}
        </div>
      ),
    },
    {
      title: t('pages.hosts.fields.endpoint'),
      key: 'endpoint',
      render: (_, h) => <span className="host-endpoint">{`${h.address || '—'}${h.port ? `:${h.port}` : ''}`}</span>,
    },
    {
      title: t('pages.hosts.fields.inbound'),
      key: 'inbound',
      render: (_, h) => inboundLabel.get(h.inboundId) ?? `#${h.inboundId}`,
    },
    {
      title: t('pages.hosts.fields.security'),
      dataIndex: 'security',
      key: 'security',
      render: (security: string) => <Tag>{security || 'same'}</Tag>,
    },
    {
      title: t('pages.hosts.fields.tags'),
      key: 'tags',
      render: (_, h) => (h.tags && h.tags.length > 0
        ? <Space size={[0, 4]} wrap>{h.tags.map((tag) => <Tag key={tag} color="blue">{tag}</Tag>)}</Space>
        : <span className="host-muted">—</span>),
    },
  ];

  const toolbar = (
    <div className="card-toolbar">
      {selectedIds.length === 0 ? (
        <Button type="primary" icon={<PlusOutlined />} onClick={onAdd}>
          {!isMobile && t('pages.hosts.addHost')}
        </Button>
      ) : (
        <>
          <Tag
            color="blue"
            closable
            onClose={() => onSelectionChange([])}
            style={{ marginInlineEnd: 0, padding: '4px 8px', fontSize: 13 }}
          >
            {t('pages.hosts.selectedCount', { count: selectedIds.length })}
          </Tag>
          <Button onClick={() => onBulkEnable(true)}>{t('pages.hosts.bulkEnable')}</Button>
          <Button onClick={() => onBulkEnable(false)}>{t('pages.hosts.bulkDisable')}</Button>
          <Button danger icon={<DeleteOutlined />} onClick={onBulkDelete}>{t('pages.hosts.bulkDelete')}</Button>
        </>
      )}
    </div>
  );

  return (
    <Card size="small" hoverable title={toolbar} className="hosts-card">
      <Table<HostRecord>
        rowKey="id"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={sorted}
        pagination={false}
        scroll={{ x: 'max-content' }}
        rowSelection={{
          selectedRowKeys: selectedIds,
          onChange: (keys) => onSelectionChange(keys as number[]),
        }}
        locale={{
          emptyText: (
            <div className="card-empty">
              <GlobalOutlined style={{ fontSize: 32, marginBottom: 8 }} />
              <div>{t('noData')}</div>
            </div>
          ),
        }}
      />
    </Card>
  );
}
