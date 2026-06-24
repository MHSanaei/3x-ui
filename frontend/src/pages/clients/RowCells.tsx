import { memo } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Popover, Space, Tag, Tooltip } from 'antd';
import {
  DeleteOutlined,
  EditOutlined,
  InfoCircleOutlined,
  QrcodeOutlined,
  RetweetOutlined,
} from '@ant-design/icons';

import { formatInboundLabel } from '@/lib/inbounds/label';
import type { InboundOption } from '@/hooks/useClients';
import type { NodeRecord } from '@/schemas/node';

const ICON_BUTTON_STYLE = { fontSize: 16 } as const;

interface ClientRowActionsProps {
  email: string;
  onShowQr: (email: string) => void;
  onShowInfo: (email: string) => void;
  onResetTraffic: (email: string) => void;
  onEdit: (email: string) => void;
  onDelete: (email: string) => void;
}

// Five Tooltip-wrapped buttons per row, none of which depend on traffic. Left
// inline they re-ran rc-tooltip's alignment machinery for every visible row on
// every traffic push — 125 Tooltips on a 25-row page, five seconds apart.
// Keyed on the email rather than the row object, because a push replaces the row
// object of every client whose counters moved; the page resolves the live row.
export const ClientRowActions = memo(function ClientRowActions({
  email,
  onShowQr,
  onShowInfo,
  onResetTraffic,
  onEdit,
  onDelete,
}: ClientRowActionsProps) {
  const { t } = useTranslation();
  return (
    <Space size={4}>
      <Tooltip title={t('pages.clients.qrCode')}>
        <Button
          size="small"
          type="text"
          style={ICON_BUTTON_STYLE}
          icon={<QrcodeOutlined />}
          aria-label={t('pages.clients.qrCode')}
          onClick={() => onShowQr(email)}
        />
      </Tooltip>
      <Tooltip title={t('pages.clients.clientInfo')}>
        <Button
          size="small"
          type="text"
          style={ICON_BUTTON_STYLE}
          icon={<InfoCircleOutlined />}
          aria-label={t('pages.clients.clientInfo')}
          onClick={() => onShowInfo(email)}
        />
      </Tooltip>
      <Tooltip title={t('pages.inbounds.resetTraffic')}>
        <Button
          size="small"
          type="text"
          style={ICON_BUTTON_STYLE}
          icon={<RetweetOutlined />}
          aria-label={t('pages.inbounds.resetTraffic')}
          onClick={() => onResetTraffic(email)}
        />
      </Tooltip>
      <Tooltip title={t('edit')}>
        <Button
          size="small"
          type="text"
          style={ICON_BUTTON_STYLE}
          icon={<EditOutlined />}
          aria-label={t('edit')}
          onClick={() => onEdit(email)}
        />
      </Tooltip>
      <Tooltip title={t('delete')}>
        <Button
          size="small"
          type="text"
          danger
          style={ICON_BUTTON_STYLE}
          icon={<DeleteOutlined />}
          aria-label={t('delete')}
          onClick={() => onDelete(email)}
        />
      </Tooltip>
    </Space>
  );
});

const OVERFLOW_CHIP_STYLE = { margin: 2, cursor: 'pointer' } as const;
const OVERFLOW_LIST_STYLE = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: 4,
  maxWidth: 280,
  maxHeight: 280,
  overflowY: 'auto' as const,
};

interface ClientInboundChipsProps {
  ids: number[];
  inboundsById: Record<number, InboundOption>;
  nodesById?: ReadonlyMap<number, NodeRecord>;
  localNodeLabel?: string;
  protocolColors: Record<string, string>;
  chipLimit: number;
}

// Attachments never change on a traffic push either, so the same memoisation
// applies: one Tooltip per visible chip plus a Popover for the overflow.
export const ClientInboundChips = memo(function ClientInboundChips({
  ids,
  inboundsById,
  nodesById,
  localNodeLabel,
  protocolColors,
  chipLimit,
}: ClientInboundChipsProps) {
  if (ids.length === 0) return <span className="cell-empty">—</span>;

  const inboundLabel = (id: number) => {
    const ib = inboundsById[id];
    return formatInboundLabel(ib?.tag, ib?.remark, ib?.port);
  };
  const nodeLabel = (id: number) => {
    const ib = inboundsById[id];
    if (!ib || !nodesById) return '';
    if (ib.nodeId == null) return localNodeLabel || '';
    const node = nodesById.get(ib.nodeId);
    return (node?.remark || node?.name || node?.address || `#${ib.nodeId}`).trim();
  };
  const chip = (id: number) => {
    const ib = inboundsById[id];
    const proto = (ib?.protocol || '').toLowerCase();
    const inbound = inboundLabel(id);
    const node = nodeLabel(id);
    const fullLabel = node && inbound ? `${node} / ${inbound}` : node || inbound;
    return (
      <Tooltip key={id} title={fullLabel}>
        <Tag color={protocolColors[proto] ?? 'default'} className="client-inbound-chip">
          {node && <span className="client-inbound-node">{node}</span>}
          <span className="client-inbound-name">{inbound}</span>
        </Tag>
      </Tooltip>
    );
  };

  const visible = ids.slice(0, chipLimit);
  const overflow = ids.slice(chipLimit);
  return (
    <>
      {visible.map(chip)}
      {overflow.length > 0 && (
        <Popover
          trigger="click"
          placement="bottomRight"
          content={<div style={OVERFLOW_LIST_STYLE}>{overflow.map(chip)}</div>}
        >
          <Tag color="default" style={OVERFLOW_CHIP_STYLE}>
            +{overflow.length}
          </Tag>
        </Popover>
      )}
    </>
  );
});
