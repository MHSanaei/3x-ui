import { useTranslation } from 'react-i18next';
import { Button, Dropdown, Tag, Tooltip } from 'antd';
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

import { chipPreview, ruleCriteriaChips } from './helpers';
import type { RuleRow } from './types';

interface RuleCardListProps {
  rows: RuleRow[];
  draggedIndex: number | null;
  dropTargetIndex: number | null;
  onHandlePointerDown: (idx: number, ev: React.PointerEvent) => void;
  openEdit: (idx: number) => void;
  moveUp: (idx: number) => void;
  moveDown: (idx: number) => void;
  confirmDelete: (idx: number) => void;
}

export default function RuleCardList({
  rows,
  draggedIndex,
  dropTargetIndex,
  onHandlePointerDown,
  openEdit,
  moveUp,
  moveDown,
  confirmDelete,
}: RuleCardListProps) {
  const { t } = useTranslation();
  return (
    <div className="rule-list">
      {rows.length === 0 ? (
        <div className="rule-empty">—</div>
      ) : (
        rows.map((rule, index) => (
          <div
            key={rule.key}
            className={`rule-card ${draggedIndex === index ? 'row-dragging' : ''} ${
              dropTargetIndex === index && draggedIndex != null && index < draggedIndex ? 'drop-before' : ''
            } ${dropTargetIndex === index && draggedIndex != null && index > draggedIndex ? 'drop-after' : ''}`}
            data-row-key={index}
          >
            <div className="rule-card-head">
              <HolderOutlined
                className="drag-handle"
                onPointerDown={(ev) => onHandlePointerDown(index, ev)}
              />
              <span className="rule-number">#{index + 1}</span>
              <Dropdown
                trigger={['click']}
                menu={{
                  items: [
                    { key: 'edit', label: <><EditOutlined /> {t('edit')}</>, onClick: () => openEdit(index) },
                    { key: 'up', label: <ArrowUpOutlined />, disabled: index === 0, onClick: () => moveUp(index) },
                    { key: 'down', label: <ArrowDownOutlined />, disabled: index === rows.length - 1, onClick: () => moveDown(index) },
                    { key: 'del', danger: true, label: <><DeleteOutlined /> {t('delete')}</>, onClick: () => confirmDelete(index) },
                  ],
                }}
              >
                <Button shape="circle" size="small" icon={<MoreOutlined />} />
              </Dropdown>
            </div>

            <div className="rule-flow">
              <div className="flow-side">
                <span className="flow-label">{t('pages.xray.Inbounds')}</span>
                {rule.inboundTag ? (
                  <Tag color="blue" className="flow-tag">{chipPreview(rule.inboundTag)}</Tag>
                ) : (
                  <span className="criterion-empty">any</span>
                )}
              </div>
              <span className="flow-arrow">→</span>
              <div className="flow-side flow-side-target">
                <span className="flow-label">
                  {rule.balancerTag ? t('pages.xray.balancer') || 'Balancer' : t('pages.xray.Outbounds')}
                </span>
                {rule.outboundTag ? (
                  <Tag color="green" className="flow-tag">
                    <ExportOutlined /> {rule.outboundTag}
                  </Tag>
                ) : rule.balancerTag ? (
                  <Tag color="purple" className="flow-tag">
                    <ClusterOutlined /> {rule.balancerTag}
                  </Tag>
                ) : (
                  <span className="criterion-empty">—</span>
                )}
              </div>
            </div>

            {ruleCriteriaChips(rule).length > 0 && (
              <div className="rule-criteria">
                {ruleCriteriaChips(rule).map((chip) => (
                  <Tooltip key={chip.label} title={`${chip.label}: ${chip.value}`}>
                    <span className="criterion-chip">
                      <span className="criterion-chip-label">{chip.label}</span>
                      <span className="criterion-chip-value">{chipPreview(chip.value)}</span>
                    </span>
                  </Tooltip>
                ))}
              </div>
            )}
          </div>
        ))
      )}
    </div>
  );
}
