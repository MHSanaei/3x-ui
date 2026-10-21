import { useEffect, useMemo, useRef, useState } from 'react';
import type { PointerEvent as ReactPointerEvent } from 'react';
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
  const [drag, setDrag] = useState<{ from: string; to: string; after: boolean } | null>(null);
  const tableRef = useRef<HTMLDivElement>(null);
  const stopDrag = useRef<(() => void) | null>(null);
  useEffect(() => () => stopDrag.current?.(), []);

  function onHandlePointerDown(from: string, event: ReactPointerEvent<HTMLElement>) {
    if (
      reordering ||
      loading ||
      event.isPrimary === false ||
      (event.button != null && event.button !== 0)
    )
      return;
    event.preventDefault();
    event.currentTarget.setPointerCapture?.(event.pointerId);
    stopDrag.current?.();
    let to = from;
    let after = false;
    let moved = false;
    const pointerId = event.pointerId;
    const startY = event.clientY;
    setDrag({ from, to, after });
    const move = (e: PointerEvent) => {
      if (e.pointerId !== pointerId || Math.abs(e.clientY - startY) < 5) return;
      const row = document
        .elementFromPoint(e.clientX, e.clientY)
        ?.closest<HTMLElement>('tr[data-row-key]');
      if (!row || !tableRef.current?.contains(row)) return;
      moved = true;
      to = row.dataset.rowKey || from;
      const rect = row.getBoundingClientRect();
      after = e.clientY > rect.top + rect.height / 2;
      setDrag({ from, to, after });
    };
    const cleanup = () => {
      document.removeEventListener('pointermove', move);
      document.removeEventListener('pointerup', up);
      document.removeEventListener('pointercancel', cancel);
      stopDrag.current = null;
    };
    const cancel = (e: PointerEvent) => {
      if (e.pointerId !== pointerId) return;
      cleanup();
      setDrag(null);
    };
    const up = (e: PointerEvent) => {
      if (e.pointerId !== pointerId) return;
      cleanup();
      setDrag(null);
      if (!moved) return;
      const ids = sorted.map((h) => h.groupId);
      const source = ids.indexOf(from);
      const target = ids.indexOf(to);
      if (source < 0 || target < 0) return;
      const slot = target + (after ? 1 : 0);
      const destination = slot - (source < slot ? 1 : 0);
      if (source === destination) return;
      ids.splice(source, 1);
      ids.splice(destination, 0, from);
      onReorder(ids);
    };
    stopDrag.current = cleanup;
    document.addEventListener('pointermove', move);
    document.addEventListener('pointerup', up);
    document.addEventListener('pointercancel', cancel);
  }

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
              icon={<HolderOutlined />}
              aria-label={t('pages.hosts.dragToReorder', { name: h.remark || h.groupId })}
              disabled={reordering || loading}
              onPointerDown={(event) => onHandlePointerDown(h.groupId, event)}
            />
            <Tooltip title={t('pages.hosts.moveUp')}>
              <Button
                size="small"
                type="text"
                icon={<ArrowUpOutlined />}
                aria-label={t('pages.hosts.moveUp')}
                disabled={reordering || loading || idx === 0}
                onClick={() => onMove(h, 'up')}
              />
            </Tooltip>
            <Tooltip title={t('pages.hosts.moveDown')}>
              <Button
                size="small"
                type="text"
                icon={<ArrowDownOutlined />}
                aria-label={t('pages.hosts.moveDown')}
                disabled={reordering || loading || idx >= count - 1}
                onClick={() => onMove(h, 'down')}
              />
            </Tooltip>
            <Tooltip title={t('edit')}>
              <Button
                size="small"
                type="text"
                icon={<EditOutlined />}
                aria-label={t('edit')}
                onClick={() => onEdit(h)}
              />
            </Tooltip>
            <Tooltip title={t('delete')}>
              <Button
                size="small"
                type="text"
                danger
                icon={<DeleteOutlined />}
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
            {t('pages.hosts.selectedCount', { count: selectedGroupIds.length })}
          </Tag>
          <Button onClick={() => onBulkEnable(true)}>{t('pages.hosts.bulkEnable')}</Button>
          <Button onClick={() => onBulkEnable(false)}>{t('pages.hosts.bulkDisable')}</Button>
          <Button danger icon={<DeleteOutlined />} onClick={onBulkDelete}>
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
              <GlobalOutlined style={{ fontSize: 32, marginBottom: 8 }} />
              <div>{t('noData')}</div>
            </div>
          ),
        }}
      />
    </Card>
  );
}
