import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Popover, Progress } from 'antd';

import InfinityIcon from '@/components/ui/InfinityIcon';
import { useTheme } from '@/hooks/useTheme';
import { computeTrafficDisplay } from '@/lib/clients/traffic-display';
import { SizeFormatter } from '@/utils';
import './ClientTrafficCell.css';

export interface ClientTrafficCellProps {
  up?: number;
  down?: number;
  total?: number;
  enabled?: boolean;
  trafficDiff?: number;
  compact?: boolean;
}

export default function ClientTrafficCell({
  up = 0,
  down = 0,
  total = 0,
  enabled = true,
  trafficDiff = 0,
  compact = false,
}: ClientTrafficCellProps) {
  const { t } = useTranslation();
  const { isDark } = useTheme();

  const display = useMemo(
    () => computeTrafficDisplay({ up, down, total, enabled, trafficDiff }, isDark),
    [up, down, total, enabled, trafficDiff, isDark],
  );

  const popover = (
    <table className="client-traffic-popover">
      <tbody>
        <tr>
          <td>↑</td>
          <td>{SizeFormatter.sizeFormat(up)}</td>
          <td>↓</td>
          <td>{SizeFormatter.sizeFormat(down)}</td>
        </tr>
        {!display.isUnlimited && (
          <tr>
            <td colSpan={2}>{t('remained')}</td>
            <td colSpan={2}>{SizeFormatter.sizeFormat(display.remaining)}</td>
          </tr>
        )}
      </tbody>
    </table>
  );

  const rootClass = [
    'client-traffic-cell',
    compact ? 'is-compact' : '',
    display.isUnlimited ? 'is-unlimited' : '',
  ].filter(Boolean).join(' ');

  return (
    <Popover content={popover} trigger={['hover', 'click']} placement="top">
      <div className={rootClass}>
        <span className="client-traffic-cell-used">{SizeFormatter.sizeFormat(display.used)}</span>
        <Progress
          className="client-traffic-cell-bar"
          aria-label={`${SizeFormatter.sizeFormat(display.used)} / ${display.isUnlimited ? t('subscription.unlimited') : SizeFormatter.sizeFormat(total)}`}
          percent={display.percent}
          showInfo={false}
          strokeColor={display.strokeColor}
          status={display.status}
          size={compact ? 'small' : 'medium'}
        />
        <span className="client-traffic-cell-limit">
          {display.isUnlimited ? (
            <span className="client-traffic-cell-infinity" role="img" aria-label={t('subscription.unlimited')}>
              <InfinityIcon />
            </span>
          ) : (
            SizeFormatter.sizeFormat(total)
          )}
        </span>
      </div>
    </Popover>
  );
}
