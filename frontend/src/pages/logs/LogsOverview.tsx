import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Card, Col, Empty, Row, Spin, Statistic, Typography, theme } from 'antd';
import type { UseQueryResult } from '@tanstack/react-query';

import { Sparkline } from '@/components/viz';
import type { LogCount, LogStats } from '@/generated/types';
import { useDatepicker } from '@/hooks/useDatepicker';
import { countryName } from '@/lib/logs/countryScale';
import { formatBucketLabel, type LogFilters, type LogTextField } from '@/lib/logs/logQuery';
import { IntlUtil, LanguageManager, SizeFormatter } from '@/utils';
import TopList from './TopList';
import WorldMap from './WorldMap';

const CHART_HEIGHT = 200;
const UNCATEGORIZED = 'other';
const COUNTRY_LIST_SIZE = 10;
const CHART_RIGHT_PADDING = 40;

const compactNumber = new Intl.NumberFormat(undefined, {
  notation: 'compact',
  maximumFractionDigits: 1,
});
const formatCount = (value: number) => Math.round(value).toLocaleString();

interface LogsOverviewProps {
  query: UseQueryResult<LogStats>;
  filters: LogFilters;
  onToggleField: (field: LogTextField, value: string) => void;
  onSelectTimeSpan: (since: number, until: number) => void;
}

