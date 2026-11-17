import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Card, theme } from 'antd';

import { Sparkline } from '@/components/viz';
import type { Status } from '@/models/status';

interface ConnectionsCardProps {
  status: Status;
  tcp: number[];
  udp: number[];
  labels: string[];
  isMobile: boolean;
}

export default function ConnectionsCard({
  status,
  tcp,
  udp,
  labels,
  isMobile,
}: ConnectionsCardProps) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const accent = token.colorPrimary;
  const udpColor = token.colorTextTertiary;

  const referenceLines = useMemo(
    () => [
      { y: status.udpCount, color: udpColor, dash: '2 4' },
      { y: status.tcpCount, color: accent, dash: '2 4' },
    ],
    [status.tcpCount, status.udpCount, accent, udpColor],
  );

  return (
    <Card hoverable styles={{ body: { padding: 0 } }}>
      <div className="ov-wide-head ov-wide-head-stack">
        <div className="ov-kicker">{t('pages.index.connectionCount')}</div>
        <div className="ov-conn-total">
          <span className="ov-tile-number">{status.tcpCount + status.udpCount}</span>
          <span className="ov-tile-unit">{t('pages.index.openSockets')}</span>
        </div>
      </div>

      <div className="ov-conn-legend">
        <div className="ov-legend-label">
          <span className="ov-swatch" style={{ background: accent }} />
          TCP
          <span className="ov-legend-num">{status.tcpCount.toLocaleString()}</span>
        </div>
        <div className="ov-legend-label">
          <span className="ov-swatch" style={{ background: udpColor }} />
          UDP
          <span className="ov-legend-num">{status.udpCount.toLocaleString()}</span>
        </div>
      </div>

      <div className="ov-wide-chart">
        <Sparkline
          data={tcp}
          data2={udp}
          labels={labels}
          height={isMobile ? 120 : 170}
          strokeWidth={1.5}
          fillOpacity={0.24}
          showTooltip
          showLegend={false}
          valueMax={null}
          stroke={accent}
          stroke2={udpColor}
          name1="TCP"
          name2="UDP"
          yFormatter={(v) => Math.round(v).toLocaleString()}
          referenceLines={referenceLines}
        />
      </div>
    </Card>
  );
}
