import { useEffect, useMemo, useRef } from 'react';
import uPlot from 'uplot';
import 'uplot/dist/uPlot.min.css';
import './Sparkline.css';

export interface SparklineReferenceLine {
  y: number;
  label?: string;
  color?: string;
  dash?: string;
}

export interface SparklineExtrema {
  show?: boolean;
  formatter?: (v: number) => string;
  minColor?: string;
  maxColor?: string;
}

const DEFAULT_STROKE = '#008771';
const DEFAULT_STROKE2 = '#722ed1';
const DEFAULT_STROKE3 = '#a0d911';
const DEFAULT_MIN_COLOR = '#52c41a';
const DEFAULT_MAX_COLOR = '#fa541c';
const GRID_COLOR = 'rgba(128, 128, 140, 0.35)';
const AXIS_FONT = '10px system-ui, -apple-system, "Segoe UI", Roboto, sans-serif';
const LABEL_FONT = 'system-ui, -apple-system, "Segoe UI", Roboto, sans-serif';

interface SparklineProps {
  data: number[];
  data2?: number[];
  data3?: number[];
  stroke2?: string;
  stroke3?: string;
  name1?: string;
  name2?: string;
  name3?: string;
  labels?: (string | number)[];
  height?: number;
  stroke?: string;
  strokeWidth?: number;
  maxPoints?: number;
  showGrid?: boolean;
  fillOpacity?: number;
  showMarker?: boolean;
  markerRadius?: number;
  showAxes?: boolean;
  yTickStep?: number;
  tickCountX?: number;
  showTooltip?: boolean;
  valueMin?: number;
  valueMax?: number | null;
  yFormatter?: (v: number) => string;
  tooltipFormatter?: ((v: number) => string) | null;
  tooltipLabelFormatter?: ((label: string) => string) | null;
  referenceLines?: SparklineReferenceLine[];
  extrema?: SparklineExtrema;
}

interface ChartPoint {
  index: number;
  value: number;
  value2: number;
  value3: number;
  label: string;
}

interface ExtremaResult {
  min: ChartPoint;
  max: ChartPoint;
  minIdx: number;
  maxIdx: number;
}

interface SparklineView {
  points: ChartPoint[];
  yDomain: [number, number];
  yTicks: number[] | undefined;
  xTickIndexes: number[] | undefined;
  extremaPoints: ExtremaResult | null;
}

function hexToRgba(hex: string, alpha: number): string {
  let h = hex.trim();
  if (h.startsWith('#')) h = h.slice(1);
  if (h.length === 3) h = h.split('').map((c) => c + c).join('');
  if (h.length !== 6) return hex;
  const int = Number.parseInt(h, 16);
  if (Number.isNaN(int)) return hex;
  const r = (int >> 16) & 255;
  const g = (int >> 8) & 255;
  const b = int & 255;
  return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}

function cssVar(el: HTMLElement, name: string, fallback: string): string {
  const v = getComputedStyle(el).getPropertyValue(name).trim();
  return v || fallback;
}

function parseDash(dash: string, dpr: number): number[] {
  return dash.trim().split(/\s+/).map((n) => (Number(n) || 0) * dpr);
}

function dprOf(u: uPlot): number {
  return u.width > 0 ? u.ctx.canvas.width / u.width : (uPlot.pxRatio || 1);
}

