import { useCallback, useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Modal, Select, Tabs } from 'antd';
import {
  ApiOutlined,
  DashboardOutlined,
  DatabaseOutlined,
  DeploymentUnitOutlined,
  GlobalOutlined,
  HddOutlined,
  LineChartOutlined,
  PieChartOutlined,
  TeamOutlined,
} from '@ant-design/icons';

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
  tabKey?: string;
  title: string;
  icon: ReactNode;
  valueMax: number | null;
  unit: string;
  stroke: string;
  key2?: string;
  stroke2?: string;
  name1?: string;
  name2?: string;
  key3?: string;
  stroke3?: string;
  name3?: string;
}

const METRICS: MetricDef[] = [
  { key: 'cpu', tab: 'CPU', tabKey: 'pages.index.cpu', title: 'pages.index.historyTitleCpu', icon: <DashboardOutlined />, valueMax: 100, unit: '%', stroke: '' },
  { key: 'mem', tab: 'RAM', tabKey: 'pages.index.memory', title: 'pages.index.historyTitleMem', icon: <DatabaseOutlined />, valueMax: 100, unit: '%', stroke: '#7c4dff', key2: 'swap', stroke2: '#ffa940', name1: 'pages.index.memory', name2: 'pages.index.swap' },
  { key: 'netUp', tab: 'Bandwidth', tabKey: 'pages.index.historyTabBandwidth', title: 'pages.index.historyTitleNetwork', icon: <GlobalOutlined />, valueMax: null, unit: 'B/s', stroke: '#1890ff', key2: 'netDown', stroke2: '#13c2c2', name1: 'Up', name2: 'Down' },
  { key: 'pktUp', tab: 'Packets', tabKey: 'pages.index.historyTabPackets', title: 'pages.index.historyTitlePackets', icon: <DeploymentUnitOutlined />, valueMax: null, unit: 'pkt/s', stroke: '#2f54eb', key2: 'pktDown', stroke2: '#36cfc9', name1: 'Up', name2: 'Down' },
  { key: 'tcpCount', tab: 'Connections', tabKey: 'pages.index.historyTabConnections', title: 'pages.index.historyTitleConnections', icon: <ApiOutlined />, valueMax: null, unit: '', stroke: '#597ef7', key2: 'udpCount', stroke2: '#73d13d', name1: 'TCP', name2: 'UDP' },
  { key: 'diskRead', tab: 'Disk I/O', tabKey: 'pages.index.historyTabDisk', title: 'pages.index.historyTitleDisk', icon: <HddOutlined />, valueMax: null, unit: 'B/s', stroke: '#eb2f96', key2: 'diskWrite', stroke2: '#722ed1', name1: 'Read', name2: 'Write' },
  { key: 'diskUsage', tab: 'Disk Usage', tabKey: 'pages.index.historyTabDiskUsage', title: 'pages.index.historyTitleDiskUsage', icon: <PieChartOutlined />, valueMax: 100, unit: '%', stroke: '#13c2c2' },
  { key: 'online', tab: 'Online', tabKey: 'pages.index.historyTabOnline', title: 'pages.index.historyTitleOnline', icon: <TeamOutlined />, valueMax: null, unit: '', stroke: '#52c41a' },
  { key: 'load1', tab: 'Load', tabKey: 'pages.index.historyTabLoad', title: 'pages.index.historyTitleLoad', icon: <LineChartOutlined />, valueMax: null, unit: '', stroke: '#fa8c16', key2: 'load5', stroke2: '#f5222d', name1: '1m', name2: '5m', key3: 'load15', stroke3: '#a0d911', name3: '15m' },
];

function unitFormatter(unit: string, activeKey: string): (v: number) => string {
  if (unit === 'B/s') {
    return (v) => `${SizeFormatter.sizeFormat(Math.max(0, Number(v) || 0)).replace(/\.\d+/, '')}/s`;
  }
  if (unit === 'pkt/s') {
    return (v) => `${Math.round(Math.max(0, Number(v) || 0)).toLocaleString()}/s`;
  }
  if (unit === '%') {
    return (v) => `${Number(v).toFixed(1)}%`;
  }
  return (v) => {
    const n = Number(v) || 0;
    if (activeKey === 'online' || activeKey === 'tcpCount' || activeKey === 'udpCount') {
      return Math.round(n).toLocaleString();
    }
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
  const [points2, setPoints2] = useState<number[]>([]);
  const [points3, setPoints3] = useState<number[]>([]);
  const [labels, setLabels] = useState<string[]>([]);
  const [timestamps, setTimestamps] = useState<number[]>([]);

  const activeMetric = useMemo(() => METRICS.find((m) => m.key === activeKey), [activeKey]);
  const trName = (n?: string) => (n && n.startsWith('pages.') ? t(n) : n);
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

        const fetchAligned = async (key?: string): Promise<number[]> => {
          if (!key) return [];
          const m = await HttpUtil.get(`/panel/api/server/history/${key}/${bucket}`);
          if (m?.success && Array.isArray(m.obj)) {
            const byTs = new Map<number, number>();
            for (const p of m.obj) byTs.set(Number(p.t) || 0, Number(p.v) || 0);
            return tss.map((ts) => byTs.get(ts) ?? 0);
          }
          return [];
        };
        setPoints2(await fetchAligned(activeMetric.key2));
        setPoints3(await fetchAligned(activeMetric.key3));
      } else {
        setLabels([]);
        setPoints([]);
        setPoints2([]);
        setPoints3([]);
        setTimestamps([]);
      }
    } catch (e) {
      console.error('Failed to fetch history bucket', e);
      setLabels([]);
      setPoints([]);
      setPoints2([]);
      setPoints3([]);
      setTimestamps([]);
    }
  }, [activeMetric, bucket]);

  useEffect(() => {
    if (open) setActiveKey('cpu');
  }, [open]);

  useEffect(() => {
    if (open) fetchBucket();
  }, [open, activeKey, bucket, fetchBucket]);

  useEffect(() => {
    if (!open) return undefined;
    const ms = bucket <= 30 ? 2000 : 10000;
    const id = window.setInterval(() => fetchBucket(), ms);
    return () => window.clearInterval(id);
  }, [open, bucket, fetchBucket]);

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
        items={METRICS.map((m) => {
          const tabLabel = m.tabKey ? t(m.tabKey) : m.tab;
          return {
            key: m.key,
            label: isMobile ? <span title={tabLabel} aria-label={tabLabel}>{m.icon}</span> : tabLabel,
          };
        })}
      />

      <div className="cpu-chart-wrap">
        {activeMetric?.title && <div className="history-chart-title">{t(activeMetric.title)}</div>}
        <Sparkline
          data={points}
          data2={activeMetric?.key2 ? points2 : undefined}
          data3={activeMetric?.key3 ? points3 : undefined}
          stroke2={activeMetric?.stroke2}
          stroke3={activeMetric?.stroke3}
          name1={trName(activeMetric?.name1)}
          name2={trName(activeMetric?.name2)}
          name3={trName(activeMetric?.name3)}
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
          extrema={{ show: !activeMetric?.key2, formatter: yFormatter }}
        />
      </div>
    </Modal>
  );
}
