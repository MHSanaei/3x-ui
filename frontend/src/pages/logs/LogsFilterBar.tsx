import { useTranslation } from 'react-i18next';
import { Button, Card, Col, DatePicker, Input, Row, Segmented, Select, Space } from 'antd';
import { ClearOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';

import type { LogFile } from '@/generated/types';
import {
  EMPTY_LOG_FILTERS,
  hasActiveLogFilters,
  LOG_LEVELS,
  LOG_RANGE_PRESETS,
  LOG_TEXT_FIELDS,
  type LogFilters,
  type LogLevelFilter,
  type LogRangePreset,
  type LogTextField,
} from '@/lib/logs/logQuery';
import { SizeFormatter } from '@/utils';

const AUTO_REFRESH_SECONDS = [5, 15, 60];

// Short values get narrow inputs, so all the fields fit on one row beside the search box.
const FIELD_SPANS: Record<LogTextField, number> = {
  ip: 3,
  country: 2,
  user: 3,
  host: 3,
  inbound: 3,
  protocol: 2,
  status: 3,
};

interface LogsFilterBarProps {
  files: LogFile[];
  file: string | null;
  onFileChange: (name: string) => void;
  draft: LogFilters;
  onDraftChange: (next: LogFilters) => void;
  onApply: (next: LogFilters) => void;
  autoRefresh: number | false;
  onAutoRefreshChange: (next: number | false) => void;
  refreshing: boolean;
  onRefresh: () => void;
}

export default function LogsFilterBar({
  files,
  file,
  onFileChange,
  draft,
  onDraftChange,
  onApply,
  autoRefresh,
  onAutoRefreshChange,
  refreshing,
  onRefresh,
}: LogsFilterBarProps) {
  const { t } = useTranslation();

  return (
    <Card size="small" className="logs-filter-bar">
      <Row gutter={[12, 12]} align="middle">
        <Col xs={24} md={12} xl={7}>
          <Select
            aria-label={t('pages.logs.file')}
            className="logs-control"
            showSearch={{ optionFilterProp: 'value' }}
            value={file}
            onChange={onFileChange}
            options={files.map((f) => ({
              value: f.name,
              label: `${f.name} (${SizeFormatter.sizeFormat(f.size)})`,
            }))}
          />
        </Col>
        <Col xs={24} md={12} xl={5}>
          <Select<LogRangePreset>
            aria-label={t('pages.logs.timeRange')}
            className="logs-control"
            value={draft.range}
            onChange={(range) => onApply({ ...draft, range })}
            options={LOG_RANGE_PRESETS.map((preset) => ({
              value: preset,
              label: t(`pages.logs.ranges.${preset}`),
            }))}
          />
        </Col>
        <Col xs={24} xl={7} className="logs-levels">
          <Segmented<LogLevelFilter>
            aria-label={t('pages.logs.level')}
            value={draft.level}
            onChange={(level) => onApply({ ...draft, level })}
            options={LOG_LEVELS.map((level) => ({
              value: level,
              label: t(`pages.logs.levels.${level}`),
            }))}
          />
        </Col>
        <Col xs={24} xl={5}>
          <Space.Compact className="logs-control">
            <Select<number>
              aria-label={t('pages.logs.autoRefresh')}
              className="logs-control"
              value={autoRefresh === false ? 0 : autoRefresh}
              onChange={(ms) => onAutoRefreshChange(ms === 0 ? false : ms)}
              options={[
                { value: 0, label: t('pages.logs.autoRefreshOff') },
                ...AUTO_REFRESH_SECONDS.map((seconds) => ({
                  value: seconds * 1000,
                  label: t('pages.logs.autoRefreshEvery', { seconds }),
                })),
              ]}
            />
            <Button
              aria-label={t('refresh')}
              icon={<ReloadOutlined spin={refreshing} aria-hidden />}
              onClick={onRefresh}
            />
            <Button
              aria-label={t('reset')}
              title={t('reset')}
              icon={<ClearOutlined aria-hidden />}
              disabled={!hasActiveLogFilters(draft)}
              onClick={() => onApply(EMPTY_LOG_FILTERS)}
            />
          </Space.Compact>
        </Col>
        {draft.range === 'custom' && (
          <Col xs={24}>
            <DatePicker.RangePicker
              aria-label={t('pages.logs.ranges.custom')}
              className="logs-control"
              showTime
              allowEmpty={[true, true]}
              value={[
                draft.since ? dayjs(draft.since) : null,
                draft.until ? dayjs(draft.until) : null,
              ]}
              onChange={(range) =>
                onApply({
                  ...draft,
                  since: range?.[0]?.valueOf(),
                  until: range?.[1]?.valueOf(),
                })
              }
            />
          </Col>
        )}
        <Col xs={24} md={12} xl={5}>
          <Input
            allowClear
            prefix={<SearchOutlined aria-hidden />}
            aria-label={t('pages.logs.fields.query')}
            placeholder={t('pages.logs.fields.query')}
            value={draft.query}
            onChange={(e) => onDraftChange({ ...draft, query: e.target.value })}
          />
        </Col>
        {LOG_TEXT_FIELDS.map((field) => (
          <Col key={field} xs={12} md={6} xl={FIELD_SPANS[field]}>
            <Input
              allowClear
              aria-label={t(`pages.logs.fields.${field}`)}
              placeholder={t(`pages.logs.fields.${field}`)}
              value={draft[field]}
              onChange={(e) => onDraftChange({ ...draft, [field]: e.target.value })}
            />
          </Col>
        ))}
      </Row>
    </Card>
  );
}
