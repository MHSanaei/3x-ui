import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Dropdown, Switch, Tag } from 'antd';
import {
  MoreOutlined,
  EditOutlined,
  DeleteOutlined,
  ExportOutlined,
  ClusterOutlined,
  ArrowUpOutlined,
  ArrowDownOutlined,
  HolderOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';

import { useInboundOptions } from '@/api/queries/useInboundOptions';
import CriterionRow from './CriterionRow';
import { buildRemarkByTag, formatInboundTagList, inboundTagsDisplayTitle, isApiRule } from './helpers';
import type { RuleRow } from './types';

interface RoutingColumnsParams {
  isMobile: boolean;
  rowsLength: number;
  showSource: boolean;
  showBalancer: boolean;
  onHandlePointerDown: (idx: number, ev: React.PointerEvent) => void;
  openEdit: (idx: number) => void;
  moveUp: (idx: number) => void;
  moveDown: (idx: number) => void;
  confirmDelete: (idx: number) => void;
  toggleRule: (idx: number, enabled: boolean) => void;
}

export function useRoutingColumns({
  isMobile,
  rowsLength,
  showSource,
  showBalancer,
  onHandlePointerDown,
  openEdit,
  moveUp,
  moveDown,
  confirmDelete,
  toggleRule,
}: RoutingColumnsParams): ColumnsType<RuleRow> {
  const { t } = useTranslation();
  const { data: inboundOptions } = useInboundOptions();
  const remarkByTag = useMemo(() => buildRemarkByTag(inboundOptions || []), [inboundOptions]);
  return useMemo(
    () => [
      {
        title: '#',
        align: 'center',
        width: 60,
        key: 'index',
        render: (_v, _r, index) => (
          <div className="action-cell" style={{ justifyContent: 'center' }}>
            <HolderOutlined
              className="drag-handle"
              title={t('pages.xray.routing.dragToReorder')}
              onPointerDown={(ev: React.PointerEvent) => onHandlePointerDown(index, ev)}
            />
            <span className="row-index">{index + 1}</span>
          </div>
        ),
      },
      {
        title: t('pages.clients.actions'),
        align: 'center',
        width: 80,
        key: 'action',
        render: (_v, _r, index) => (
          <div className={!isMobile ? 'action-buttons' : ''} style={{ justifyContent: 'center', margin: 0 }}>
            {!isMobile && (
              <Button shape="circle" size="small" icon={<EditOutlined />} onClick={() => openEdit(index)} />
            )}
            <Dropdown
              trigger={['click']}
              menu={{
                items: [
                  ...(isMobile
                    ? [{ key: 'edit', label: <><EditOutlined /> {t('edit')}</>, onClick: () => openEdit(index) }]
                    : []),
                  { key: 'up', label: <ArrowUpOutlined />, disabled: index === 0, onClick: () => moveUp(index) },
                  {
                    key: 'down',
                    label: <ArrowDownOutlined />,
                    disabled: index === rowsLength - 1,
                    onClick: () => moveDown(index),
                  },
                  { key: 'del', danger: true, label: <><DeleteOutlined /> {t('delete')}</>, onClick: () => confirmDelete(index) },
                ],
              }}
            >
              <Button shape="circle" size="small" icon={<MoreOutlined />} />
            </Dropdown>
          </div>
        ),
      },
      {
        title: t('enable'),
        align: 'center',
        width: 80,
        key: 'enabled',
        render: (_v, _r, index) => (
          <Switch
            size="small"
            checked={_r.enabled !== false}
            onChange={(checked) => toggleRule(index, checked)}
            disabled={isApiRule(_r)}
          />
        ),
      },
      {
        title: t('pages.xray.rules.source'),
        align: 'left',
        width: 180,
        key: 'source',
        hidden: !showSource,
        render: (_v, record) => (
          <div className="criterion-flow">
            {record.sourceIP && <CriterionRow label="IP" value={record.sourceIP} title={`Source IP: ${record.sourceIP}`} />}
            {record.sourcePort && <CriterionRow label="Port" value={record.sourcePort} title={`Source port: ${record.sourcePort}`} />}
            {record.vlessRoute && <CriterionRow label="VLESS" value={record.vlessRoute} title={`VLESS route: ${record.vlessRoute}`} />}
            {!record.sourceIP && !record.sourcePort && !record.vlessRoute && <span className="criterion-empty">—</span>}
          </div>
        ),
      },
      {
        title: t('pages.inbounds.network'),
        align: 'left',
        width: 180,
        key: 'network',
        render: (_v, record) => (
          <div className="criterion-flow">
            {record.network && <CriterionRow label="L4" value={record.network.toUpperCase()} title={`L4: ${record.network.toUpperCase()}`} />}
            {record.protocol && <CriterionRow label="Protocol" value={record.protocol} title={`Protocol: ${record.protocol}`} />}
            {record.attrs && <CriterionRow label="Attrs" value={record.attrs} title={`Attrs: ${record.attrs}`} />}
            {!record.network && !record.protocol && !record.attrs && <span className="criterion-empty">—</span>}
          </div>
        ),
      },
      {
        title: t('pages.xray.rules.dest'),
        align: 'left',
        width: 200,
        key: 'destination',
        render: (_v, record) => (
          <div className="criterion-flow">
            {record.ip && <CriterionRow label="IP" value={record.ip} title={`Destination IP: ${record.ip}`} />}
            {record.domain && <CriterionRow label="Domain" value={record.domain} title={`Domain: ${record.domain}`} />}
            {record.port && <CriterionRow label="Port" value={record.port} title={`Destination port: ${record.port}`} />}
            {!record.ip && !record.domain && !record.port && <span className="criterion-empty">—</span>}
          </div>
        ),
      },
      {
        title: t('pages.xray.Inbounds'),
        align: 'left',
        width: 180,
        key: 'inbound',
        render: (_v, record) => {
          const inboundParts = formatInboundTagList(record.inboundTag, remarkByTag);
          return (
            <div className="criterion-flow">
              {inboundParts.length > 0 && (
                <CriterionRow
                  label="Tag"
                  values={inboundParts}
                  title={`Inbound tag: ${inboundTagsDisplayTitle(record.inboundTag, remarkByTag) ?? inboundParts.join(', ')}`}
                />
              )}
              {record.user && <CriterionRow label="User" value={record.user} title={`User: ${record.user}`} />}
              {inboundParts.length === 0 && !record.user && <span className="criterion-empty">—</span>}
            </div>
          );
        },
      },
      {
        title: t('pages.xray.Outbounds'),
        align: 'left',
        width: 170,
        key: 'outbound',
        render: (_v, record) =>
          record.outboundTag ? (
            <div className="target-row">
              <ExportOutlined className="target-icon" />
              <Tag color="green">{record.outboundTag}</Tag>
            </div>
          ) : (
            <span className="criterion-empty">—</span>
          ),
      },
      {
        title: t('pages.xray.Balancers'),
        align: 'left',
        width: 150,
        key: 'balancer',
        hidden: !showBalancer,
        render: (_v, record) =>
          record.balancerTag ? (
            <div className="target-row">
              <ClusterOutlined className="target-icon" />
              <Tag color="purple">{record.balancerTag}</Tag>
            </div>
          ) : (
            <span className="criterion-empty">—</span>
          ),
      },
    ],
    [t, isMobile, rowsLength, showSource, showBalancer, remarkByTag, onHandlePointerDown, openEdit, moveUp, moveDown, confirmDelete, toggleRule],
  );
}
