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
import type { NodeRecord } from '@/hooks/useNodes';
import './NodeList.css';

interface NodeListProps {
  nodes: NodeRecord[];
  loading?: boolean;
  isMobile?: boolean;
  onAdd: () => void;
  onEdit: (node: NodeRecord) => void;
  onDelete: (node: NodeRecord) => void;
  onProbe: (node: NodeRecord) => void;
  onToggleEnable: (node: NodeRecord, next: boolean) => void;
}

interface NodeRow extends NodeRecord {
  url: string;
  key: number;
}

function badgeStatus(status?: string): BadgeProps['status'] {
  switch (status) {
    case 'online': return 'success';
    case 'offline': return 'error';
    default: return 'default';
  }
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
  onAdd,
  onEdit,
  onDelete,
  onProbe,
  onToggleEnable,
}: NodeListProps) {
  const { t } = useTranslation();
  const relativeTime = useRelativeTime();

  const [showAddress, setShowAddress] = useState(false);
  const [statsNode, setStatsNode] = useState<NodeRow | null>(null);
  const [expandedIds, setExpandedIds] = useState<Set<number>>(new Set());

  const dataSource = useMemo<NodeRow[]>(
    () => nodes.map((n) => ({
      ...n,
      url: `${n.scheme}://${n.address}:${n.port}${n.basePath || '/'}`,
      key: n.id,
    })),
    [nodes],
  );

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
      width: 160,
      render: (_value, record) => (
        <Space>
          <Tooltip title={t('pages.nodes.probe')}>
            <Button type="text" size="small" icon={<ThunderboltOutlined />} onClick={() => onProbe(record)} />
          </Tooltip>
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
      render: (_value, record) => (
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
        <div className="name-cell">
          <span className="name">{record.name}</span>
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
          <Badge status={badgeStatus(record.status)} />
          <span>{t(`pages.nodes.statusValues.${record.status || 'unknown'}`)}</span>
          {record.lastError && (
            <Tooltip title={record.lastError}>
              <ExclamationCircleOutlined style={{ color: '#faad14' }} />
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
      render: (_value, record) => record.panelVersion || '-',
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
  ], [t, showAddress, relativeTime, onToggleEnable, onProbe, onEdit, onDelete]);

  return (
    <Card size="small" hoverable>
      <div className="toolbar">
        <Button type="primary" icon={<PlusOutlined />} onClick={onAdd}>
          {t('pages.nodes.addNode')}
        </Button>
      </div>

      {isMobile ? (
        <>
          <div className="node-cards">
            {dataSource.length === 0 ? (
              <div className="card-empty">—</div>
            ) : (
              dataSource.map((record) => (
                <div key={record.id} className="node-card">
                  <div className="card-head" onClick={() => toggleExpanded(record.id)}>
                    <RightOutlined className={`card-expand${expandedIds.has(record.id) ? ' is-expanded' : ''}`} />
                    <Badge status={badgeStatus(record.status)} />
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
                  <Badge status={badgeStatus(statsNode.status)} />
                  <span>{t(`pages.nodes.statusValues.${statsNode.status || 'unknown'}`)}</span>
                  {statsNode.lastError && (
                    <Tooltip title={statsNode.lastError}>
                      <ExclamationCircleOutlined style={{ color: '#faad14' }} />
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
          expandable={{
            expandedRowRender: (record) => <NodeHistoryPanel node={record} />,
          }}
        />
      )}
    </Card>
  );
}
