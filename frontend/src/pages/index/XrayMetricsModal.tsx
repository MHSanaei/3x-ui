import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Modal, Select, Tabs, Tag } from 'antd';

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
  { key: 'xrAlloc', tab: 'Heap', unit: 'B', stroke: '#7c4dff' },
  { key: 'xrSys', tab: 'Sys', unit: 'B', stroke: '#1890ff' },
  { key: 'xrHeapObjects', tab: 'Objects', unit: '', stroke: '#13c2c2' },
  { key: 'xrNumGC', tab: 'GC Count', unit: '', stroke: '#fa8c16' },
  { key: 'xrPauseNs', tab: 'GC Pause', unit: 'ns', stroke: '#f5222d' },
  { key: OBS_KEY, tab: 'Observatory', unit: 'ms', stroke: '#52c41a' },
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
  const sameDay = d.getFullYear() === today.getFullYear()
    && d.getMonth() === today.getMonth()
    && d.getDate() === today.getDate();
  const hh = String(d.getHours()).padStart(2, '0');
  const mm = String(d.getMinutes()).padStart(2, '0');
  const ss = String(d.getSeconds()).padStart(2, '0');
  const time = `${hh}:${mm}:${ss}`;
  if (sameDay) return time;
  const MM = String(d.getMonth() + 1).padStart(2, '0');
  const DD = String(d.getDate()).padStart(2, '0');
  return `${MM}-${DD} ${time}`;
}

export default function XrayMetricsModal({ open, onClose }: XrayMetricsModalProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();
  const [activeKey, setActiveKey] = useState('xrAlloc');
  const [bucket, setBucket] = useState(2);
  const [points, setPoints] = useState<number[]>([]);
  const [labels, setLabels] = useState<string[]>([]);
  const [timestamps, setTimestamps] = useState<number[]>([]);
  const [state, setState] = useState<XrayState>({ enabled: false, listen: '', reason: '' });
  const [obsTags, setObsTags] = useState<ObservatoryTag[]>([]);
  const [obsActiveTag, setObsActiveTag] = useState('');
  const obsTimerRef = useRef<number | null>(null);
  const openRef = useRef(open);

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

  const applyHistory = useCallback((msg: Msg<{ t: number; v: number }[]> | null | undefined, currentBucket: number) => {
    if (msg?.success && Array.isArray(msg.obj)) {
      const vals: number[] = [];
      const labs: string[] = [];
      const tss: number[] = [];
      for (const p of msg.obj) {
        const d = new Date(p.t * 1000);
        const hh = String(d.getHours()).padStart(2, '0');
        const mm = String(d.getMinutes()).padStart(2, '0');
        const ss = String(d.getSeconds()).padStart(2, '0');
        labs.push(currentBucket >= 60 ? `${hh}:${mm}` : `${hh}:${mm}:${ss}`);
        vals.push(Number(p.v) || 0);
        tss.push(Number(p.t) || 0);
      }
      setLabels(labs);
      setPoints(vals);
      setTimestamps(tss);
    } else {
      setLabels([]);
      setPoints([]);
      setTimestamps([]);
    }
  }, []);

  const fetchState = useCallback(async () => {
    try {
      const msg = await HttpUtil.get<XrayState>('/panel/api/server/xrayMetricsState');
      if (msg?.success && msg.obj) setState(msg.obj);
    } catch (e) {
      console.error('Failed to fetch xray metrics state', e);
    }
  }, []);

  const fetchObservatory = useCallback(async () => {
    try {
      const msg = await HttpUtil.get<ObservatoryTag[]>('/panel/api/server/xrayObservatory');
      if (msg?.success && Array.isArray(msg.obj)) {
        const tags = msg.obj;
        setObsTags(tags);
        setObsActiveTag((prev) => {
          if (tags.find((tg) => tg.tag === prev)) return prev;
          return tags[0]?.tag || '';
        });
      } else {
        setObsTags([]);
      }
    } catch (e) {
      console.error('Failed to fetch observatory snapshot', e);
      setObsTags([]);
    }
  }, []);

  const fetchMetricBucket = useCallback(async () => {
    if (!activeMetric) return;
    try {
      const url = `/panel/api/server/xrayMetricsHistory/${activeMetric.key}/${bucket}`;
      const msg = await HttpUtil.get<{ t: number; v: number }[]>(url);
      applyHistory(msg, bucket);
    } catch (e) {
      console.error('Failed to fetch xray metrics bucket', e);
      setLabels([]);
      setPoints([]);
      setTimestamps([]);
    }
  }, [activeMetric, bucket, applyHistory]);

  const fetchObsBucket = useCallback(async () => {
    if (!obsActiveTag) {
      setLabels([]);
      setPoints([]);
      setTimestamps([]);
      return;
    }
    try {
      const url = `/panel/api/server/xrayObservatoryHistory/${encodeURIComponent(obsActiveTag)}/${bucket}`;
      const msg = await HttpUtil.get<{ t: number; v: number }[]>(url);
      applyHistory(msg, bucket);
    } catch (e) {
      console.error('Failed to fetch observatory bucket', e);
      setLabels([]);
      setPoints([]);
      setTimestamps([]);
    }
  }, [obsActiveTag, bucket, applyHistory]);

  const stopObsPolling = useCallback(() => {
    if (obsTimerRef.current != null) {
      window.clearInterval(obsTimerRef.current);
      obsTimerRef.current = null;
    }
  }, []);

  useEffect(() => {
    openRef.current = open;
    if (open) {
      setActiveKey('xrAlloc');
      fetchState();
    } else {
      stopObsPolling();
    }
  }, [open, fetchState, stopObsPolling]);

  useEffect(() => {
    if (!open) return;
    if (isObservatory) {
      fetchObservatory();
      fetchObsBucket();
      stopObsPolling();
      obsTimerRef.current = window.setInterval(async () => {
        if (!openRef.current || !isObservatory) return;
        await fetchObservatory();
        fetchObsBucket();
      }, 2000);
    } else {
      stopObsPolling();
      fetchMetricBucket();
    }
    return () => {
      stopObsPolling();
    };
  }, [open, activeKey, isObservatory, fetchObservatory, fetchObsBucket, fetchMetricBucket, stopObsPolling]);

  useEffect(() => {
    if (!open) return;
    if (isObservatory) {
      fetchObsBucket();
    } else {
      fetchMetricBucket();
    }
  }, [open, bucket, isObservatory, fetchObsBucket, fetchMetricBucket]);

  useEffect(() => {
    if (open && isObservatory) fetchObsBucket();
  }, [open, obsActiveTag, isObservatory, fetchObsBucket]);

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
              { value: 30, label: '30m' },
              { value: 60, label: '1h' },
              { value: 120, label: '2h' },
              { value: 180, label: '3h' },
              { value: 300, label: '5h' },
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
        items={METRICS.map((m) => ({ key: m.key, label: m.tab }))}
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
                    {t('pages.index.xrayObservatoryLastSeen')}: {fmtTimestamp(activeObsTag.lastSeenTime)}
                  </span>
                  <span className="obs-stamp">
                    {t('pages.index.xrayObservatoryLastTry')}: {fmtTimestamp(activeObsTag.lastTryTime)}
                  </span>
                </div>
              )}
            </div>
          )}
        </div>
      )}

      <div className="cpu-chart-wrap">
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
