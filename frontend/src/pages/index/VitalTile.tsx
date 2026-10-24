import { useMemo } from 'react';
import type { ReactNode } from 'react';
import { Card, theme } from 'antd';

import { Sparkline } from '@/components/viz';
import { mean, peak } from './useOverviewHistory';

interface VitalTileProps {
  available?: boolean;
  icon: ReactNode;
  label: string;
  percent: number;
  statusColor: string;
  detail: string;
  footLeft: string;
  footRight: string;
  data: number[];
  isMobile: boolean;
}

export default function VitalTile({
  available = true,
  icon,
  label,
  percent,
  statusColor,
  detail,
  footLeft,
  footRight,
  data,
  isMobile,
}: VitalTileProps) {
  const { token } = theme.useToken();
  const meanColor = token.colorTextTertiary;

  const referenceLines = useMemo(
    () => (data.length > 1 ? [{ y: mean(data), dash: '3 4', color: meanColor }] : []),
    [data, meanColor],
  );

  return (
    <Card hoverable className="ov-tile" styles={{ body: { padding: 0 } }}>
      <div className="ov-tile-head">
        <span className="ov-tile-icon">{icon}</span>
        <span className="ov-kicker">{label}</span>
      </div>

      <div className="ov-tile-value">
        <span className="ov-tile-number">{available ? percent.toFixed(1) : '—'}</span>
        {available && <span className="ov-tile-unit">%</span>}
      </div>

      <div className="ov-tile-detail">{available ? detail : '—'}</div>

      <div className="ov-tile-foot">
        <span>{available ? footLeft : '—'}</span>
        <span>{available ? footRight : '—'}</span>
      </div>

      <div className="ov-tile-chart">
        <Sparkline
          data={data}
          height={isMobile ? 48 : 62}
          strokeWidth={1.5}
          fillOpacity={0.3}
          showGrid={false}
          showMarker={false}
          valueMax={peak(data) > 0 ? null : 100}
          stroke={statusColor}
          referenceLines={referenceLines}
          yFormatter={(v) => `${v.toFixed(0)}%`}
          name1={label}
        />
      </div>
    </Card>
  );
}
