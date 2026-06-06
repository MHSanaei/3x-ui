import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Badge,
  Button,
  Card,
  Dropdown,
  Modal,
  Space,
  Switch,
  Table,
  Tag,
  Tooltip,
} from 'antd';
import type { BadgeProps } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  ApartmentOutlined,
  ClusterOutlined,
  CloudDownloadOutlined,
  DeleteOutlined,
  EditOutlined,
  ExclamationCircleOutlined,
  EyeInvisibleOutlined,
  EyeOutlined,
  InfoCircleOutlined,
  MoreOutlined,
  PlusOutlined,
  RightOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';

import NodeHistoryPanel from './NodeHistoryPanel';
import type { NodeRecord } from '@/api/queries/useNodesQuery';
import { isPanelUpdateAvailable } from '@/lib/panel-version';
import './NodeList.css';

interface NodeListProps {
  nodes: NodeRecord[];
  loading?: boolean;
  isMobile?: boolean;
  latestVersion?: string;
  selectedIds: number[];
  onSelectionChange: (ids: number[]) => void;
  onAdd: () => void;
  onEdit: (node: NodeRecord) => void;
  onDelete: (node: NodeRecord) => void;
  onProbe: (node: NodeRecord) => void;
  onToggleEnable: (node: NodeRecord, next: boolean) => void;
  onUpdateNode: (node: NodeRecord) => void;
  onUpdateSelected: () => void;
}

function isUpdateEligible(n: NodeRecord): boolean {
  return !!n.enable && n.status === 'online';
}

interface NodeRow extends NodeRecord {
  url: string;
  key: string | number;
}

function badgeStatus(status?: string): BadgeProps['status'] {
  switch (status) {
    case 'online': return 'success';
    case 'offline': return 'error';
    default: return 'default';
  }
}

function StatusDot({ status }: { status?: string }) {
  if (status === 'online') return <span className="online-dot" />;
  return <Badge status={badgeStatus(status)} />;
}

function StatusLabel({ status }: { status?: string }) {
  const { t } = useTranslation();
  return (
    <span style={status === 'online' ? { color: 'var(--ant-color-success)' } : undefined}>
      {t(`pages.nodes.statusValues.${status || 'unknown'}`)}
    </span>
  );
}

function formatPct(p?: number): string {
  if (typeof p !== 'number' || Number.isNaN(p)) return '-';
  return `${p.toFixed(1)}%`;
}

function formatUptime(secs?: number): string {
  if (!secs) return '-';
  const days = Math.floor(secs / 86400);
  const hours = Math.floor((secs % 86400) / 3600);
  if (days > 0) return `${days}d ${hours}h`;
  const mins = Math.floor((secs % 3600) / 60);
  if (hours > 0) return `${hours}h ${mins}m`;
  return `${mins}m`;
}

function useRelativeTime() {
  const { t } = useTranslation();
  return (unixSeconds?: number) => {
    if (!unixSeconds) return t('pages.nodes.never');
    const diffSec = Math.max(0, Math.floor(Date.now() / 1000 - unixSeconds));
    if (diffSec < 5) return t('pages.nodes.justNow');
    if (diffSec < 60) return `${diffSec}s`;
    if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m`;
    if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h`;
    return `${Math.floor(diffSec / 86400)}d`;
  };
}

