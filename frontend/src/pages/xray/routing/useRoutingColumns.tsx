import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Dropdown, Tag } from 'antd';
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

import CriterionRow from './CriterionRow';
import type { RuleRow } from './types';

interface RoutingColumnsParams {
  isMobile: boolean;
  rowsLength: number;
  onHandlePointerDown: (idx: number, ev: React.PointerEvent) => void;
  openEdit: (idx: number) => void;
  moveUp: (idx: number) => void;
  moveDown: (idx: number) => void;
  confirmDelete: (idx: number) => void;
}

export function useRoutingColumns({
  isMobile,
  rowsLength,
  onHandlePointerDown,
  openEdit,
  moveUp,
  moveDown,
  confirmDelete,
}: RoutingColumnsParams): ColumnsType<RuleRow> {
  const { t } = useTranslation();
  return useMemo(
    () => [
      {
        title: '#',
        align: 'center',
        width: 100,
        key: 'action',
        render: (_v, _r, index) => (
          <div className="action-cell">
            <HolderOutlined
              className="drag-handle"
              title={t('pages.xray.routing.dragToReorder')}
              onPointerDown={(ev: React.PointerEvent) => onHandlePointerDown(index, ev)}
            />
            <span className="row-index">{index + 1}</span>
            <div className={!isMobile ? 'action-buttons' : ''}>
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
          </div>
        ),
      },
      {
        title: t('pages.xray.rules.source'),
        align: 'left',
        width: 180,
        key: 'source',
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
            {record.network && <CriterionRow label="L4" value={record.network} title={`L4: ${record.network}`} />}
            {record.protocol && <CriterionRow label="Protocol" value={record.protocol} title={`Protocol: ${record.protocol}`} />}
            {record.attrs && <CriterionRow label="Attrs" value={record.attrs} title={`Attrs: ${record.attrs}`} />}
            {!record.network && !record.protocol && !record.attrs && <span className="criterion-empty">—</span>}
          </div>
        ),
      },
      {
        title: t('pages.xray.rules.dest'),
        align: 'left',
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
        render: (_v, record) => (
          <div className="criterion-flow">
            {record.inboundTag && <CriterionRow label="Tag" value={record.inboundTag} title={`Inbound tag: ${record.inboundTag}`} />}
            {record.user && <CriterionRow label="User" value={record.user} title={`User: ${record.user}`} />}
            {!record.inboundTag && !record.user && <span className="criterion-empty">—</span>}
          </div>
        ),
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t, isMobile, rowsLength],
  );
}
