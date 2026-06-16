import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { HttpUtil } from '@/utils';
import { Sparkline } from '@/components/viz';
import './NodeHistoryPanel.css';

interface NodeRef {
  id: number;
}

interface NodeHistoryPanelProps {
  node: NodeRef;
  bucket?: number;
}

interface SeriesPoint {
  t: number;
  v: number;
}

interface ApiMsg<T = unknown> {
  success?: boolean;
  obj?: T;
}

const REFRESH_MS = 15000;

export default function NodeHistoryPanel({ node, bucket = 30 }: NodeHistoryPanelProps) {
  const { t } = useTranslation();
  const [cpuPoints, setCpuPoints] = useState<number[]>([]);
  const [cpuLabels, setCpuLabels] = useState<string[]>([]);
  const [memPoints, setMemPoints] = useState<number[]>([]);
  const [memLabels, setMemLabels] = useState<string[]>([]);
  const [netUpPoints, setNetUpPoints] = useState<number[]>([]);
  const [netUpLabels, setNetUpLabels] = useState<string[]>([]);
  const [netDownPoints, setNetDownPoints] = useState<number[]>([]);
  const [netDownLabels, setNetDownLabels] = useState<string[]>([]);

  const lastNodeId = useRef<number>(node.id);

  useEffect(() => {
    let cancelled = false;

    const bucketLabel = (unixSec: number) => {
      const d = new Date(unixSec * 1000);
      const hh = String(d.getHours()).padStart(2, '0');
      const mm = String(d.getMinutes()).padStart(2, '0');
      if (bucket >= 60) return `${hh}:${mm}`;
      const ss = String(d.getSeconds()).padStart(2, '0');
      return `${hh}:${mm}:${ss}`;
    };

    // cpu/mem are percentages (clamp 0-100); net throughput is bytes/sec shown
    // as KB/s (no upper clamp, the sparkline auto-scales).
    const fetchSeries = async (metric: string, kind: 'pct' | 'rate') => {
      try {
        const url = `/panel/api/nodes/history/${node.id}/${metric}/${bucket}`;
        const msg = await HttpUtil.get(url) as ApiMsg<SeriesPoint[]>;
        if (msg?.success && Array.isArray(msg.obj)) {
          const vals: number[] = [];
          const labs: string[] = [];
          for (const p of msg.obj) {
            labs.push(bucketLabel(p.t));
            const n = Number(p.v) || 0;
            vals.push(kind === 'pct' ? Math.max(0, Math.min(100, n)) : Math.max(0, n / 1024));
          }
          return { vals, labs };
        }
      } catch (e) {
        console.error('node history fetch failed', metric, e);
      }
      return { vals: [] as number[], labs: [] as string[] };
    };

    const refresh = async () => {
      const [cpu, mem, netUp, netDown] = await Promise.all([
        fetchSeries('cpu', 'pct'),
        fetchSeries('mem', 'pct'),
        fetchSeries('netUp', 'rate'),
        fetchSeries('netDown', 'rate'),
      ]);
      if (cancelled) return;
      setCpuPoints(cpu.vals);
      setCpuLabels(cpu.labs);
      setMemPoints(mem.vals);
      setMemLabels(mem.labs);
      setNetUpPoints(netUp.vals);
      setNetUpLabels(netUp.labs);
      setNetDownPoints(netDown.vals);
      setNetDownLabels(netDown.labs);
    };

    refresh();
    const timer = window.setInterval(refresh, REFRESH_MS);
    lastNodeId.current = node.id;

    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [node.id, bucket]);

  return (
    <div className="node-history-panel">
      <div className="series">
        <div className="series-title">{t('pages.nodes.cpu')}</div>
        <Sparkline
          data={cpuPoints}
          labels={cpuLabels}
          height={120}
          stroke="#008771"
          showGrid
          showAxes
          tickCountX={4}
          maxPoints={cpuPoints.length || 1}
          fillOpacity={0.18}
          markerRadius={2.6}
          showTooltip
        />
      </div>
      <div className="series">
        <div className="series-title">{t('pages.nodes.mem')}</div>
        <Sparkline
          data={memPoints}
          labels={memLabels}
          height={120}
          stroke="#7c4dff"
          showGrid
          showAxes
          tickCountX={4}
          maxPoints={memPoints.length || 1}
          fillOpacity={0.18}
          markerRadius={2.6}
          showTooltip
        />
      </div>
      <div className="series">
        <div className="series-title">{t('pages.nodes.netUp')}</div>
        <Sparkline
          data={netUpPoints}
          labels={netUpLabels}
          height={120}
          stroke="#1677ff"
          showGrid
          showAxes
          tickCountX={4}
          maxPoints={netUpPoints.length || 1}
          fillOpacity={0.18}
          markerRadius={2.6}
          showTooltip
        />
      </div>
      <div className="series">
        <div className="series-title">{t('pages.nodes.netDown')}</div>
        <Sparkline
          data={netDownPoints}
          labels={netDownLabels}
          height={120}
          stroke="#fa8c16"
          showGrid
          showAxes
          tickCountX={4}
          maxPoints={netDownPoints.length || 1}
          fillOpacity={0.18}
          markerRadius={2.6}
          showTooltip
        />
      </div>
    </div>
  );
}
