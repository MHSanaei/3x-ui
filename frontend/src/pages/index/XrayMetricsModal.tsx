import { useCallback, useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Modal, Select, Tabs, Tag } from 'antd';
import {
  BlockOutlined,
  CloudServerOutlined,
  DatabaseOutlined,
  DeleteOutlined,
  EyeOutlined,
  PauseCircleOutlined,
} from '@ant-design/icons';

import { HttpUtil, Msg, SizeFormatter } from '@/utils';
import { Sparkline } from '@/components/viz';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import './XrayMetricsModal.css';

const OBS_KEY = 'xrObs';

interface XrayMetricsModalProps {
  open: boolean;
  onClose: () => void;
}

interface MetricDef {
  key: string;
  tab: string;
  tabKey: string;
  title: string;
  icon: ReactNode;
  unit: 'B' | 'ns' | 'ms' | '';
  stroke: string;
}

interface XrayState {
  enabled: boolean;
  listen: string;
  reason: string;
}

interface ObservatoryTag {
  tag: string;
  alive: boolean;
  delay: number;
  lastSeenTime: number;
  lastTryTime: number;
}

const METRICS: MetricDef[] = [
  {
    key: 'xrAlloc',
    tab: 'Heap',
    tabKey: 'pages.index.xrayTabHeap',
    title: 'pages.index.xrayTitleHeap',
    icon: <DatabaseOutlined />,
    unit: 'B',
    stroke: '#7c4dff',
  },
  {
    key: 'xrSys',
    tab: 'Sys',
    tabKey: 'pages.index.xrayTabSys',
    title: 'pages.index.xrayTitleSys',
    icon: <CloudServerOutlined />,
    unit: 'B',
    stroke: '#1890ff',
  },
  {
    key: 'xrHeapObjects',
    tab: 'Objects',
    tabKey: 'pages.index.xrayTabObjects',
    title: 'pages.index.xrayTitleObjects',
    icon: <BlockOutlined />,
    unit: '',
    stroke: '#13c2c2',
  },
  {
    key: 'xrNumGC',
    tab: 'GC Count',
    tabKey: 'pages.index.xrayTabGcCount',
    title: 'pages.index.xrayTitleGcCount',
    icon: <DeleteOutlined />,
    unit: '',
    stroke: '#fa8c16',
  },
  {
    key: 'xrPauseNs',
    tab: 'GC Pause',
    tabKey: 'pages.index.xrayTabGcPause',
    title: 'pages.index.xrayTitleGcPause',
    icon: <PauseCircleOutlined />,
    unit: 'ns',
    stroke: '#f5222d',
  },
  {
    key: OBS_KEY,
    tab: 'Observatory',
    tabKey: 'pages.index.xrayTabObservatory',
    title: 'pages.index.xrayTitleObservatory',
    icon: <EyeOutlined />,
    unit: 'ms',
    stroke: '#52c41a',
  },
];

function unitFormatter(unit: string): (v: number) => string {
  if (unit === 'B') return (v) => SizeFormatter.sizeFormat(Math.max(0, Number(v) || 0));
  if (unit === 'ns') {
    return (v) => {
      const n = Math.max(0, Number(v) || 0);
      if (n >= 1e6) return `${(n / 1e6).toFixed(2)} ms`;
      if (n >= 1e3) return `${(n / 1e3).toFixed(1)} µs`;
      return `${n.toFixed(0)} ns`;
    };
  }
  if (unit === 'ms') return (v) => `${Math.round(Number(v) || 0)} ms`;
  return (v) => {
    const n = Number(v) || 0;
    return Math.round(n).toLocaleString();
  };
}

function fmtTimestamp(unixSec: number): string {
  if (!unixSec) return '—';
  const d = new Date(unixSec * 1000);
  const hh = String(d.getHours()).padStart(2, '0');
  const mm = String(d.getMinutes()).padStart(2, '0');
  const ss = String(d.getSeconds()).padStart(2, '0');
  return `${d.toLocaleDateString()} ${hh}:${mm}:${ss}`;
}

function formatFullTimestamp(unixSec: number): string {
  const d = new Date(unixSec * 1000);
  const today = new Date();
  const sameDay =
    d.getFullYear() === today.getFullYear() &&
    d.getMonth() === today.getMonth() &&
    d.getDate() === today.getDate();
  const hh = String(d.getHours()).padStart(2, '0');
  const mm = String(d.getMinutes()).padStart(2, '0');
  const ss = String(d.getSeconds()).padStart(2, '0');
  const time = `${hh}:${mm}:${ss}`;
  if (sameDay) return time;
  const MM = String(d.getMonth() + 1).padStart(2, '0');
  const DD = String(d.getDate()).padStart(2, '0');
  return `${MM}-${DD} ${time}`;
}

