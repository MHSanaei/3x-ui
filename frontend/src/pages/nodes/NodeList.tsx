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
  SafetyCertificateOutlined,
  TeamOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';

import NodeHistoryPanel from './NodeHistoryPanel';
import type { NodeRecord } from '@/api/queries/useNodesQuery';
import { isPanelUpdateAvailable } from '@/lib/panel-version';
import { activateOnKey } from '@/utils/a11y';
import './NodeList.css';

interface NodeListProps {
  nodes: NodeRecord[];
  loading?: boolean;
  isMobile?: boolean;
  latestVersion?: string;
  selectedIds: number[];
  onSelectionChange: (ids: number[]) => void;
  onAdd: () => void;
  onMtls: () => void;
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

interface HealthProps {
  status?: string;
  xrayState?: string;
  xrayError?: string;
}

// Purple: the node's panel API is reachable (status=online) but its Xray core
// has failed or been stopped. Distinct from a normal offline/unknown node.
const XRAY_ERROR_COLOR = '#722ED1';

// True when the panel is online but Xray itself reports error/stop.
function hasXrayProblem(status?: string, xrayState?: string): boolean {
  if (status !== 'online') return false;
  const xs = (xrayState || '').toLowerCase().trim();
  return xs === 'error' || xs === 'stop';
}

// Tooltip text + icon color for the status cell. A real probe error (lastError)
// is a warning and takes precedence; otherwise an Xray-core problem shows purple.
function statusIssue(record: Pick<NodeRecord, 'status' | 'xrayState' | 'xrayError' | 'lastError'>) {
  const tip = record.lastError || (hasXrayProblem(record.status, record.xrayState) ? record.xrayError : '') || '';
  const iconColor = !record.lastError && hasXrayProblem(record.status, record.xrayState)
    ? XRAY_ERROR_COLOR
    : 'var(--ant-color-warning)';
  return { tip, iconColor };
}

function StatusDot({ status, xrayState }: HealthProps) {
  if (status === 'online') {
    return hasXrayProblem(status, xrayState)
      ? <span className="xray-error-dot" />
      : <span className="online-dot" />;
  }
  return <Badge status={badgeStatus(status)} />;
}

function StatusLabel({ status, xrayState }: HealthProps) {
  const { t } = useTranslation();
  if (status === 'online') {
    const xs = (xrayState || '').toLowerCase().trim();
    if (xs === 'error' || xs === 'stop') {
      const detail = xs === 'error'
        ? t('pages.nodes.statusValues.xrayError')
        : t('pages.nodes.statusValues.xrayStopped');
      return (
        <span style={{ color: XRAY_ERROR_COLOR }}>
          {t('pages.nodes.statusValues.online')} ({detail})
        </span>
      );
    }
    return (
      <span style={{ color: 'var(--ant-color-success)' }}>
        {t('pages.nodes.statusValues.online')}
      </span>
    );
  }
  return <span>{t(`pages.nodes.statusValues.${status || 'unknown'}`)}</span>;
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
  onMtls,
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
            <Button type="text" size="small" style={{ fontSize: 16 }} icon={<ThunderboltOutlined />} aria-label={t('pages.nodes.probe')} onClick={() => onProbe(record)} />
          </Tooltip>
          {isUpdateEligible(record) && (
            <Tooltip title={t('pages.nodes.updatePanel')}>
              <Button type="text" size="small" style={{ fontSize: 16 }} icon={<CloudDownloadOutlined />} aria-label={t('pages.nodes.updatePanel')} onClick={() => onUpdateNode(record)} />
            </Tooltip>
          )}
          <Tooltip title={t('edit')}>
            <Button type="text" size="small" style={{ fontSize: 16 }} icon={<EditOutlined />} aria-label={t('edit')} onClick={() => onEdit(record)} />
          </Tooltip>
          <Tooltip title={t('delete')}>
            <Button type="text" size="small" danger style={{ fontSize: 16 }} icon={<DeleteOutlined />} aria-label={t('delete')} onClick={() => onDelete(record)} />
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
              <EyeOutlined className="ip-toggle-icon" role="button" tabIndex={0} aria-label={t('pages.index.toggleIpVisibility')} onClick={() => setShowAddress(false)} onKeyDown={activateOnKey(() => setShowAddress(false))} />
            ) : (
              <EyeInvisibleOutlined className="ip-toggle-icon" role="button" tabIndex={0} aria-label={t('pages.index.toggleIpVisibility')} onClick={() => setShowAddress(true)} onKeyDown={activateOnKey(() => setShowAddress(true))} />
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
      render: (_value, record) => {
        const { tip, iconColor } = statusIssue(record);
        return (
          <Space size={4}>
            <StatusDot status={record.status} xrayState={record.xrayState} />
            <StatusLabel status={record.status} xrayState={record.xrayState} />
            {tip && (
              <Tooltip title={tip}>
                <ExclamationCircleOutlined style={{ color: iconColor }} />
              </Tooltip>
            )}
          </Space>
        );
      },
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
                <Tag color="orange" style={{ margin: 0, cursor: 'pointer' }} role="button" tabIndex={0} onClick={() => onUpdateNode(record)} onKeyDown={activateOnKey(() => onUpdateNode(record))}>
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
      width: 180,
      render: (_value, record) => (
        <Space size={2}>
          <Tag className="client-count-tag" style={{ margin: 0, padding: '0 2px' }}><TeamOutlined /> {record.clientCount || 0}</Tag>
          {record.activeCount ? (
            <Tooltip title={t('subscription.active')}>
              <Tag color="green" className="client-count-tag" style={{ margin: 0, padding: '0 2px' }}>{record.activeCount}</Tag>
            </Tooltip>
          ) : null}
          {record.disabledCount ? (
            <Tooltip title={t('disabled')}>
              <Tag className="client-count-tag" style={{ margin: 0, padding: '0 2px' }}>{record.disabledCount}</Tag>
            </Tooltip>
          ) : null}
          {record.depletedCount ? (
            <Tooltip title={t('depleted')}>
              <Tag color="red" className="client-count-tag" style={{ margin: 0, padding: '0 2px' }}>{record.depletedCount}</Tag>
            </Tooltip>
          ) : null}
          {record.onlineCount ? (
            <Tooltip title={t('online')}>
              <Tag color="blue" className="client-count-tag" style={{ margin: 0, padding: '0 2px' }}>{record.onlineCount}</Tag>
            </Tooltip>
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
        <Button icon={<SafetyCertificateOutlined />} onClick={onMtls}>
          {t('pages.nodes.mtls.title')}
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
                    <StatusDot status={record.status} xrayState={record.xrayState} />
                    <span className="node-name">{record.name}</span>
                    <div className="card-actions">
                      <Tag icon={<ApartmentOutlined />} style={{ margin: 0 }}>{t('pages.nodes.subNode')}</Tag>
                    </div>
                  </div>
                </div>
              ) : (
                <div key={record.id} className="node-card">
                  {/* eslint-disable-next-line jsx-a11y/no-static-element-interactions, jsx-a11y/click-events-have-key-events -- mouse click-to-expand mirrors the keyboard-accessible chevron disclosure button */}
                  <div
                    className="card-head"
                    onClick={(e) => {
                      if (!(e.target as HTMLElement).closest('.card-actions')) toggleExpanded(record.id);
                    }}
                  >
                    <RightOutlined
                      className={`card-expand${expandedIds.has(record.id) ? ' is-expanded' : ''}`}
                      role="button"
                      tabIndex={0}
                      aria-expanded={expandedIds.has(record.id)}
                      aria-label={record.name}
                      onKeyDown={activateOnKey(() => toggleExpanded(record.id))}
                    />
                    <StatusDot status={record.status} xrayState={record.xrayState} />
                    <span className="node-name">{record.name}</span>
                    <div className="card-actions">
                      <Tooltip title={t('info')}>
                        <InfoCircleOutlined
                          className="row-action-trigger"
                          role="button"
                          tabIndex={0}
                          aria-label={t('info')}
                          onClick={() => setStatsNode(record)}
                          onKeyDown={activateOnKey(() => setStatsNode(record))}
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
                        <Button type="text" size="small" className="row-action-trigger" icon={<MoreOutlined />} aria-label={t('more')} />
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
                      <EyeOutlined className="ip-toggle-icon" role="button" tabIndex={0} aria-label={t('pages.index.toggleIpVisibility')} onClick={() => setShowAddress(false)} onKeyDown={activateOnKey(() => setShowAddress(false))} />
                    ) : (
                      <EyeInvisibleOutlined className="ip-toggle-icon" role="button" tabIndex={0} aria-label={t('pages.index.toggleIpVisibility')} onClick={() => setShowAddress(true)} onKeyDown={activateOnKey(() => setShowAddress(true))} />
                    )}
                  </Tooltip>
                </div>
                <div className="stat-row">
                  <span className="stat-label">{t('pages.nodes.status')}</span>
                  <StatusDot status={statsNode.status} xrayState={statsNode.xrayState} />
                  <StatusLabel status={statsNode.status} xrayState={statsNode.xrayState} />
                  {(() => {
                    const { tip, iconColor } = statusIssue(statsNode);
                    return tip ? (
                      <Tooltip title={tip}>
                        <ExclamationCircleOutlined style={{ color: iconColor }} />
                      </Tooltip>
                    ) : null;
                  })()}
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
                  <Tag><TeamOutlined /> {statsNode.clientCount || 0}</Tag>
                  {statsNode.activeCount ? (
                    <Tag color="green">{statsNode.activeCount} {t('subscription.active')}</Tag>
                  ) : null}
                  {statsNode.disabledCount ? (
                    <Tag>{statsNode.disabledCount} {t('disabled')}</Tag>
                  ) : null}
                  {statsNode.depletedCount ? (
                    <Tag color="red">{statsNode.depletedCount} {t('depleted')}</Tag>
                  ) : null}
                  {statsNode.onlineCount ? (
                    <Tag color="blue">{statsNode.onlineCount} {t('online')}</Tag>
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
