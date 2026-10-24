import { useMemo, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, Popover, Space, Switch, Table, Tag, Tooltip } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  DeleteOutlined,
  EditOutlined,
  GlobalOutlined,
  HolderOutlined,
  PlusOutlined,
} from '@ant-design/icons';

import type { HostRecord } from '@/api/queries/useHostsQuery';
import type { InboundOption } from '@/schemas/client';
import { usePointerRowReorder, type PointerRowDrop } from '@/hooks/usePointerRowReorder';
import './HostList.css';

interface HostListProps {
  hosts: HostRecord[];
  inboundOptions: InboundOption[];
  loading?: boolean;
  isMobile?: boolean;
  selectedGroupIds: string[];
  onSelectionChange: (groupIds: string[]) => void;
  onAdd: () => void;
  onEdit: (host: HostRecord) => void;
  onDelete: (host: HostRecord) => void;
  onToggleEnable: (host: HostRecord, next: boolean) => void;
  onMove: (host: HostRecord, dir: 'up' | 'down') => void;
  onReorder: (groupIds: string[]) => void;
  reordering?: boolean;
  onBulkEnable: (enable: boolean) => void;
  onBulkDelete: () => void;
}

const INBOUND_PROTOCOL_COLORS: Record<string, string> = {
  vless: 'blue',
  vmess: 'geekblue',
  trojan: 'volcano',
  shadowsocks: 'magenta',
  hysteria: 'cyan',
  hysteria2: 'green',
  wireguard: 'gold',
  amneziawg: 'yellow',
  http: 'purple',
  mixed: 'lime',
  tunnel: 'orange',
  tuic: 'orange',
  masque: 'red',
};

export function sortHosts(hosts: HostRecord[]): HostRecord[] {
  return [...hosts].sort((a, b) => {
    const sa = a.sortOrder ?? 0;
    const sb = b.sortOrder ?? 0;
    if (sa !== sb) return sa - sb;
    const ar = a.remark || '';
    const br = b.remark || '';
    if (ar !== br) return ar < br ? -1 : 1;
    return a.groupId < b.groupId ? -1 : a.groupId > b.groupId ? 1 : 0;
  });
}

function hostDropDestination(keys: string[], { from, to, after }: PointerRowDrop<string>) {
  const source = keys.indexOf(from);
  const slot = keys.indexOf(to) + (after ? 1 : 0);
  return slot - (source < slot ? 1 : 0);
}

