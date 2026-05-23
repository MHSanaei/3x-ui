import { useCallback, useEffect, useId, useMemo, useRef, useState } from 'react';
import type { MouseEvent } from 'react';
import './Sparkline.css';

interface SparklineProps {
  data: number[];
  labels?: (string | number)[];
  vbWidth?: number;
  height?: number;
  stroke?: string;
  strokeWidth?: number;
  maxPoints?: number;
  showGrid?: boolean;
  gridColor?: string;
  fillOpacity?: number;
  showMarker?: boolean;
  markerRadius?: number;
  showAxes?: boolean;
  yTickStep?: number;
  tickCountX?: number;
  paddingLeft?: number;
  paddingRight?: number;
  paddingTop?: number;
  paddingBottom?: number;
  showTooltip?: boolean;
  valueMin?: number;
  valueMax?: number | null;
  yFormatter?: (v: number) => string;
  tooltipFormatter?: ((v: number) => string) | null;
}

export default function Sparkline({
  data,
  labels = [],
  vbWidth = 320,
  height = 80,
  stroke = '#008771',
  strokeWidth = 2,
  maxPoints = 120,
  showGrid = true,
  gridColor = 'rgba(0,0,0,0.08)',
  fillOpacity = 0.22,
  showMarker = true,
  markerRadius = 3,
  showAxes = false,
  yTickStep = 25,
  tickCountX = 4,
  paddingLeft = 56,
  paddingRight = 6,
  paddingTop = 6,
  paddingBottom = 20,
  showTooltip = false,
  valueMin = 0,
  valueMax = 100,
  yFormatter = (v: number) => `${Math.round(v)}%`,
  tooltipFormatter = null,
}: SparklineProps) {
  const svgRef = useRef<SVGSVGElement | null>(null);
  const [measuredWidth, setMeasuredWidth] = useState(0);
  const [hoverIdx, setHoverIdx] = useState(-1);

  const reactId = useId();
  const safeId = reactId.replace(/[^a-zA-Z0-9]/g, '');
  const gradId = `spkGrad-${safeId}`;
  const shadowId = `spkShadow-${safeId}`;
  const glowId = `spkGlow-${safeId}`;

  useEffect(() => {
    const el = svgRef.current;
    if (!el) return;
    const measure = () => {
      const w = el.getBoundingClientRect?.().width || 0;
      if (w > 0) setMeasuredWidth(Math.round(w));
    };
    measure();
    if (typeof ResizeObserver !== 'undefined') {
      const ro = new ResizeObserver(measure);
      ro.observe(el);
      return () => ro.disconnect();
    }
    window.addEventListener('resize', measure);
    return () => window.removeEventListener('resize', measure);
  }, []);

  const effectiveVbWidth = measuredWidth > 0 ? measuredWidth : vbWidth;
  const drawWidth = Math.max(1, effectiveVbWidth - paddingLeft - paddingRight);
  const drawHeight = Math.max(1, height - paddingTop - paddingBottom);
  const nPoints = Math.min(data.length, maxPoints);

  const dataSlice = useMemo(
    () => (nPoints === 0 ? [] : data.slice(data.length - nPoints)),
    [data, nPoints],
  );

  const labelsSlice = useMemo(() => {
    if (!labels?.length || nPoints === 0) return [] as (string | number)[];
    const start = Math.max(0, labels.length - nPoints);
    return labels.slice(start);
  }, [labels, nPoints]);

  const yDomain = useMemo(() => {
    const min = valueMin;
    if (valueMax != null) return { min, max: valueMax };
    let max = min;
    for (const v of dataSlice) {
      const n = Number(v);
      if (Number.isFinite(n) && n > max) max = n;
    }
    if (max <= min) max = min + 1;
    return { min, max: max * 1.1 };
  }, [dataSlice, valueMin, valueMax]);

  const project = useCallback(
    (v: number) => {
      const { min, max } = yDomain;
      const span = max - min;
      if (span <= 0) return paddingTop + drawHeight;
      const clipped = Math.max(min, Math.min(max, Number(v) || 0));
      const ratio = (clipped - min) / span;
      return Math.round(paddingTop + (drawHeight - ratio * drawHeight));
    },
    [yDomain, paddingTop, drawHeight],
  );

  const pointsArr = useMemo<[number, number][]>(() => {
    if (nPoints === 0) return [];
    const w = drawWidth;
    const dx = nPoints > 1 ? w / (nPoints - 1) : 0;
    return dataSlice.map((v, i) => {
      const x = Math.round(paddingLeft + i * dx);
      return [x, project(v)];
    });
  }, [dataSlice, nPoints, drawWidth, paddingLeft, project]);

  const pointsStr = useMemo(() => pointsArr.map((p) => `${p[0]},${p[1]}`).join(' '), [pointsArr]);

  const areaPath = useMemo(() => {
    if (pointsArr.length === 0) return '';
    const first = pointsArr[0];
    const last = pointsArr[pointsArr.length - 1];
    const baseY = paddingTop + drawHeight;
    const line = pointsStr.replace(/ /g, ' L ');
    return `M ${first[0]},${baseY} L ${line} L ${last[0]},${baseY} Z`;
  }, [pointsArr, pointsStr, paddingTop, drawHeight]);

  const gridLines = useMemo(() => {
    if (!showGrid) return [];
    const h = drawHeight;
    const w = drawWidth;
    return [0, 0.25, 0.5, 0.75, 1].map((r) => {
      const y = Math.round(paddingTop + h * r);
      return { x1: paddingLeft, y1: y, x2: paddingLeft + w, y2: y };
    });
  }, [showGrid, drawHeight, drawWidth, paddingTop, paddingLeft]);

  const lastPoint = pointsArr.length === 0 ? null : pointsArr[pointsArr.length - 1];

  const yTicks = useMemo(() => {
    if (!showAxes) return [];
    const { min, max } = yDomain;
    const out: { y: number; label: string }[] = [];
    if (valueMax === 100 && valueMin === 0 && yTickStep > 0) {
      for (let p = min; p <= max; p += yTickStep) {
        out.push({ y: project(p), label: yFormatter(p) });
      }
      return out;
    }
    const ticks = 5;
    for (let i = 0; i < ticks; i++) {
      const v = min + ((max - min) * i) / (ticks - 1);
      out.push({ y: project(v), label: yFormatter(v) });
    }
    return out;
  }, [showAxes, yDomain, valueMax, valueMin, yTickStep, project, yFormatter]);

  const xTicks = useMemo(() => {
    if (!showAxes) return [];
    if (nPoints === 0) return [];
    const m = Math.max(2, tickCountX);
    const w = drawWidth;
    const dx = nPoints > 1 ? w / (nPoints - 1) : 0;
    const out: { x: number; label: string }[] = [];
    for (let i = 0; i < m; i++) {
      const idx = Math.round((i * (nPoints - 1)) / (m - 1));
      const label = labelsSlice[idx] != null ? String(labelsSlice[idx]) : String(idx);
      const x = Math.round(paddingLeft + idx * dx);
      out.push({ x, label });
    }
    return out;
  }, [showAxes, labelsSlice, nPoints, tickCountX, drawWidth, paddingLeft]);

  const onMouseMove = useCallback(
    (evt: MouseEvent<SVGSVGElement>) => {
      if (!showTooltip || pointsArr.length === 0) return;
      const rect = evt.currentTarget.getBoundingClientRect();
      const px = evt.clientX - rect.left;
      const x = (px / rect.width) * effectiveVbWidth;
      const dx = nPoints > 1 ? drawWidth / (nPoints - 1) : 0;
      const idx = Math.max(0, Math.min(nPoints - 1, Math.round((x - paddingLeft) / (dx || 1))));
      setHoverIdx(idx);
    },
    [showTooltip, pointsArr.length, effectiveVbWidth, nPoints, drawWidth, paddingLeft],
  );

  const onMouseLeave = useCallback(() => setHoverIdx(-1), []);

  const hoverText = useMemo(() => {
    const idx = hoverIdx;
    if (idx < 0 || idx >= dataSlice.length) return '';
    const raw = Number(dataSlice[idx] || 0);
    const fmt = tooltipFormatter || yFormatter;
    const val = fmt(Number.isFinite(raw) ? raw : 0);
    const lab = labelsSlice[idx] != null ? labelsSlice[idx] : '';
    return `${val}${lab ? ' • ' + lab : ''}`;
  }, [hoverIdx, dataSlice, labelsSlice, tooltipFormatter, yFormatter]);

  const tooltipPillWidth = Math.max(48, hoverText.length * 6.2 + 14);
  const hoverPoint = hoverIdx >= 0 ? pointsArr[hoverIdx] : null;
  const tooltipX = hoverPoint
    ? Math.max(
        paddingLeft + 2,
        Math.min(effectiveVbWidth - paddingRight - tooltipPillWidth - 2, hoverPoint[0] - tooltipPillWidth / 2),
      )
    : 0;

  return (
    <svg
      ref={svgRef}
      width="100%"
      height={height}
      viewBox={`0 0 ${effectiveVbWidth} ${height}`}
      preserveAspectRatio="none"
      className="sparkline-svg"
      onMouseMove={onMouseMove}
      onMouseLeave={onMouseLeave}
    >
      <defs>
        <linearGradient id={gradId} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={stroke} stopOpacity={Math.min(1, fillOpacity * 1.8)} />
          <stop offset="50%" stopColor={stroke} stopOpacity={fillOpacity * 0.7} />
          <stop offset="100%" stopColor={stroke} stopOpacity={0} />
        </linearGradient>
        <filter id={shadowId} x="-10%" y="-50%" width="120%" height="200%">
          <feGaussianBlur in="SourceAlpha" stdDeviation="2.4" />
          <feOffset dx="0" dy="2" result="offsetBlur" />
          <feComponentTransfer>
            <feFuncA type="linear" slope="0.45" />
          </feComponentTransfer>
          <feMerge>
            <feMergeNode />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
        <radialGradient id={glowId}>
          <stop offset="0%" stopColor={stroke} stopOpacity="0.55" />
          <stop offset="100%" stopColor={stroke} stopOpacity="0" />
        </radialGradient>
      </defs>

      {showGrid && (
        <g>
          {gridLines.map((g, i) => (
            <line
              key={i}
              x1={g.x1}
              y1={g.y1}
              x2={g.x2}
              y2={g.y2}
              stroke={gridColor}
              strokeWidth={1}
              strokeDasharray="3 5"
              className="cpu-grid-line"
            />
          ))}
        </g>
      )}

      {showAxes && (
        <g>
          {yTicks.map((tk, i) => (
            <text
              key={`y${i}`}
              className="cpu-grid-y-text"
              x={Math.max(0, paddingLeft - 6)}
              y={tk.y + 4}
              textAnchor="end"
              fontSize={10.5}
            >
              {tk.label}
            </text>
          ))}
          {xTicks.map((tk, i) => (
            <text
              key={`x${i}`}
              className="cpu-grid-x-text"
              x={tk.x}
              y={paddingTop + drawHeight + 14}
              textAnchor="middle"
              fontSize={10.5}
            >
              {tk.label}
            </text>
          ))}
        </g>
      )}

      {areaPath && <path d={areaPath} fill={`url(#${gradId})`} stroke="none" />}
      <polyline
        points={pointsStr}
        fill="none"
        stroke={stroke}
        strokeWidth={strokeWidth}
        strokeLinecap="round"
        strokeLinejoin="round"
        filter={`url(#${shadowId})`}
      />
      {showMarker && lastPoint && (
        <>
          <circle cx={lastPoint[0]} cy={lastPoint[1]} r={markerRadius * 3} fill={`url(#${glowId})`}>
            <animate attributeName="r" values={`${markerRadius * 2.4};${markerRadius * 3.4};${markerRadius * 2.4}`} dur="2.6s" repeatCount="indefinite" />
          </circle>
          <circle cx={lastPoint[0]} cy={lastPoint[1]} r={markerRadius + 1.5} fill={stroke} fillOpacity={0.25} />
          <circle cx={lastPoint[0]} cy={lastPoint[1]} r={markerRadius} fill={stroke} stroke="#fff" strokeWidth={1.5} />
        </>
      )}

      {showTooltip && hoverIdx >= 0 && pointsArr[hoverIdx] && (
        <g>
          <line
            className="cpu-grid-h-line"
            x1={pointsArr[hoverIdx][0]}
            x2={pointsArr[hoverIdx][0]}
            y1={paddingTop}
            y2={paddingTop + drawHeight}
            stroke={stroke}
            strokeOpacity={0.45}
            strokeWidth={1}
            strokeDasharray="3 4"
          />
          <circle cx={pointsArr[hoverIdx][0]} cy={pointsArr[hoverIdx][1]} r={5} fill={stroke} fillOpacity={0.25} />
          <circle cx={pointsArr[hoverIdx][0]} cy={pointsArr[hoverIdx][1]} r={3.5} fill={stroke} stroke="#fff" strokeWidth={1.5} />
          <rect
            x={tooltipX}
            y={paddingTop + 2}
            width={tooltipPillWidth}
            height={18}
            rx={9}
            ry={9}
            className="cpu-tooltip-pill"
            fill={stroke}
            fillOpacity={0.92}
          />
          <text
            className="cpu-tooltip-text"
            x={tooltipX + tooltipPillWidth / 2}
            y={paddingTop + 14}
            textAnchor="middle"
            fontSize={11}
            fontWeight={600}
            fill="#fff"
          >
            {hoverText}
          </text>
        </g>
      )}
    </svg>
  );
}
