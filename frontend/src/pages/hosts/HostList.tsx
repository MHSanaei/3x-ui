import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, Space, Switch, Table, Tag, Tooltip } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
} from '@ant-design/icons';

import type { HostRecord } from '@/api/queries/useHostsQuery';
import type { InboundOption } from '@/schemas/client';
import './HostList.css';

interface HostListProps {
  hosts: HostRecord[];
  inboundOptions: InboundOption[];
  loading?: boolean;
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
    hosts, inboundOptions, loading, selectedIds, onSelectionChange,
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
    const byInbound = new Map<number, number>(); // inboundId -> count
    for (const h of sorted) byInbound.set(h.inboundId, (byInbound.get(h.inboundId) ?? 0) + 1);
    const idxInGroup = new Map<number, number>();
    const counters = new Map<number, number>();
    for (const h of sorted) {
      const c = counters.get(h.inboundId) ?? 0;
      idxInGroup.set(h.id, c);
      counters.set(h.inboundId, c + 1);
    }
    return { byInbound, idxInGroup };
  }, [sorted]);

  const columns: ColumnsType<HostRecord> = [
    {
      title: t('pages.hosts.fields.remark'),
      dataIndex: 'remark',
      key: 'remark',
      render: (remark: string) => <span className="host-remark">{remark}</span>,
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
    {
      title: t('pages.hosts.fields.enable'),
      key: 'enable',
      render: (_, h) => (
        <Switch
          size="small"
          checked={!h.isDisabled}
          onChange={(next) => onToggleEnable(h, next)}
        />
      ),
    },
    {
      title: t('pages.hosts.fields.actions'),
      key: 'actions',
      width: 180,
      render: (_, h) => {
        const idx = movable.idxInGroup.get(h.id) ?? 0;
        const count = movable.byInbound.get(h.inboundId) ?? 1;
        return (
          <Space size="small">
            <Tooltip title={t('pages.hosts.moveUp')}>
              <Button size="small" type="text" icon={<ArrowUpOutlined />} disabled={idx === 0} onClick={() => onMove(h, 'up')} />
            </Tooltip>
            <Tooltip title={t('pages.hosts.moveDown')}>
              <Button size="small" type="text" icon={<ArrowDownOutlined />} disabled={idx >= count - 1} onClick={() => onMove(h, 'down')} />
            </Tooltip>
            <Tooltip title={t('edit')}>
              <Button size="small" type="text" icon={<EditOutlined />} onClick={() => onEdit(h)} />
            </Tooltip>
            <Tooltip title={t('delete')}>
              <Button size="small" type="text" danger icon={<DeleteOutlined />} onClick={() => onDelete(h)} />
            </Tooltip>
          </Space>
        );
      },
    },
  ];

  const toolbar = (
    <Space wrap>
      <Button type="primary" icon={<PlusOutlined />} onClick={onAdd}>{t('pages.hosts.addHost')}</Button>
      <Button disabled={selectedIds.length === 0} onClick={() => onBulkEnable(true)}>{t('pages.hosts.bulkEnable')}</Button>
      <Button disabled={selectedIds.length === 0} onClick={() => onBulkEnable(false)}>{t('pages.hosts.bulkDisable')}</Button>
      <Button danger disabled={selectedIds.length === 0} onClick={onBulkDelete}>{t('pages.hosts.bulkDelete')}</Button>
    </Space>
  );

  return (
    <Card size="small" title={t('menu.hosts')} extra={toolbar} className="hosts-card">
      <Table<HostRecord>
        rowKey="id"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={sorted}
        pagination={false}
        rowSelection={{
          selectedRowKeys: selectedIds,
          onChange: (keys) => onSelectionChange(keys as number[]),
        }}
        locale={{ emptyText: t('pages.hosts.empty') }}
      />
    </Card>
  );
}
