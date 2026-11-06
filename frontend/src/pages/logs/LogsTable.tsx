import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Empty, Table, Tag, message } from 'antd';
import type { TableColumnsType } from 'antd';
import { CopyOutlined, ScissorOutlined } from '@ant-design/icons';
import type { InfiniteData, UseInfiniteQueryResult } from '@tanstack/react-query';

import type { LogLine, LogPage } from '@/generated/types';
import { useDatepicker } from '@/hooks/useDatepicker';
import {
  LOG_TEXT_FIELDS,
  splitHighlights,
  type LogFilters,
  type LogTextField,
} from '@/lib/logs/logQuery';
import { ClipboardManager, IntlUtil, SizeFormatter } from '@/utils';

// Bounds what the browser holds: past this the filters are the way to reach older lines.
const MAX_LOADED_LINES = 5000;

const LEVEL_COLORS: Record<string, string> = {
  error: 'red',
  warning: 'gold',
  info: 'blue',
  debug: 'default',
};

const FIELD_COLUMNS: { field: LogTextField; width: number }[] = [
  { field: 'ip', width: 140 },
  { field: 'country', width: 80 },
  { field: 'user', width: 160 },
  { field: 'host', width: 180 },
  { field: 'protocol', width: 90 },
  { field: 'inbound', width: 130 },
  { field: 'status', width: 110 },
];

interface LogsTableProps {
  query: UseInfiniteQueryResult<InfiniteData<LogPage>>;
  filters: LogFilters;
  onToggleField: (field: LogTextField, value: string) => void;
}

function Highlight({ text, terms }: { text: string; terms: string[] }) {
  return (
    <bdi>
      {splitHighlights(text, terms).map((part, i) =>
        part.match ? <mark key={i}>{part.text}</mark> : part.text,
      )}
    </bdi>
  );
}

const FIELD_OF_LINE: Record<LogTextField, (line: LogLine) => string> = {
  ip: (line) => line.ip,
  country: (line) => line.country,
  user: (line) => line.user,
  host: (line) => line.host,
  inbound: (line) => line.inbound,
  protocol: (line) => line.protocol,
  status: (line) => line.status,
};

function LineDetails({
  line,
  onToggleField,
}: {
  line: LogLine;
  onToggleField: LogsTableProps['onToggleField'];
}) {
  const { t } = useTranslation();
  const [messageApi, contextHolder] = message.useMessage();
  const copy = async () => {
    if (await ClipboardManager.copyText(line.raw)) void messageApi.success(t('copySuccess'));
  };
  return (
    <div className="logs-details">
      {contextHolder}
      <div className="logs-details-fields">
        {LOG_TEXT_FIELDS.filter((field) => FIELD_OF_LINE[field](line)).map((field) => (
          <Button
            key={field}
            size="small"
            onClick={() => onToggleField(field, FIELD_OF_LINE[field](line))}
          >
            {t(`pages.logs.fields.${field}`)}: <bdi>{FIELD_OF_LINE[field](line)}</bdi>
          </Button>
        ))}
        {line.outbound && (
          <Tag>
            {t('pages.logs.outbound')}: <bdi>{line.outbound}</bdi>
          </Tag>
        )}
        {line.truncated && (
          <Tag icon={<ScissorOutlined aria-hidden />} color="orange">
            {t('pages.logs.truncated')}
          </Tag>
        )}
        <Button size="small" icon={<CopyOutlined aria-hidden />} onClick={() => void copy()}>
          {t('copy')}
        </Button>
      </div>
      <pre className="logs-raw" dir="ltr">
        {line.raw}
      </pre>
    </div>
  );
}

export default function LogsTable({ query, filters, onToggleField }: LogsTableProps) {
  const { t } = useTranslation();
  const { datepicker } = useDatepicker();

  const pages = useMemo(() => query.data?.pages ?? [], [query.data]);
  const lines = useMemo(() => pages.flatMap((page) => page.lines), [pages]);
  const lastPage = pages[pages.length - 1];

  const columns = useMemo<TableColumnsType<LogLine>>(() => {
    const highlighted = (field: LogTextField, width: number) => ({
      title: field === 'status' ? t('status') : t(`pages.logs.fields.${field}`),
      dataIndex: field,
      key: field,
      width,
      ellipsis: true,
      render: (value: string) => <Highlight text={value} terms={[filters[field], filters.query]} />,
    });
    return [
      {
        title: t('pages.logs.columns.time'),
        dataIndex: 'time',
        key: 'time',
        width: 180,
        render: (time: number) => (time ? IntlUtil.formatDate(time, datepicker) : '—'),
      },
      {
        title: t('pages.logs.level'),
        dataIndex: 'level',
        key: 'level',
        width: 100,
        render: (level: string) =>
          level ? <Tag color={LEVEL_COLORS[level]}>{t(`pages.logs.levels.${level}`)}</Tag> : '—',
      },
      ...FIELD_COLUMNS.filter(
        ({ field }) =>
          lines.length === 0 || lines.some((line) => FIELD_OF_LINE[field](line) !== ''),
      ).map(({ field, width }) => highlighted(field, width)),
      {
        title: t('pages.logs.columns.message'),
        dataIndex: 'message',
        key: 'message',
        ellipsis: true,
        render: (text: string) => <Highlight text={text} terms={[filters.status, filters.query]} />,
      },
    ];
  }, [t, datepicker, filters, lines]);

  if (query.isError) {
    return <Alert type="error" showIcon title={query.error.message} />;
  }

  const atLineCap = lines.length >= MAX_LOADED_LINES;

  return (
    <div className="logs-table">
      <Table<LogLine>
        size="small"
        rowKey="offset"
        tableLayout="fixed"
        columns={columns}
        dataSource={lines}
        loading={query.isPending}
        scroll={{ x: 1400, y: 600 }}
        pagination={{ defaultPageSize: 50, showSizeChanger: true, hideOnSinglePage: true }}
        expandable={{
          expandRowByClick: true,
          expandedRowRender: (line) => <LineDetails line={line} onToggleField={onToggleField} />,
        }}
        locale={{ emptyText: <Empty description={t('pages.logs.noMatches')} /> }}
      />
      {lastPage?.budgetExhausted && (
        <Alert
          type="info"
          showIcon
          title={t(query.hasNextPage ? 'pages.logs.readBudget' : 'pages.logs.scanStopped', {
            scanned: SizeFormatter.sizeFormat(lastPage.scannedBytes),
          })}
        />
      )}
      {atLineCap ? (
        <Alert
          type="info"
          showIcon
          title={t('pages.logs.lineCap', { count: lines.length.toLocaleString() })}
        />
      ) : (
        query.hasNextPage && (
          <div className="logs-load-more">
            <Button loading={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}>
              {t('pages.logs.loadOlder')}
            </Button>
          </div>
        )
      )}
    </div>
  );
}
