import { useId, useMemo } from 'react';
import {
  Area,
  AreaChart,
  CartesianGrid,
  ReferenceDot,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
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

const DEFAULT_MIN_COLOR = '#52c41a';
const DEFAULT_MAX_COLOR = '#fa541c';

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

export default function Sparkline({
  data,
  data2 = [],
  data3 = [],
  stroke2 = '#722ed1',
  stroke3 = '#a0d911',
  name1,
  name2,
  name3,
  labels = [],
  height = 80,
  stroke = '#008771',
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
}: SparklineProps) {
  const reactId = useId();
  const safeId = reactId.replace(/[^a-zA-Z0-9]/g, '');
  const gradId = `spkGrad-${safeId}`;
  const gradId2 = `spkGrad2-${safeId}`;
  const gradId3 = `spkGrad3-${safeId}`;
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

  const yTicks = useMemo(() => {
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

  const xTickIndexes = useMemo(() => {
    if (!showAxes || points.length === 0) return undefined;
    const m = Math.max(2, tickCountX);
    return Array.from({ length: m }, (_, i) => Math.round((i * (points.length - 1)) / (m - 1)));
  }, [showAxes, tickCountX, points.length]);

  const fmtTooltip = tooltipFormatter ?? yFormatter;

  const extremaPoints = useMemo(() => {
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

  return (
    <div className="sparkline-container">
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
      <ResponsiveContainer width="100%" height={height} className="sparkline-svg">
        <AreaChart
          data={points}
          margin={{
            top: showAxes ? 14 : 6,
            right: showAxes ? 12 : 6,
            bottom: showAxes ? 26 : 4,
            left: 4,
          }}
        >
          <defs>
            <linearGradient id={gradId} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={stroke} stopOpacity={fillOpacity} />
              <stop offset="100%" stopColor={stroke} stopOpacity={0} />
            </linearGradient>
            <linearGradient id={gradId2} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={stroke2} stopOpacity={fillOpacity} />
              <stop offset="100%" stopColor={stroke2} stopOpacity={0} />
            </linearGradient>
            <linearGradient id={gradId3} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={stroke3} stopOpacity={fillOpacity} />
              <stop offset="100%" stopColor={stroke3} stopOpacity={0} />
            </linearGradient>
          </defs>
          {showGrid && (
            <CartesianGrid stroke="rgba(128, 128, 140, 0.35)" strokeDasharray="3 4" vertical={false} />
          )}
          <XAxis
            dataKey="label"
            hide={!showAxes}
            tick={{ fontSize: 10, fill: 'var(--ant-color-text-tertiary)' }}
            axisLine={false}
            tickLine={false}
            tickMargin={14}
            interval={0}
            ticks={xTickIndexes?.map((i) => points[i]?.label).filter(Boolean) as string[] | undefined}
          />
          <YAxis
            domain={yDomain}
            hide={!showAxes}
            tick={{ fontSize: 10, fill: 'var(--ant-color-text-tertiary)', dx: -4 }}
            axisLine={false}
            tickLine={false}
            tickMargin={8}
            tickFormatter={yFormatter}
            ticks={yTicks}
            width={56}
          />
          {showTooltip && (
            <Tooltip
              cursor={{ stroke: 'var(--ant-color-border)', strokeDasharray: '2 4' }}
              contentStyle={{
                background: 'var(--ant-color-bg-elevated)',
                border: '1px solid var(--ant-color-border-secondary)',
                borderRadius: 6,
                fontSize: 12,
                padding: '6px 10px',
                boxShadow: '0 4px 14px rgba(0, 0, 0, 0.12)',
              }}
              labelStyle={{ color: 'var(--ant-color-text-tertiary)', marginBottom: 4, fontSize: 11 }}
              itemStyle={{ color: 'var(--ant-color-text)', padding: 0, fontWeight: 500 }}
              formatter={(v, name) => [fmtTooltip(Number(v) || 0), multiSeries && typeof name === 'string' ? name : '']}
              labelFormatter={(label) => (tooltipLabelFormatter ? tooltipLabelFormatter(String(label)) : String(label))}
              separator={multiSeries ? ': ' : ''}
            />
          )}
          {referenceLines?.map((rl, idx) => (
            <ReferenceLine
              key={`ref-${idx}-${rl.y}`}
              y={rl.y}
              stroke={rl.color || stroke}
              strokeDasharray={rl.dash || '5 4'}
              strokeWidth={1.4}
              label={rl.label ? {
                value: rl.label,
                position: 'insideTopRight',
                fill: rl.color || stroke,
                fontSize: 10,
                fontWeight: 600,
              } : undefined}
              ifOverflow="extendDomain"
            />
          ))}
          {extremaPoints && (
            <>
              <ReferenceDot
                x={extremaPoints.max.label}
                y={extremaPoints.max.value}
                r={4.5}
                fill={maxColor}
                stroke="var(--ant-color-bg-elevated)"
                strokeWidth={2}
                ifOverflow="extendDomain"
              />
              <ReferenceDot
                x={extremaPoints.min.label}
                y={extremaPoints.min.value}
                r={4.5}
                fill={minColor}
                stroke="var(--ant-color-bg-elevated)"
                strokeWidth={2}
                ifOverflow="extendDomain"
              />
            </>
          )}
          <Area
            type="monotone"
            dataKey="value"
            name={multiSeries ? name1 : undefined}
            stroke={stroke}
            strokeWidth={strokeWidth}
            fill={`url(#${gradId})`}
            dot={false}
            activeDot={showMarker ? { r: markerRadius, fill: stroke, strokeWidth: 0 } : false}
            isAnimationActive={false}
          />
          {hasSeries2 && (
            <Area
              type="monotone"
              dataKey="value2"
              name={name2}
              stroke={stroke2}
              strokeWidth={strokeWidth}
              fill={`url(#${gradId2})`}
              dot={false}
              activeDot={showMarker ? { r: markerRadius, fill: stroke2, strokeWidth: 0 } : false}
              isAnimationActive={false}
            />
          )}
          {hasSeries3 && (
            <Area
              type="monotone"
              dataKey="value3"
              name={name3}
              stroke={stroke3}
              strokeWidth={strokeWidth}
              fill={`url(#${gradId3})`}
              dot={false}
              activeDot={showMarker ? { r: markerRadius, fill: stroke3, strokeWidth: 0 } : false}
              isAnimationActive={false}
            />
          )}
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
