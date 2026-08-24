import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Card, theme } from 'antd';
import { ArrowDownOutlined, ArrowUpOutlined } from '@ant-design/icons';

import { SizeFormatter } from '@/utils';
import { Sparkline } from '@/components/viz';
import type { Status } from '@/models/status';
import { mean, peak } from './useOverviewHistory';

interface ThroughputCardProps {
  status: Status;
  up: number[];
  down: number[];
  labels: string[];
  isMobile: boolean;
}

export default function ThroughputCard({
  status,
  up,
  down,
  labels,
  isMobile,
}: ThroughputCardProps) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const accent = token.colorPrimary;
  const downColor = token.colorTextTertiary;

  const referenceLines = useMemo(
    () => [
      { y: status.netIO.down, color: downColor, dash: '2 4' },
      { y: status.netIO.up, color: accent, dash: '2 4' },
    ],
    [status.netIO.up, status.netIO.down, accent, downColor],
  );

  return (
    <Card hoverable styles={{ body: { padding: 0 } }}>
      <div className="ov-wide-head">
        <div>
          <div className="ov-kicker">{t('pages.index.overallSpeed')}</div>
          <div className="ov-sub">
            {`${t('pages.index.throughputSub')} · ${t('pages.index.peak')} ${SizeFormatter.speedFormat(peak(down))}`}
          </div>
        </div>
        <div className="ov-wide-legend">
          <div className="ov-legend-label">
            <ArrowUpOutlined style={{ color: accent }} />
            {t('pages.index.upload')}
            <span className="ov-legend-num">{SizeFormatter.speedFormat(status.netIO.up)}</span>
          </div>
          <div className="ov-legend-label">
            <ArrowDownOutlined style={{ color: downColor }} />
            {t('pages.index.download')}
            <span className="ov-legend-num">{SizeFormatter.speedFormat(status.netIO.down)}</span>
          </div>
        </div>
      </div>

      <div className="ov-wide-chart">
        <Sparkline
          data={up}
          data2={down}
          labels={labels}
          height={isMobile ? 140 : 186}
          strokeWidth={1.75}
          fillOpacity={0.24}
          showTooltip
          showLegend={false}
          valueMax={null}
          stroke={accent}
          stroke2={downColor}
          name1={t('pages.index.upload')}
          name2={t('pages.index.download')}
          yFormatter={SizeFormatter.speedFormat}
          referenceLines={referenceLines}
        />
      </div>

      <div className="ov-wide-foot">
        <div>
          <div className="ov-kicker">{t('pages.index.sent')}</div>
          <div className="ov-foot-value">{SizeFormatter.sizeFormat(status.netTraffic.sent)}</div>
        </div>
        <span className="ov-foot-sep" />
        <div>
          <div className="ov-kicker">{t('pages.index.received')}</div>
          <div className="ov-foot-value">{SizeFormatter.sizeFormat(status.netTraffic.recv)}</div>
        </div>
        <span className="ov-foot-sep" />
        <div>
          <div className="ov-kicker">{t('pages.index.avgWindow')}</div>
          <div className="ov-foot-value">
            <span className="ov-foot-part">{`↑ ${SizeFormatter.speedFormat(mean(up))}`}</span>{' '}
            <span className="ov-foot-part">{`↓ ${SizeFormatter.speedFormat(mean(down))}`}</span>
          </div>
        </div>
      </div>
    </Card>
  );
}