export default function NodeList({
  nodes,
  loading = false,
  isMobile = false,
  latestVersion = '',
  selectedIds,
  onSelectionChange,
  onAdd,
  onEdit,
  onDelete,
  onProbe,
  onToggleEnable,
  onUpdateNode,
  onUpdateSelected,
}: NodeListProps) {
  const { t } = useTranslation();
  const relativeTime = useRelativeTime();

  const [showAddress, setShowAddress] = useState(false);
  const [statsNode, setStatsNode] = useState<NodeRow | null>(null);
  const [expandedIds, setExpandedIds] = useState<Set<number>>(new Set());

  // Map a node GUID to its display name so a transitive sub-node can show which
  // parent it is reached through (#4983).
  const nameByGuid = useMemo(() => {
    const m = new Map<string, string>();
    for (const n of nodes) if (n.guid) m.set(n.guid, n.name || n.guid);
    return m;
  }, [nodes]);

  // Order direct nodes first, each immediately followed by its transitive
  // sub-nodes, so the table reads as a parent -> child tree without colliding
  // with the per-row history expander (transitive nodes carry id 0).
  const dataSource = useMemo<NodeRow[]>(() => {
    const toRow = (n: NodeRecord): NodeRow => ({
      ...n,
      url: `${n.scheme}://${n.address}:${n.port}${n.basePath || '/'}`,
      key: n.transitive ? `t-${n.guid || ''}` : n.id,
    });
    const childrenByParent = new Map<string, NodeRecord[]>();
    for (const n of nodes) {
      if (n.transitive && n.parentGuid) {
        const arr = childrenByParent.get(n.parentGuid) || [];
        arr.push(n);
        childrenByParent.set(n.parentGuid, arr);
      }
    }
    const ordered: NodeRow[] = [];
    const added = new Set<string>();
    const push = (n: NodeRecord) => {
      const row = toRow(n);
      ordered.push(row);
      added.add(String(row.key));
    };
    for (const n of nodes) {
      if (n.transitive) continue;
      push(n);
      if (n.guid) for (const child of childrenByParent.get(n.guid) || []) push(child);
    }
    // Transitive nodes whose parent isn't in the list still get shown.
    for (const n of nodes) {
      if (n.transitive && !added.has(`t-${n.guid || ''}`)) push(n);
    }
    return ordered;
  }, [nodes]);

  function toggleExpanded(id: number) {
    setExpandedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  }

  const columns = useMemo<ColumnsType<NodeRow>>(() => [
    {
      title: t('pages.nodes.actions'),
      align: 'center',
      width: 190,
      render: (_value, record) => record.transitive ? (
        <Tooltip title={t('pages.nodes.subNodeTip', { parent: record.parentGuid ? (nameByGuid.get(record.parentGuid) || '-') : '-' })}>
          <Tag icon={<ApartmentOutlined />} style={{ margin: 0 }}>{t('pages.nodes.subNode')}</Tag>
        </Tooltip>
      ) : (
        <Space>
          <Tooltip title={t('pages.nodes.probe')}>
            <Button type="text" size="small" icon={<ThunderboltOutlined />} onClick={() => onProbe(record)} />
          </Tooltip>
          {isUpdateEligible(record) && (
            <Tooltip title={t('pages.nodes.updatePanel')}>
              <Button type="text" size="small" icon={<CloudDownloadOutlined />} onClick={() => onUpdateNode(record)} />
            </Tooltip>
          )}
          <Tooltip title={t('edit')}>
            <Button type="text" size="small" icon={<EditOutlined />} onClick={() => onEdit(record)} />
          </Tooltip>
          <Tooltip title={t('delete')}>
            <Button type="text" size="small" danger icon={<DeleteOutlined />} onClick={() => onDelete(record)} />
          </Tooltip>
        </Space>
      ),
    },
    {
      title: t('pages.nodes.enable'),
      dataIndex: 'enable',
      align: 'center',
      width: 80,
      render: (_value, record) => record.transitive ? (
        <span style={{ opacity: 0.4 }}>—</span>
      ) : (
        <Switch
          checked={!!record.enable}
          size="small"
          onChange={(v) => onToggleEnable(record, v)}
        />
      ),
    },
    {
      title: t('pages.nodes.name'),
      dataIndex: 'name',
      ellipsis: true,
      render: (_value, record) => (
        <div className="name-cell" style={record.transitive ? { paddingInlineStart: 20 } : undefined}>
          <span className="name">
            {record.transitive && <ApartmentOutlined style={{ marginInlineEnd: 6, opacity: 0.6 }} />}
            {record.name}
          </span>
          {record.remark && <span className="remark">{record.remark}</span>}
        </div>
      ),
    },
    {
      title: (
        <span className="address-header">
          {t('pages.nodes.address')}
          <Tooltip title={t('pages.index.toggleIpVisibility')}>
            {showAddress ? (
              <EyeOutlined className="ip-toggle-icon" onClick={() => setShowAddress(false)} />
            ) : (
              <EyeInvisibleOutlined className="ip-toggle-icon" onClick={() => setShowAddress(true)} />
            )}
          </Tooltip>
        </span>
      ),
      dataIndex: 'url',
      ellipsis: true,
      render: (_value, record) => (
        <a
          href={record.url}
          target="_blank"
          rel="noopener noreferrer"
          className={showAddress ? 'address-visible' : 'address-hidden'}
        >
          {record.url}
        </a>
      ),
    },
    {
      title: t('pages.nodes.status'),
      dataIndex: 'status',
      align: 'center',
      render: (_value, record) => (
        <Space size={4}>
          <StatusDot status={record.status} />
          <StatusLabel status={record.status} />
          {record.lastError && (
            <Tooltip title={record.lastError}>
              <ExclamationCircleOutlined style={{ color: 'var(--ant-color-warning)' }} />
            </Tooltip>
          )}
        </Space>
      ),
    },
    {
      title: t('pages.nodes.cpu'),
      dataIndex: 'cpuPct',
      align: 'center',
      width: 90,
      render: (_value, record) => formatPct(record.cpuPct),
    },
    {
      title: t('pages.nodes.mem'),
      dataIndex: 'memPct',
      align: 'center',
      width: 90,
      render: (_value, record) => formatPct(record.memPct),
    },
    {
      title: t('pages.nodes.xrayVersion'),
      dataIndex: 'xrayVersion',
      align: 'center',
      render: (_value, record) => record.xrayVersion || '-',
    },
    {
      title: t('pages.nodes.panelVersion') || 'Panel version',
      dataIndex: 'panelVersion',
      align: 'center',
      render: (_value, record) => {
        const canUpdate = isUpdateEligible(record)
          && isPanelUpdateAvailable(latestVersion, record.panelVersion || '');
        return (
          <Space size={4}>
            <span>{record.panelVersion || '-'}</span>
            {canUpdate && (
              <Tooltip title={`${t('pages.nodes.updateAvailable')}: ${latestVersion}`}>
                <Tag color="orange" style={{ margin: 0, cursor: 'pointer' }} onClick={() => onUpdateNode(record)}>
                  {t('pages.nodes.updateAvailable')}
                </Tag>
              </Tooltip>
            )}
          </Space>
        );
      },
    },
    {
      title: t('pages.nodes.uptime'),
      dataIndex: 'uptimeSecs',
      align: 'center',
      render: (_value, record) => formatUptime(record.uptimeSecs),
    },
    {
      title: t('clients'),
      align: 'center',
      width: 160,
      render: (_value, record) => (
        <Space size={4}>
          <Tag color="green">{record.clientCount || 0}</Tag>
          {record.onlineCount ? (
            <Tag color="blue">{record.onlineCount} {t('online')}</Tag>
          ) : null}
          {record.depletedCount ? (
            <Tag color="red">{record.depletedCount} {t('depleted')}</Tag>
          ) : null}
        </Space>
      ),
    },
    {
      title: t('pages.nodes.latency'),
      dataIndex: 'latencyMs',
      align: 'center',
      width: 100,
      render: (_value, record) =>
        record.latencyMs && record.latencyMs > 0 ? `${record.latencyMs} ms` : '-',
    },
    {
      title: t('pages.nodes.lastHeartbeat'),
      dataIndex: 'lastHeartbeat',
      align: 'center',
      width: 120,
      render: (_value, record) => relativeTime(record.lastHeartbeat),
    },
  ], [t, showAddress, relativeTime, latestVersion, onToggleEnable, onProbe, onEdit, onDelete, onUpdateNode, nameByGuid]);

  return (
    <Card size="small" hoverable>
      <div className="toolbar">
        <Button type="primary" icon={<PlusOutlined />} onClick={onAdd}>
          {t('pages.nodes.addNode')}
        </Button>
        {selectedIds.length > 0 && (
          <Button icon={<CloudDownloadOutlined />} onClick={onUpdateSelected}>
            {t('pages.nodes.updateSelected', { count: selectedIds.length })}
          </Button>
        )}
      </div>

      {isMobile ? (
        <>
          <div className="node-cards">
            {dataSource.length === 0 ? (
              <div className="card-empty">
                <ClusterOutlined style={{ fontSize: 28, opacity: 0.5 }} />
                <div>{t('noData')}</div>
              </div>
            ) : (
              dataSource.map((record) => record.transitive ? (
                <div key={String(record.key)} className="node-card" style={{ paddingInlineStart: 16, opacity: 0.85 }}>
                  <div className="card-head">
                    <ApartmentOutlined style={{ opacity: 0.6 }} />
                    <StatusDot status={record.status} />
                    <span className="node-name">{record.name}</span>
                    <div className="card-actions">
                      <Tag icon={<ApartmentOutlined />} style={{ margin: 0 }}>{t('pages.nodes.subNode')}</Tag>
                    </div>
                  </div>
                </div>
              ) : (
                <div key={record.id} className="node-card">
                  <div className="card-head" onClick={() => toggleExpanded(record.id)}>
                    <RightOutlined className={`card-expand${expandedIds.has(record.id) ? ' is-expanded' : ''}`} />
                    <StatusDot status={record.status} />
                    <span className="node-name">{record.name}</span>
                    <div className="card-actions" onClick={(e) => e.stopPropagation()}>
                      <Tooltip title={t('info')}>
                        <InfoCircleOutlined
                          className="row-action-trigger"
                          onClick={() => setStatsNode(record)}
                        />
                      </Tooltip>
                      <Switch
                        checked={!!record.enable}
                        size="small"
                        onChange={(v) => onToggleEnable(record, v)}
                      />
                      <Dropdown
                        trigger={['click']}
                        placement="bottomRight"
                        menu={{
                          items: [
                            {
                              key: 'probe',
                              label: <><ThunderboltOutlined /> {t('pages.nodes.probe')}</>,
                              onClick: () => onProbe(record),
                            },
                            ...(isUpdateEligible(record) ? [{
                              key: 'update',
                              label: <><CloudDownloadOutlined /> {t('pages.nodes.updatePanel')}</>,
                              onClick: () => onUpdateNode(record),
                            }] : []),
                            {
                              key: 'edit',
                              label: <><EditOutlined /> {t('edit')}</>,
                              onClick: () => onEdit(record),
                            },
                            {
                              key: 'delete',
                              danger: true,
                              label: <><DeleteOutlined /> {t('delete')}</>,
                              onClick: () => onDelete(record),
                            },
                          ],
                        }}
                      >
                        <MoreOutlined className="row-action-trigger" />
                      </Dropdown>
                    </div>
                  </div>

                  {expandedIds.has(record.id) && (
                    <div className="card-history">
                      <NodeHistoryPanel node={record} />
                    </div>
                  )}
                </div>
              ))
            )}
          </div>

          <Modal
            open={!!statsNode}
            footer={null}
            width={360}
            centered
            title={statsNode?.name || ''}
            onCancel={() => setStatsNode(null)}
          >
            {statsNode && (
              <div className="card-stats">
                {statsNode.remark && (
                  <div className="stat-row">
                    <span className="stat-label">{t('pages.nodes.name')}</span>
                    <span>{statsNode.remark}</span>
                  </div>
                )}
                <div className="stat-row">
                  <span className="stat-label">{t('pages.nodes.address')}</span>
                  <a
                    href={statsNode.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className={showAddress ? 'address-visible' : 'address-hidden'}
                  >
                    {statsNode.url}
                  </a>
                  <Tooltip title={t('pages.index.toggleIpVisibility')}>
                    {showAddress ? (
                      <EyeOutlined className="ip-toggle-icon" onClick={() => setShowAddress(false)} />
                    ) : (
                      <EyeInvisibleOutlined className="ip-toggle-icon" onClick={() => setShowAddress(true)} />
                    )}
                  </Tooltip>
                </div>
                <div className="stat-row">
                  <span className="stat-label">{t('pages.nodes.status')}</span>
                  <StatusDot status={statsNode.status} />
                  <StatusLabel status={statsNode.status} />
                  {statsNode.lastError && (
                    <Tooltip title={statsNode.lastError}>
                      <ExclamationCircleOutlined style={{ color: 'var(--ant-color-warning)' }} />
                    </Tooltip>
                  )}
                </div>
                <div className="stat-row">
                  <span className="stat-label">{t('pages.nodes.cpu')}</span>
                  <Tag>{formatPct(statsNode.cpuPct)}</Tag>
                </div>
                <div className="stat-row">
                  <span className="stat-label">{t('pages.nodes.mem')}</span>
                  <Tag>{formatPct(statsNode.memPct)}</Tag>
                </div>
                <div className="stat-row">
                  <span className="stat-label">{t('pages.nodes.xrayVersion')}</span>
                  <Tag>{statsNode.xrayVersion || '-'}</Tag>
                </div>
                <div className="stat-row">
                  <span className="stat-label">{t('pages.nodes.panelVersion') || 'Panel version'}</span>
                  <Tag>{statsNode.panelVersion || '-'}</Tag>
                </div>
                <div className="stat-row">
                  <span className="stat-label">{t('pages.nodes.uptime')}</span>
                  <Tag>{formatUptime(statsNode.uptimeSecs)}</Tag>
                </div>
                <div className="stat-row">
                  <span className="stat-label">{t('pages.nodes.latency')}</span>
                  <Tag>
                    {statsNode.latencyMs && statsNode.latencyMs > 0 ? `${statsNode.latencyMs} ms` : '-'}
                  </Tag>
                </div>
                <div className="stat-row">
                  <span className="stat-label">{t('clients')}</span>
                  <Tag color="green">{statsNode.clientCount || 0}</Tag>
                  {statsNode.onlineCount ? (
                    <Tag color="blue">{statsNode.onlineCount} {t('online')}</Tag>
                  ) : null}
                  {statsNode.depletedCount ? (
                    <Tag color="red">{statsNode.depletedCount} {t('depleted')}</Tag>
                  ) : null}
                </div>
                <div className="stat-row">
                  <span className="stat-label">{t('pages.nodes.lastHeartbeat')}</span>
                  <Tag>{relativeTime(statsNode.lastHeartbeat)}</Tag>
                </div>
              </div>
            )}
          </Modal>
        </>
      ) : (
        <Table<NodeRow>
          dataSource={dataSource}
          columns={columns}
          pagination={false}
          loading={loading}
          scroll={{ x: 'max-content' }}
          size="middle"
          rowKey="id"
          rowSelection={dataSource.length > 1 ? {
            selectedRowKeys: selectedIds,
            onChange: (keys) => onSelectionChange(keys.filter((k) => typeof k === 'number') as number[]),
            getCheckboxProps: (record) => ({ disabled: !!record.transitive || !isUpdateEligible(record) }),
          } : undefined}
          locale={{
            emptyText: (
              <div className="card-empty">
                <ClusterOutlined style={{ fontSize: 32, marginBottom: 8 }} />
                <div>{t('noData')}</div>
              </div>
            ),
          }}
          expandable={{
            expandedRowRender: (record) => <NodeHistoryPanel node={record} />,
            rowExpandable: (record) => !record.transitive,
          }}
        />
      )}
    </Card>
  );
}