interface MetricsChart {
  points: number[];
  labels: string[];
  timestamps: number[];
}

const EMPTY_CHART: MetricsChart = { points: [], labels: [], timestamps: [] };

function toChart(msg: Msg<{ t: number; v: number }[]> | null | undefined, bucket: number) {
  if (!msg?.success || !Array.isArray(msg.obj)) return EMPTY_CHART;
  const points: number[] = [];
  const labels: string[] = [];
  const timestamps: number[] = [];
  for (const p of msg.obj) {
    const d = new Date(p.t * 1000);
    const hh = String(d.getHours()).padStart(2, '0');
    const mm = String(d.getMinutes()).padStart(2, '0');
    const ss = String(d.getSeconds()).padStart(2, '0');
    labels.push(bucket >= 60 ? `${hh}:${mm}` : `${hh}:${mm}:${ss}`);
    points.push(Number(p.v) || 0);
    timestamps.push(Number(p.t) || 0);
  }
  return { points, labels, timestamps };
}

async function loadHistory(url: string | null, bucket: number): Promise<MetricsChart> {
  if (!url) return EMPTY_CHART;
  try {
    return toChart(await HttpUtil.get<{ t: number; v: number }[]>(url), bucket);
  } catch (e) {
    console.error('Failed to fetch xray metrics bucket', e);
    return EMPTY_CHART;
  }
}

async function loadState(): Promise<XrayState | null> {
  try {
    const msg = await HttpUtil.get<XrayState>('/panel/api/server/xrayMetricsState');
    return msg?.success && msg.obj ? msg.obj : null;
  } catch (e) {
    console.error('Failed to fetch xray metrics state', e);
    return null;
  }
}

async function loadObservatory(): Promise<ObservatoryTag[]> {
  try {
    const msg = await HttpUtil.get<ObservatoryTag[]>('/panel/api/server/xrayObservatory');
    return msg?.success && Array.isArray(msg.obj) ? msg.obj : [];
  } catch (e) {
    console.error('Failed to fetch observatory snapshot', e);
    return [];
  }
}

