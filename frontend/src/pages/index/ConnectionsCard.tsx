import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Card, theme } from 'antd';

import { Sparkline } from '@/components/viz';
import type { Status } from '@/models/status';

interface ConnectionsCardProps {
  status: Status;
  tcp: number[];
  udp: number[];
  conntrack: number[];
  labels: string[];
  isMobile: boolean;
}

export default function ConnectionsCard({
  status,
  tcp,
  udp,
  conntrack,
  labels,
  isMobile,
}: ConnectionsCardProps) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const accent = token.colorPrimary;
  const udpColor = token.colorTextTertiary;
  const conntrackColor = token.colorWarning;
  const hasConntrack = status.conntrackMax > 0;

  const referenceLines = useMemo(
    () => [
      { y: status.udpCount, color: udpColor, dash: '2 4' },
      { y: status.tcpCount, color: accent, dash: '2 4' },
      ...(hasConntrack
        ? [
            {
              y: status.conntrackMax,
              color: token.colorError,
              dash: '5 4',
              label: `${t('pages.index.conntrackLimit')} ${status.conntrackMax.toLocaleString()}`,
            },
          ]
        : []),
    ],
    [
      status.tcpCount,
      status.udpCount,
      status.conntrackMax,
      hasConntrack,
      accent,
      udpColor,
      token.colorError,
      t,
    ],
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
        {hasConntrack && (
          <div className="ov-legend-label">
            <span className="ov-swatch" style={{ background: conntrackColor }} />
            {t('pages.index.conntrackUsage')}
            <span className="ov-legend-num">
              {status.conntrackCount.toLocaleString()} / {status.conntrackMax.toLocaleString()}
            </span>
          </div>
        )}
      </div>

      <div className="ov-wide-chart">
        <Sparkline
          data={tcp}
          data2={udp}
          data3={hasConntrack ? conntrack : undefined}
          labels={labels}
          height={isMobile ? 120 : 170}
          strokeWidth={1.5}
          fillOpacity={0.24}
          showTooltip
          showLegend={false}
          valueMax={
            hasConntrack ? Math.max(status.conntrackMax, ...tcp, ...udp, ...conntrack) * 1.1 : null
          }
          stroke={accent}
          stroke2={udpColor}
          stroke3={conntrackColor}
          name1="TCP"
          name2="UDP"
          name3={t('pages.index.conntrackUsage')}
          yFormatter={(v) => Math.round(v).toLocaleString()}
          referenceLines={referenceLines}
        />
      </div>
    </Card>
  );
}