export default function Sparkline(props: SparklineProps) {
  const {
    data,
    data2 = [],
    data3 = [],
    stroke = DEFAULT_STROKE,
    stroke2 = DEFAULT_STROKE2,
    stroke3 = DEFAULT_STROKE3,
    name1,
    name2,
    name3,
    labels = [],
    height = 80,
    strokeWidth = 2,
    maxPoints = 120,
    showGrid = true,
    fillOpacity = 0.22,
    showMarker = true,
    markerRadius = 3,
    showAxes = false,
    yTickStep = 25,
    tickCountX = 4,
    showTooltip = false,
    valueMin = 0,
    valueMax = 100,
    yFormatter = (v: number) => `${Math.round(v)}%`,
    tooltipFormatter = null,
    tooltipLabelFormatter = null,
    referenceLines,
    extrema,
  } = props;

  const hasSeries2 = data2.length > 0;
  const hasSeries3 = data3.length > 0;
  const multiSeries = hasSeries2 || hasSeries3;

  const points = useMemo<ChartPoint[]>(() => {
    const n = Math.min(data.length, maxPoints);
    if (n === 0) return [];
    const sliceStart = data.length - n;
    const labelStart = Math.max(0, labels.length - n);
    const slice2Start = data2.length - n;
    const slice3Start = data3.length - n;
    return data.slice(sliceStart).map((value, i) => ({
      index: i,
      value: Number(value) || 0,
      value2: data2.length ? Number(data2[slice2Start + i]) || 0 : 0,
      value3: data3.length ? Number(data3[slice3Start + i]) || 0 : 0,
      label: String(labels[labelStart + i] ?? i + 1),
    }));
  }, [data, data2, data3, labels, maxPoints]);

  const yDomain = useMemo<[number, number]>(() => {
    if (valueMax != null) return [valueMin, valueMax];
    let max = valueMin;
    for (const p of points) {
      if (Number.isFinite(p.value) && p.value > max) max = p.value;
      if (hasSeries2 && Number.isFinite(p.value2) && p.value2 > max) max = p.value2;
      if (hasSeries3 && Number.isFinite(p.value3) && p.value3 > max) max = p.value3;
    }
    if (max <= valueMin) max = valueMin + 1;
    return [valueMin, max * 1.1];
  }, [points, valueMin, valueMax, hasSeries2, hasSeries3]);

  const yTicks = useMemo<number[] | undefined>(() => {
    if (!showAxes) return undefined;
    const [min, max] = yDomain;
    if (valueMax === 100 && valueMin === 0 && yTickStep > 0) {
      const out: number[] = [];
      for (let v = min; v <= max; v += yTickStep) out.push(v);
      return out;
    }
    const n = 5;
    return Array.from({ length: n }, (_, i) => min + ((max - min) * i) / (n - 1));
  }, [showAxes, yDomain, valueMin, valueMax, yTickStep]);

  const xTickIndexes = useMemo<number[] | undefined>(() => {
    if (!showAxes || points.length === 0) return undefined;
    const m = Math.max(2, tickCountX);
    return Array.from({ length: m }, (_, i) => Math.round((i * (points.length - 1)) / (m - 1)));
  }, [showAxes, tickCountX, points.length]);

  const extremaPoints = useMemo<ExtremaResult | null>(() => {
    if (!extrema?.show || multiSeries || points.length < 2) return null;
    let minIdx = 0;
    let maxIdx = 0;
    for (let i = 1; i < points.length; i++) {
      if (points[i].value < points[minIdx].value) minIdx = i;
      if (points[i].value > points[maxIdx].value) maxIdx = i;
    }
    if (minIdx === maxIdx) return null;
    return { min: points[minIdx], max: points[maxIdx], minIdx, maxIdx };
  }, [points, extrema?.show, multiSeries]);

  const legendItems = useMemo(
    () =>
      [
        { name: name1, color: stroke },
        { name: name2, color: stroke2 },
        { name: name3, color: stroke3 },
      ].filter((s, i) => s.name && (i === 0 ? multiSeries : i === 1 ? hasSeries2 : hasSeries3)),
    [name1, name2, name3, stroke, stroke2, stroke3, multiSeries, hasSeries2, hasSeries3],
  );

  const fmtExtrema = extrema?.formatter ?? yFormatter;
  const minColor = extrema?.minColor ?? DEFAULT_MIN_COLOR;
  const maxColor = extrema?.maxColor ?? DEFAULT_MAX_COLOR;

  const ariaSummary = useMemo(() => {
    if (points.length === 0) return name1 ?? '';
    const last = points[points.length - 1];
    const parts: string[] = [];
    parts.push(name1 ? `${name1}: ${yFormatter(last.value)}` : yFormatter(last.value));
    if (hasSeries2 && name2) parts.push(`${name2}: ${yFormatter(last.value2)}`);
    if (hasSeries3 && name3) parts.push(`${name3}: ${yFormatter(last.value3)}`);
    return parts.join(', ');
  }, [points, name1, name2, name3, hasSeries2, hasSeries3, yFormatter]);

  const cfg = {
    stroke,
    stroke2,
    stroke3,
    strokeWidth,
    fillOpacity,
    markerRadius,
    showGrid,
    showMarker,
    showAxes,
    showTooltip,
    height,
    name1,
    name2,
    name3,
    yFormatter,
    tooltipFormatter,
    tooltipLabelFormatter,
    referenceLines,
    extrema,
  };
  const cfgRef = useRef(cfg);
  cfgRef.current = cfg;
  const viewRef = useRef<SparklineView>({ points, yDomain, yTicks, xTickIndexes, extremaPoints });
  viewRef.current = { points, yDomain, yTicks, xTickIndexes, extremaPoints };

  const containerRef = useRef<HTMLDivElement>(null);
  const plotRef = useRef<uPlot | null>(null);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    let tooltipEl: HTMLDivElement | null = null;

    const seriesColor = (idx: number): string => {
      const p = cfgRef.current;
      if (idx <= 1) return p.stroke ?? DEFAULT_STROKE;
      if (idx === 2) return p.stroke2 ?? DEFAULT_STROKE2;
      return p.stroke3 ?? DEFAULT_STROKE3;
    };

    const gridTicks = (): number[] => {
      const yt = viewRef.current.yTicks;
      if (yt && yt.length) return yt;
      const [mn, mx] = viewRef.current.yDomain;
      const n = 4;
      return Array.from({ length: n + 1 }, (_, i) => mn + ((mx - mn) * i) / n);
    };

    const buildData = (): uPlot.AlignedData => {
      const v = viewRef.current;
      const xs = v.points.map((_, i) => i);
      const series: number[][] = [v.points.map((p) => p.value)];
      if (hasSeries2) series.push(v.points.map((p) => p.value2));
      if (hasSeries3) series.push(v.points.map((p) => p.value3));
      return [xs, ...series];
    };

    const makeSeries = (): uPlot.Series => ({
      stroke: (_u, sidx) => seriesColor(sidx),
      width: cfgRef.current.strokeWidth ?? 2,
      fill: (u, sidx) => {
        const { ctx, bbox } = u;
        const color = seriesColor(sidx);
        const grad = ctx.createLinearGradient(0, bbox.top, 0, bbox.top + bbox.height);
        grad.addColorStop(0, hexToRgba(color, cfgRef.current.fillOpacity ?? 0.22));
        grad.addColorStop(1, hexToRgba(color, 0));
        return grad;
      },
      paths: uPlot.paths.spline?.(),
      points: { show: false },
      spanGaps: true,
    });

    const series: uPlot.Series[] = [{}, makeSeries()];
    if (hasSeries2) series.push(makeSeries());
    if (hasSeries3) series.push(makeSeries());

    const axisStroke = (u: uPlot) => cssVar(u.root, '--ant-color-text-tertiary', '#8c8c8c');

    const axes: uPlot.Axis[] = [
      {
        show: showAxes,
        stroke: axisStroke,
        grid: { show: false },
        ticks: { show: false },
        font: AXIS_FONT,
        gap: 6,
        size: 28,
        splits: () => viewRef.current.xTickIndexes ?? [],
        values: (_u, splits) => splits.map((i) => viewRef.current.points[i]?.label ?? ''),
      },
      {
        show: showAxes,
        scale: 'y',
        side: 3,
        stroke: axisStroke,
        grid: { show: false },
        ticks: { show: false },
        font: AXIS_FONT,
        gap: 4,
        size: 56,
        splits: () => viewRef.current.yTicks ?? [],
        values: (_u, splits) =>
          splits.map((v) => (cfgRef.current.yFormatter ? cfgRef.current.yFormatter(v) : String(v))),
      },
    ];

    const drawGrid = (u: uPlot) => {
      if (cfgRef.current.showGrid === false) return;
      const { ctx, bbox } = u;
      const dpr = dprOf(u);
      ctx.save();
      ctx.strokeStyle = GRID_COLOR;
      ctx.lineWidth = dpr;
      ctx.setLineDash([3 * dpr, 4 * dpr]);
      ctx.beginPath();
      for (const ty of gridTicks()) {
        const py = Math.round(u.valToPos(ty, 'y', true)) + 0.5;
        ctx.moveTo(bbox.left, py);
        ctx.lineTo(bbox.left + bbox.width, py);
      }
      ctx.stroke();
      ctx.restore();
    };

    const drawOverlay = (u: uPlot) => {
      const p = cfgRef.current;
      const v = viewRef.current;
      const { ctx, bbox } = u;
      const dpr = dprOf(u);
      const right = bbox.left + bbox.width;

      if (p.referenceLines?.length) {
        for (const rl of p.referenceLines) {
          const color = rl.color || p.stroke || DEFAULT_STROKE;
          const py = Math.round(u.valToPos(rl.y, 'y', true)) + 0.5;
          ctx.save();
          ctx.strokeStyle = color;
          ctx.lineWidth = 1.4 * dpr;
          ctx.setLineDash(parseDash(rl.dash ?? '5 4', dpr));
          ctx.beginPath();
          ctx.moveTo(bbox.left, py);
          ctx.lineTo(right, py);
          ctx.stroke();
          ctx.restore();
          if (rl.label) {
            ctx.save();
            ctx.fillStyle = color;
            ctx.font = `600 ${10 * dpr}px ${LABEL_FONT}`;
            ctx.textAlign = 'right';
            ctx.textBaseline = 'bottom';
            ctx.fillText(rl.label, right - 4 * dpr, py - 3 * dpr);
            ctx.restore();
          }
        }
      }

      const ex = v.extremaPoints;
      if (p.extrema?.show && ex) {
        const ringColor = cssVar(u.root, '--ant-color-bg-elevated', '#ffffff');
        const dot = (value: number, idx: number, color: string) => {
          const px = u.valToPos(idx, 'x', true);
          const py = u.valToPos(value, 'y', true);
          ctx.save();
          ctx.beginPath();
          ctx.arc(px, py, 4.5 * dpr, 0, Math.PI * 2);
          ctx.fillStyle = color;
          ctx.fill();
          ctx.lineWidth = 2 * dpr;
          ctx.strokeStyle = ringColor;
          ctx.stroke();
          ctx.restore();
        };
        dot(ex.max.value, ex.maxIdx, p.extrema.maxColor ?? DEFAULT_MAX_COLOR);
        dot(ex.min.value, ex.minIdx, p.extrema.minColor ?? DEFAULT_MIN_COLOR);
      }
    };

    const updateTooltip = (u: uPlot) => {
      if (!tooltipEl) return;
      const idx = u.cursor.idx;
      const v = viewRef.current;
      const p = cfgRef.current;
      if (idx == null || idx < 0 || idx >= v.points.length) {
        tooltipEl.style.display = 'none';
        return;
      }
      const pt = v.points[idx];
      const fmt = p.tooltipFormatter ?? p.yFormatter ?? ((x: number) => String(x));
      const label = p.tooltipLabelFormatter ? p.tooltipLabelFormatter(String(pt.label)) : String(pt.label);
      const multi = hasSeries2 || hasSeries3;

      tooltipEl.textContent = '';
      const labelDiv = document.createElement('div');
      labelDiv.className = 'spk-tt-label';
      labelDiv.textContent = label;
      tooltipEl.appendChild(labelDiv);

      const rows = [
        { name: p.name1, color: p.stroke ?? DEFAULT_STROKE, val: pt.value, on: true },
        { name: p.name2, color: p.stroke2 ?? DEFAULT_STROKE2, val: pt.value2, on: hasSeries2 },
        { name: p.name3, color: p.stroke3 ?? DEFAULT_STROKE3, val: pt.value3, on: hasSeries3 },
      ];
      for (const r of rows) {
        if (!r.on) continue;
        const row = document.createElement('div');
        row.className = 'spk-tt-row';
        if (multi) {
          const marker = document.createElement('span');
          marker.className = 'spk-tt-dot';
          marker.style.background = r.color;
          row.appendChild(marker);
          const nm = document.createElement('span');
          nm.className = 'spk-tt-name';
          nm.textContent = r.name ?? '';
          row.appendChild(nm);
        }
        const val = document.createElement('span');
        val.className = 'spk-tt-val';
        val.textContent = fmt(r.val);
        row.appendChild(val);
        tooltipEl.appendChild(row);
      }

      tooltipEl.style.display = '';
      const overW = u.over.clientWidth;
      const overH = u.over.clientHeight;
      const cx = u.cursor.left ?? 0;
      const cy = u.cursor.top ?? 0;
      const w = tooltipEl.offsetWidth;
      const h = tooltipEl.offsetHeight;
      let x = cx + 12;
      if (x + w + 8 > overW) x = cx - w - 12;
      if (x < 0) x = 4;
      let y = cy - h - 12;
      if (y < 0) y = Math.min(cy + 12, overH - h - 4);
      tooltipEl.style.transform = `translate(${Math.round(x)}px, ${Math.round(y)}px)`;
    };

    const opts: uPlot.Options = {
      width: container.clientWidth || 600,
      height,
      padding: [8, 8, showAxes ? 0 : 2, showAxes ? 0 : 2],
      legend: { show: false },
      cursor: {
        show: showTooltip,
        x: showTooltip,
        y: false,
        drag: { x: false, y: false, setScale: false },
        points: {
          show: showMarker,
          size: () => (cfgRef.current.markerRadius ?? 3) * 2,
          width: 0,
          stroke: (_u, sidx) => seriesColor(sidx),
          fill: (_u, sidx) => seriesColor(sidx),
        },
      },
      scales: {
        x: {
          time: false,
          range: (_u, dmin, dmax) => (dmin === dmax ? [dmin - 0.5, dmax + 0.5] : [dmin, dmax]),
        },
        y: {
          range: () => {
            const [mn, mx] = viewRef.current.yDomain;
            return [mn, mx];
          },
        },
      },
      series,
      axes,
      hooks: {
        init: [
          (u) => {
            if (!cfgRef.current.showTooltip) return;
            tooltipEl = document.createElement('div');
            tooltipEl.className = 'sparkline-tooltip';
            tooltipEl.style.display = 'none';
            u.over.appendChild(tooltipEl);
          },
        ],
        drawClear: [drawGrid],
        draw: [drawOverlay],
        setCursor: [updateTooltip],
      },
    };

    const u = new uPlot(opts, buildData(), container);
    plotRef.current = u;

    const ro = new ResizeObserver(() => {
      const w = container.clientWidth;
      if (w > 0) u.setSize({ width: w, height });
    });
    ro.observe(container);

    return () => {
      ro.disconnect();
      u.destroy();
      plotRef.current = null;
      tooltipEl = null;
    };
  }, [hasSeries2, hasSeries3, showAxes, showTooltip, showMarker, height]);

  useEffect(() => {
    plotRef.current?.setData(
      (() => {
        const xs = points.map((_, i) => i);
        const s: number[][] = [points.map((p) => p.value)];
        if (hasSeries2) s.push(points.map((p) => p.value2));
        if (hasSeries3) s.push(points.map((p) => p.value3));
        return [xs, ...s] as uPlot.AlignedData;
      })(),
    );
  }, [points, hasSeries2, hasSeries3, valueMin, valueMax]);

  useEffect(() => {
    plotRef.current?.redraw(false);
  });

  useEffect(() => {
    const redraw = () => plotRef.current?.redraw(false);
    const moBody = new MutationObserver(redraw);
    moBody.observe(document.body, { attributes: true, attributeFilter: ['class'] });
    const moRoot = new MutationObserver(redraw);
    moRoot.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
    return () => {
      moBody.disconnect();
      moRoot.disconnect();
    };
  }, []);

  return (
    <div className="sparkline-container" role={ariaSummary ? 'img' : undefined} aria-label={ariaSummary || undefined}>
      {extremaPoints && (
        <div className="sparkline-extrema" aria-hidden="true">
          <span className="extrema-item" style={{ color: maxColor }}>
            ▲ {fmtExtrema(extremaPoints.max.value)}
          </span>
          <span className="extrema-item" style={{ color: minColor }}>
            ▼ {fmtExtrema(extremaPoints.min.value)}
          </span>
        </div>
      )}
      {legendItems.length > 0 && (
        <div className="sparkline-legend" aria-hidden="true">
          {legendItems.map((s) => (
            <span key={s.name} className="extrema-item" style={{ color: s.color }}>● {s.name}</span>
          ))}
        </div>
      )}
      <div ref={containerRef} className="sparkline-plot" style={{ height }} />
    </div>
  );
}