export default function XrayMetricsModal({ open, onClose }: XrayMetricsModalProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();
  const [activeKey, setActiveKey] = useState('xrAlloc');
  const [bucket, setBucket] = useState(2);
  const [{ points, labels, timestamps }, setChart] = useState<MetricsChart>(EMPTY_CHART);
  const [state, setState] = useState<XrayState>({ enabled: false, listen: '', reason: '' });
  const [obsTags, setObsTags] = useState<ObservatoryTag[]>([]);
  const [obsActiveTag, setObsActiveTag] = useState('');
  const [obsTick, setObsTick] = useState(0);

  const activeMetric = useMemo(() => METRICS.find((m) => m.key === activeKey), [activeKey]);
  const isObservatory = activeKey === OBS_KEY;
  const strokeColor = activeMetric?.stroke || '#008771';
  const yFormatter = useMemo(() => unitFormatter(activeMetric?.unit ?? ''), [activeMetric]);

  const activeObsTag = obsTags.find((tg) => tg.tag === obsActiveTag) || null;

  const tsLookup = useMemo(() => {
    const m = new Map<string, number>();
    for (let i = 0; i < labels.length; i++) {
      m.set(labels[i], timestamps[i]);
    }
    return m;
  }, [labels, timestamps]);

  const tooltipLabelFormatter = useCallback(
    (label: string) => {
      const ts = tsLookup.get(label);
      return ts ? formatFullTimestamp(ts) : label;
    },
    [tsLookup],
  );

  const [wasOpen, setWasOpen] = useState(false);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) setActiveKey('xrAlloc');
  }

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    void (async () => {
      const next = await loadState();
      if (!cancelled && next) setState(next);
    })();
    return () => {
      cancelled = true;
    };
  }, [open]);

  // The observatory snapshot is a live view, so it re-polls; obsTick then pulls
  // the chart along with it.
  useEffect(() => {
    if (!open || !isObservatory) return;
    let cancelled = false;
    const tick = async () => {
      const tags = await loadObservatory();
      if (cancelled) return;
      setObsTags(tags);
      setObsActiveTag((prev) => (tags.find((tg) => tg.tag === prev) ? prev : tags[0]?.tag || ''));
      setObsTick((n) => n + 1);
    };
    void tick();
    const id = window.setInterval(() => void tick(), 2000);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [open, isObservatory]);

  const historyUrl = isObservatory
    ? obsActiveTag
      ? `/panel/api/server/xrayObservatoryHistory/${encodeURIComponent(obsActiveTag)}/${bucket}`
      : null
    : activeMetric
      ? `/panel/api/server/xrayMetricsHistory/${activeMetric.key}/${bucket}`
      : null;

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    void (async () => {
      const next = await loadHistory(historyUrl, bucket);
      if (!cancelled) setChart(next);
    })();
    return () => {
      cancelled = true;
    };
  }, [open, historyUrl, bucket, obsTick]);

  return (
    <Modal
      open={open}
      footer={null}
      width={isMobile ? '95vw' : 900}
      onCancel={onClose}
      title={
        <div className="metric-modal-title">
          <span>{t('pages.index.xrayMetricsTitle')}</span>
          <Select
            value={bucket}
            size="small"
            className="bucket-select"
            onChange={setBucket}
            options={[
              { value: 2, label: '2m' },
              { value: 60, label: '1h' },
              { value: 180, label: '3h' },
              { value: 360, label: '6h' },
              { value: 720, label: '12h' },
            ]}
          />
        </div>
      }
    >
      {!state.enabled && (
        <Alert
          type="warning"
          showIcon
          className="metrics-alert"
          title={t('pages.index.xrayMetricsDisabled')}
          description={state.reason || t('pages.index.xrayMetricsHint')}
        />
      )}

      <Tabs
        activeKey={activeKey}
        onChange={setActiveKey}
        size="small"
        className="history-tabs"
        items={METRICS.map((m) => {
          const tabLabel = m.tabKey ? t(m.tabKey) : m.tab;
          return {
            key: m.key,
            label: isMobile ? (
              <span title={tabLabel} aria-label={tabLabel}>
                {m.icon}
              </span>
            ) : (
              tabLabel
            ),
          };
        })}
      />

      {isObservatory && (
        <div className="obs-pane">
          {state.enabled && obsTags.length === 0 ? (
            <Alert
              type="info"
              showIcon
              className="metrics-alert"
              title={t('pages.index.xrayObservatoryEmpty')}
              description={t('pages.index.xrayObservatoryHint')}
            />
          ) : (
            <div className="obs-controls">
              <Select
                value={obsActiveTag}
                size="small"
                className="obs-select"
                placeholder={t('pages.index.xrayObservatoryTagPlaceholder')}
                onChange={setObsActiveTag}
                options={obsTags.map((tg) => ({
                  value: tg.tag,
                  label: (
                    <>
                      <span className={`obs-dot ${tg.alive ? 'is-alive' : 'is-dead'}`} />
                      {tg.tag}
                    </>
                  ),
                }))}
              />

              {activeObsTag && (
                <div className="obs-stats">
                  <Tag color={activeObsTag.alive ? 'green' : 'red'}>
                    {activeObsTag.alive
                      ? t('pages.index.xrayObservatoryAlive')
                      : t('pages.index.xrayObservatoryDead')}
                  </Tag>
                  <Tag color="blue">{activeObsTag.delay} ms</Tag>
                  <span className="obs-stamp">
                    {t('pages.index.xrayObservatoryLastSeen')}:{' '}
                    {fmtTimestamp(activeObsTag.lastSeenTime)}
                  </span>
                  <span className="obs-stamp">
                    {t('pages.index.xrayObservatoryLastTry')}:{' '}
                    {fmtTimestamp(activeObsTag.lastTryTime)}
                  </span>
                </div>
              )}
            </div>
          )}
        </div>
      )}

      <div className="cpu-chart-wrap">
        {activeMetric?.title && <div className="history-chart-title">{t(activeMetric.title)}</div>}
        <Sparkline
          data={points}
          labels={labels}
          height={260}
          stroke={strokeColor}
          strokeWidth={2.2}
          showGrid
          showAxes
          tickCountX={5}
          maxPoints={points.length || 1}
          fillOpacity={0.18}
          markerRadius={3.2}
          showTooltip
          valueMin={0}
          valueMax={null}
          yFormatter={yFormatter}
          tooltipLabelFormatter={tooltipLabelFormatter}
          extrema={{ show: true, formatter: yFormatter }}
        />
      </div>
    </Modal>
  );
}