export default function HostList(props: HostListProps) {
  const { t } = useTranslation();
  const {
    hosts,
    inboundOptions,
    loading,
    isMobile,
    selectedGroupIds,
    onSelectionChange,
    onAdd,
    onEdit,
    onDelete,
    onToggleEnable,
    onMove,
    onReorder,
    reordering,
    onBulkEnable,
    onBulkDelete,
  } = props;

  const inboundsMap = useMemo(() => {
    const map = new Map<number, InboundOption>();
    for (const ib of inboundOptions) map.set(ib.id, ib);
    return map;
  }, [inboundOptions]);

  const sorted = useMemo(() => sortHosts(hosts), [hosts]);
  const tableRef = useRef<HTMLDivElement>(null);
  const keys = sorted.map((h) => h.groupId);
  const { drag, onPointerDown: onHandlePointerDown } = usePointerRowReorder({
    containerRef: tableRef,
    getRowKey: (row) => keys.find((key) => key === row.dataset.rowKey) ?? null,
    disabled: reordering || loading,
    isNoop: (drop) => keys.indexOf(drop.from) === hostDropDestination(keys, drop),
    onDrop: (drop) => {
      const ids = [...keys];
      const destination = hostDropDestination(keys, drop);
      ids.splice(ids.indexOf(drop.from), 1);
      ids.splice(destination, 0, drop.from);
      onReorder(ids);
    },
  });

  const columns: ColumnsType<HostRecord> = [
    {
      title: t('pages.hosts.fields.actions'),
      key: 'actions',
      width: 196,
      render: (_, h, idx) => {
        const count = sorted.length;
        return (
          <Space size={2}>
            <Button
              size="small"
              type="text"
              className="host-drag-handle"
              icon={<HolderOutlined aria-hidden />}
              aria-label={t('pages.hosts.dragToReorder', { name: h.remark || h.groupId })}
              disabled={reordering || loading}
              onPointerDown={(event) => onHandlePointerDown(h.groupId, event)}
            />
            <Tooltip title={t('pages.hosts.moveUp')}>
              <Button
                size="small"
                type="text"
                icon={<ArrowUpOutlined aria-hidden />}
                aria-label={t('pages.hosts.moveUp')}
                disabled={reordering || loading || idx === 0}
                onClick={() => onMove(h, 'up')}
              />
            </Tooltip>
            <Tooltip title={t('pages.hosts.moveDown')}>
              <Button
                size="small"
                type="text"
                icon={<ArrowDownOutlined aria-hidden />}
                aria-label={t('pages.hosts.moveDown')}
                disabled={reordering || loading || idx >= count - 1}
                onClick={() => onMove(h, 'down')}
              />
            </Tooltip>
            <Tooltip title={t('edit')}>
              <Button
                size="small"
                type="text"
                icon={<EditOutlined aria-hidden />}
                aria-label={t('edit')}
                onClick={() => onEdit(h)}
              />
            </Tooltip>
            <Tooltip title={t('delete')}>
              <Button
                size="small"
                type="text"
                danger
                icon={<DeleteOutlined aria-hidden />}
                aria-label={t('delete')}
                onClick={() => onDelete(h)}
              />
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
        <Switch
          size="small"
          checked={!h.isDisabled}
          aria-label={t('pages.hosts.fields.enable')}
          onChange={(next) => onToggleEnable(h, next)}
        />
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
      render: (_, h) => {
        const addrs = h.hosts?.filter((a) => a.trim() !== '') || [];
        if (addrs.length === 0)
          return <Tag color="orange">{t('pages.hosts.fields.inheritAddress') || 'inherits'}</Tag>;
        const visible = addrs.slice(0, 1);
        const overflow = addrs.slice(1);
        return (
          <>
            {visible.map((addr) => (
              <Tag key={addr}>{addr}</Tag>
            ))}
            {overflow.length > 0 && (
              <Popover
                trigger="click"
                placement="bottomRight"
                content={
                  <div
                    style={{
                      display: 'flex',
                      flexDirection: 'column',
                      gap: 4,
                      maxWidth: 280,
                      maxHeight: 280,
                      overflowY: 'auto',
                    }}
                  >
                    {overflow.map((addr) => (
                      <Tag key={addr}>{addr}</Tag>
                    ))}
                  </div>
                }
              >
                <Tag color="default" style={{ margin: 2, cursor: 'pointer' }}>
                  +{overflow.length}
                </Tag>
              </Popover>
            )}
          </>
        );
      },
    },
    {
      title: t('pages.hosts.fields.inbound'),
      key: 'inbound',
      render: (_, h) => {
        const ids = h.inboundIds || [];
        if (ids.length === 0) return <span className="host-muted">—</span>;
        const visible = ids.slice(0, 1);
        const overflow = ids.slice(1);
        const chip = (id: number) => {
          const ib = inboundsMap.get(id);
          const label = ib ? ib.remark || ib.tag || `#${id}` : `#${id}`;
          const proto = (ib?.protocol || '').toLowerCase();
          const color = INBOUND_PROTOCOL_COLORS[proto] ?? 'default';
          return (
            <Tooltip key={id} title={label}>
              <Tag color={color} style={{ margin: 2 }}>
                {label}
              </Tag>
            </Tooltip>
          );
        };
        return (
          <>
            {visible.map(chip)}
            {overflow.length > 0 && (
              <Popover
                trigger="click"
                placement="bottomRight"
                content={
                  <div
                    style={{
                      display: 'flex',
                      flexDirection: 'column',
                      gap: 4,
                      maxWidth: 280,
                      maxHeight: 280,
                      overflowY: 'auto',
                    }}
                  >
                    {overflow.map(chip)}
                  </div>
                }
              >
                <Tag color="default" style={{ margin: 2, cursor: 'pointer' }}>
                  +{overflow.length}
                </Tag>
              </Popover>
            )}
          </>
        );
      },
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
      render: (_, h) =>
        h.tags && h.tags.length > 0 ? (
          <Space size={[0, 4]} wrap>
            {h.tags.map((tag) => (
              <Tag key={tag} color="blue">
                {tag}
              </Tag>
            ))}
          </Space>
        ) : (
          <span className="host-muted">—</span>
        ),
    },
  ];

  const toolbar = (
    <div className="card-toolbar">
      {selectedGroupIds.length === 0 ? (
        <Button type="primary" icon={<PlusOutlined aria-hidden />} onClick={onAdd}>
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
            {t('pages.hosts.selectedCount', { count: selectedGroupIds.length })}
          </Tag>
          <Button onClick={() => onBulkEnable(true)}>{t('pages.hosts.bulkEnable')}</Button>
          <Button onClick={() => onBulkEnable(false)}>{t('pages.hosts.bulkDisable')}</Button>
          <Button danger icon={<DeleteOutlined aria-hidden />} onClick={onBulkDelete}>
            {t('pages.hosts.bulkDelete')}
          </Button>
        </>
      )}
    </div>
  );

  return (
    <Card size="small" hoverable title={toolbar} className="hosts-card" ref={tableRef}>
      <Table<HostRecord>
        rowKey="groupId"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={sorted}
        pagination={false}
        rowClassName={(h) =>
          [
            drag?.from === h.groupId ? 'host-dragging' : '',
            drag?.to === h.groupId ? (drag.after ? 'host-drop-after' : 'host-drop-before') : '',
          ].join(' ')
        }
        scroll={{ x: 'max-content' }}
        rowSelection={{
          selectedRowKeys: selectedGroupIds,
          onChange: (keys) => onSelectionChange(keys as string[]),
        }}
        locale={{
          emptyText: (
            <div className="card-empty">
              <GlobalOutlined aria-hidden style={{ fontSize: 32, marginBottom: 8 }} />
              <div>{t('noData')}</div>
            </div>
          ),
        }}
      />
    </Card>
  );
}
