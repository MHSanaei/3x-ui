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
import type { HostRecord } from '@/schemas/api/host';

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

const CHIP_STYLE = { margin: 2 } as const;
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
  protocolColors: Record<string, string>;
  chipLimit: number;
}

// Attachments never change on a traffic push either, so the same memoisation
// applies: one Tooltip per visible chip plus a Popover for the overflow.
export const ClientInboundChips = memo(function ClientInboundChips({
  ids,
  inboundsById,
  protocolColors,
  chipLimit,
}: ClientInboundChipsProps) {
  if (ids.length === 0) return <span className="cell-empty">—</span>;

  const label = (id: number) => {
    const ib = inboundsById[id];
    return formatInboundLabel(ib?.tag, ib?.remark);
  };
  const chip = (id: number) => {
    const proto = (inboundsById[id]?.protocol || '').toLowerCase();
    return (
      <Tooltip key={id} title={label(id)}>
        <Tag color={protocolColors[proto] ?? 'default'} style={CHIP_STYLE}>
          {label(id)}
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

interface ClientHostChipsProps {
  groupIds: string[];
  hostsByGroupId: Record<string, HostRecord>;
  inboundsById: Record<number, InboundOption>;
  chipLimit: number;
}

const HOST_OVERFLOW_LIST_STYLE = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: 4,
  maxWidth: 360,
  maxHeight: 320,
  overflowY: 'auto' as const,
};

export const ClientHostChips = memo(function ClientHostChips({
  groupIds,
  hostsByGroupId,
  inboundsById,
  chipLimit,
}: ClientHostChipsProps) {
  if (groupIds.length === 0) return <span className="cell-empty">—</span>;

  const renderChip = (groupId: string) => {
    const host = hostsByGroupId[groupId];
    const addresses = host?.hosts?.filter(Boolean) ?? [];
    const inboundNames = (host?.inboundIds ?? [])
      .map((id) => formatInboundLabel(inboundsById[id]?.tag, inboundsById[id]?.remark))
      .filter(Boolean);
    const label = addresses.length > 0 ? addresses.join(', ') : groupId;
    const title = (
      <div style={{ maxWidth: 360 }}>
        <div style={{ fontWeight: 500 }}>{host?.remark?.trim() || groupId}</div>
        <div style={{ marginTop: 4 }}>{addresses.length > 0 ? addresses.join(', ') : '—'}</div>
        {inboundNames.length > 0 && (
          <div style={{ marginTop: 4, opacity: 0.7 }}>{inboundNames.join(', ')}</div>
        )}
      </div>
    );

    return (
      <Tooltip key={groupId} title={title}>
        <Tag
          color={host ? 'purple' : 'warning'}
          style={{
            margin: 2,
            maxWidth: 220,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            verticalAlign: 'bottom',
          }}
        >
          {host ? label : '⚠ ' + groupId}
        </Tag>
      </Tooltip>
    );
  };

  const visible = groupIds.slice(0, chipLimit);
  const overflow = groupIds.slice(chipLimit);

  return (
    <>
      {visible.map(renderChip)}
      {overflow.length > 0 && (
        <Popover
          trigger="click"
          placement="bottomRight"
          content={<div style={HOST_OVERFLOW_LIST_STYLE}>{overflow.map(renderChip)}</div>}
        >
          <Tag color="default" style={OVERFLOW_CHIP_STYLE}>
            +{overflow.length}
          </Tag>
        </Popover>
      )}
    </>
  );
});