export default function LogsOverview({
  query,
  filters,
  onToggleField,
  onSelectTimeSpan,
}: LogsOverviewProps) {
  const { t } = useTranslation();
  const { datepicker } = useDatepicker();
  const { token } = theme.useToken();
  const stats = query.data;

  const series = useMemo(() => {
    const timeline = stats?.timeline ?? [];
    const step = stats?.bucketMillis ?? 0;
    const span = timeline.length ? timeline[timeline.length - 1].start - timeline[0].start : 0;
    const locale = datepicker === 'jalalian' ? 'fa-IR' : LanguageManager.getLanguage();
    return {
      labels: timeline.map((b) => formatBucketLabel(b.start, step, span, locale)),
      totals: timeline.map((b) => b.total),
      errors: timeline.map((b) => b.errors),
      warnings: timeline.map((b) => b.warnings),
    };
  }, [stats, datepicker]);

  if (query.isError) {
    return <Alert type="error" showIcon title={query.error.message} />;
  }
  if (!stats) {
    return <Spin className="logs-spin" description={t('loading')} />;
  }
  if (stats.matchedLines === 0) {
    return <Empty description={t('pages.logs.noMatches')} />;
  }

  const selectBucket = (index: number) => {
    const bucket = stats.timeline[index];
    if (bucket) onSelectTimeSpan(bucket.start, bucket.start + stats.bucketMillis - 1);
  };

  const uniqueSuffix = stats.uniqueCapped ? '+' : undefined;
  const lists: {
    key: string;
    items: LogCount[];
    field: LogTextField;
    labelFor?: (value: string) => string | undefined;
  }[] = [
    { key: 'topIps', items: stats.topIps, field: 'ip' },
    { key: 'topUsers', items: stats.topUsers, field: 'user' },
    { key: 'topHosts', items: stats.topHosts, field: 'host' },
    {
      key: 'topErrorCategories',
      items: stats.topErrorCategories,
      field: 'status',
      labelFor: (value) => (value === UNCATEGORIZED ? t('pages.logs.uncategorized') : undefined),
    },
    { key: 'topProtocols', items: stats.topProtocols, field: 'protocol' },
    { key: 'topInbounds', items: stats.topInbounds, field: 'inbound' },
    { key: 'topStatuses', items: stats.topStatuses, field: 'status' },
  ];

  const formatTime = (millis: number) =>
    millis ? IntlUtil.formatDate(millis, datepicker) : t('pages.logs.noTimestamps');

  return (
    <div className="logs-overview">
      {stats.budgetExhausted && (
        <Alert
          type="warning"
          showIcon
          title={t('pages.logs.scanStopped', {
            scanned: SizeFormatter.sizeFormat(stats.scannedBytes),
          })}
        />
      )}
      <Typography.Text type="secondary">{t('pages.logs.clickHint')}</Typography.Text>
      <Row gutter={[12, 12]} align="stretch">
        {[
          { key: 'lines', value: stats.matchedLines },
          { key: 'errors', value: stats.errors },
          { key: 'warnings', value: stats.warnings },
          { key: 'uniqueIps', value: stats.uniqueIps, suffix: uniqueSuffix },
          { key: 'uniqueUsers', value: stats.uniqueUsers, suffix: uniqueSuffix },
        ].map((card) => (
          <Col key={card.key} xs={12} md={8} xl={4}>
            <Card size="small" className="logs-stat-card">
              <Statistic
                title={t(`pages.logs.cards.${card.key}`)}
                value={card.value}
                suffix={card.suffix}
              />
            </Card>
          </Col>
        ))}
        <Col xs={12} md={8} xl={4}>
          <Card size="small" className="logs-stat-card">
            <Statistic
              title={t('pages.logs.cards.range')}
              valueRender={() => (
                <div className="logs-range-value">
                  <div>{formatTime(stats.firstTime)}</div>
                  <div>{formatTime(stats.lastTime)}</div>
                </div>
              )}
            />
          </Card>
        </Col>
      </Row>

      {stats.timeline.length > 0 && (
        <Row gutter={[12, 12]}>
          <Col xs={24} xl={12}>
            <Card size="small" title={t('pages.logs.charts.volume')}>
              <Sparkline
                data={series.totals}
                labels={series.labels}
                name1={t('pages.logs.cards.lines')}
                height={CHART_HEIGHT}
                rightPadding={CHART_RIGHT_PADDING}
                maxPoints={stats.timeline.length}
                valueMax={null}
                showAxes
                showTooltip
                yFormatter={(v) => compactNumber.format(v)}
                tooltipFormatter={formatCount}
                onPointClick={selectBucket}
              />
            </Card>
          </Col>
          <Col xs={24} xl={12}>
            <Card size="small" title={t('pages.logs.charts.problems')}>
              <Sparkline
                data={series.errors}
                data2={series.warnings}
                labels={series.labels}
                name1={t('pages.logs.cards.errors')}
                name2={t('pages.logs.cards.warnings')}
                stroke={token.colorError}
                stroke2={token.colorWarning}
                height={CHART_HEIGHT}
                rightPadding={CHART_RIGHT_PADDING}
                maxPoints={stats.timeline.length}
                valueMax={null}
                showAxes
                showTooltip
                yFormatter={(v) => compactNumber.format(v)}
                tooltipFormatter={formatCount}
                onPointClick={selectBucket}
              />
            </Card>
          </Col>
        </Row>
      )}

      {!stats.geoAvailable && stats.uniqueIps > 0 && (
        <Alert type="info" showIcon title={t('pages.logs.geoMissing')} />
      )}
      {stats.geoAvailable && stats.countries.length > 0 && (
        <Card size="small" title={t('pages.logs.charts.countries')}>
          <Row gutter={[16, 12]}>
            <Col xs={24} xl={16}>
              <WorldMap
                countries={stats.countries}
                activeCode={filters.country}
                onSelect={(code) => onToggleField('country', code)}
                label={t('pages.logs.charts.countries')}
              />
            </Col>
            <Col xs={24} xl={8}>
              <TopList
                items={stats.countries.slice(0, COUNTRY_LIST_SIZE)}
                activeValue={filters.country}
                labelFor={(code) => countryName(code, LanguageManager.getLanguage())}
                selectable={() => true}
                onSelect={(code) => onToggleField('country', code)}
              />
            </Col>
          </Row>
          <Typography.Paragraph type="secondary" className="logs-map-note">
            {t('pages.logs.geoNote')}
            {stats.unlocatedLines > 0 && (
              <> {t('pages.logs.geoUnlocated', { count: stats.unlocatedLines.toLocaleString() })}</>
            )}
          </Typography.Paragraph>
        </Card>
      )}

      <Row gutter={[12, 12]}>
        {lists
          .filter((list) => list.items.length > 0)
          .map((list) => (
            <Col key={list.key} xs={24} md={12} xl={8}>
              <Card size="small" title={t(`pages.logs.charts.${list.key}`)}>
                <TopList
                  items={list.items}
                  activeValue={filters[list.field]}
                  labelFor={list.labelFor}
                  selectable={(value) => !list.labelFor?.(value)}
                  onSelect={(value) => onToggleField(list.field, value)}
                />
              </Card>
            </Col>
          ))}
      </Row>
    </div>
  );
}
