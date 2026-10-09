import { memo } from 'react';
import { useTranslation } from 'react-i18next';
import { Popover, Tag } from 'antd';

import { useDatepicker } from '@/hooks/useDatepicker';
import { IntlUtil, SizeFormatter } from '@/utils';
import './ClientTrafficCell.css';

export interface ClientSessionCellProps {
  up?: number;
  down?: number;
  start?: number;
  lastOnline?: number;
  ongoing?: boolean;
}

// A row the panel has seen no session for yet keeps start 0 and no bytes; it
// renders as a dash, so an untracked client never reads as an empty session.
export function hasClientSession(start = 0, up = 0, down = 0): boolean {
  return start > 0 || up + down > 0;
}

const ClientSessionCell = memo(function ClientSessionCell({
  up = 0,
  down = 0,
  start = 0,
  lastOnline = 0,
  ongoing = false,
}: ClientSessionCellProps) {
  const { t } = useTranslation();
  const { datepicker } = useDatepicker();

  if (!hasClientSession(start, up, down)) return <Tag>—</Tag>;

  const popover = (
    <table className="client-traffic-popover">
      <tbody>
        <tr>
          <td>↑</td>
          <td>{SizeFormatter.sizeFormat(up)}</td>
          <td>↓</td>
          <td>{SizeFormatter.sizeFormat(down)}</td>
        </tr>
        {start > 0 && (
          <tr>
            <td colSpan={2}>{t('pages.clients.sessionStarted')}</td>
            <td colSpan={2}>{IntlUtil.formatDate(start, datepicker)}</td>
          </tr>
        )}
        {!ongoing && lastOnline > 0 && (
          <tr>
            <td colSpan={2}>{t('pages.clients.sessionEnded')}</td>
            <td colSpan={2}>{IntlUtil.formatDate(lastOnline, datepicker)}</td>
          </tr>
        )}
      </tbody>
    </table>
  );

  return (
    <Popover content={popover} trigger={['hover', 'click']} placement="top">
      <Tag color={ongoing ? 'blue' : 'default'}>{SizeFormatter.sizeFormat(up + down)}</Tag>
    </Popover>
  );
});

export default ClientSessionCell;
