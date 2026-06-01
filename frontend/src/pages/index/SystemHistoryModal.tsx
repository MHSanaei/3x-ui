import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Modal, Select, Tabs } from 'antd';

import { HttpUtil, SizeFormatter } from '@/utils';
import { Sparkline } from '@/components/viz';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import type { Status } from '@/models/status';
import './SystemHistoryModal.css';

interface SystemHistoryModalProps {
  open: boolean;
  status: Status;
  onClose: () => void;
}

interface MetricDef {
  key: string;
  tab: string;
  valueMax: number | null;
  unit: string;
  stroke: string;
}

const METRICS: MetricDef[] = [
  { key: 'cpu', tab: 'CPU', valueMax: 100, unit: '%', stroke: '' },
  { key: 'mem', tab: 'RAM', valueMax: 100, unit: '%', stroke: '#7c4dff' },
  { key: 'netUp', tab: 'Net Up', valueMax: null, unit: 'B/s', stroke: '#1890ff' },
  { key: 'netDown', tab: 'Net Down', valueMax: null, unit: 'B/s', stroke: '#13c2c2' },
  { key: 'online', tab: 'Online', valueMax: null, unit: '', stroke: '#52c41a' },
  { key: 'load1', tab: 'Load 1m', valueMax: null, unit: '', stroke: '#fa8c16' },
  { key: 'load5', tab: 'Load 5m', valueMax: null, unit: '', stroke: '#f5222d' },
  { key: 'load15', tab: 'Load 15m', valueMax: null, unit: '', stroke: '#a0d911' },
];

function unitFormatter(unit: string, activeKey: string): (v: number) => string {
  if (unit === 'B/s') {
    return (v) => `${SizeFormatter.sizeFormat(Math.max(0, Number(v) || 0)).replace(/\.\d+/, '')}/s`;
  }
  if (unit === '%') {
    return (v) => `${Number(v).toFixed(1)}%`;
  }
  return (v) => {
    const n = Number(v) || 0;
    if (activeKey === 'online') return String(Math.round(n));
    return n.toFixed(2);
  };
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

export default function SystemHistoryModal({ open, status, onClose }: SystemHistoryModalProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();
  const [activeKey, setActiveKey] = useState('cpu');
  const [bucket, setBucket] = useState(2);
  const [points, setPoints] = useState<number[]>([]);
  const [labels, setLabels] = useState<string[]>([]);
  const [timestamps, setTimestamps] = useState<number[]>([]);

  const activeMetric = useMemo(() => METRICS.find((m) => m.key === activeKey), [activeKey]);
  const strokeColor = activeMetric?.stroke || status?.cpu?.color || '#008771';
  const yFormatter = useMemo(
    () => unitFormatter(activeMetric?.unit ?? '', activeKey),
    [activeMetric, activeKey],
  );

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

  const fetchBucket = useCallback(async () => {
    if (!activeMetric) return;
    try {
      const url = `/panel/api/server/history/${activeMetric.key}/${bucket}`;
      const msg = await HttpUtil.get(url);
      if (msg?.success && Array.isArray(msg.obj)) {
        const vals: number[] = [];
        const labs: string[] = [];
        const tss: number[] = [];
        for (const p of msg.obj) {
          const d = new Date(p.t * 1000);
          const hh = String(d.getHours()).padStart(2, '0');
          const mm = String(d.getMinutes()).padStart(2, '0');
          const ss = String(d.getSeconds()).padStart(2, '0');
          labs.push(bucket >= 60 ? `${hh}:${mm}` : `${hh}:${mm}:${ss}`);
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
    } catch (e) {
      console.error('Failed to fetch history bucket', e);
      setLabels([]);
      setPoints([]);
      setTimestamps([]);
    }
  }, [activeMetric, bucket]);

  useEffect(() => {
    if (open) setActiveKey('cpu');
  }, [open]);

  useEffect(() => {
    if (open) fetchBucket();
  }, [open, activeKey, bucket, fetchBucket]);

  return (
    <Modal
      open={open}
      footer={null}
      width={isMobile ? '95vw' : 900}
      onCancel={onClose}
      title={
        <div className="metric-modal-title">
          <span>{t('pages.index.systemHistoryTitle')}</span>
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
      <Tabs
        activeKey={activeKey}
        onChange={setActiveKey}
        size="small"
        className="history-tabs"
        items={METRICS.map((m) => ({ key: m.key, label: m.tab }))}
      />

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
          valueMax={activeMetric?.valueMax ?? null}
          yFormatter={yFormatter}
          tooltipLabelFormatter={tooltipLabelFormatter}
          extrema={{ show: true, formatter: yFormatter }}
        />
      </div>
    </Modal>
  );
}
