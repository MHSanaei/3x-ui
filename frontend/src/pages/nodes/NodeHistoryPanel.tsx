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

    const fetchSeries = async (metric: 'cpu' | 'mem') => {
      try {
        const url = `/panel/api/nodes/history/${node.id}/${metric}/${bucket}`;
        const msg = await HttpUtil.get(url) as ApiMsg<SeriesPoint[]>;
        if (msg?.success && Array.isArray(msg.obj)) {
          const vals: number[] = [];
          const labs: string[] = [];
          for (const p of msg.obj) {
            labs.push(bucketLabel(p.t));
            vals.push(Math.max(0, Math.min(100, Number(p.v) || 0)));
          }
          return { vals, labs };
        }
      } catch (e) {
        console.error('node history fetch failed', metric, e);
      }
      return { vals: [] as number[], labs: [] as string[] };
    };

    const refresh = async () => {
      const [cpu, mem] = await Promise.all([fetchSeries('cpu'), fetchSeries('mem')]);
      if (cancelled) return;
      setCpuPoints(cpu.vals);
      setCpuLabels(cpu.labs);
      setMemPoints(mem.vals);
      setMemLabels(mem.labs);
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
    </div>
  );
}
