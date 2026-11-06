import { useMemo, useRef, useState } from 'react';
import type { PointerEvent as ReactPointerEvent } from 'react';

import type { LogCount } from '@/generated/types';
import { countryBin, countryName } from '@/lib/logs/countryScale';
import { LanguageManager } from '@/utils';
import { activateOnKey } from '@/utils/a11y';
import mapData from './worldMapData.json';

const { width, height, shapes, markers } = mapData as {
  width: number;
  height: number;
  shapes: Record<string, string>;
  markers: Record<string, number[]>;
};

const MARKER_RADIUS = 5;

interface WorldMapProps {
  countries: LogCount[];
  activeCode: string;
  onSelect: (code: string) => void;
  label: string;
}

interface Hover {
  code: string;
  x: number;
  y: number;
  flip: boolean;
}

export default function WorldMap({ countries, activeCode, onSelect, label }: WorldMapProps) {
  const language = LanguageManager.getLanguage();
  const wrapper = useRef<HTMLDivElement>(null);
  const [hover, setHover] = useState<Hover | null>(null);

  const counts = useMemo(() => new Map(countries.map((c) => [c.value, c.count])), [countries]);
  const peak = countries[0]?.count ?? 0;
  const lowest = countries.at(-1)?.count ?? 0;

  // Only countries with data take focus and clicks, so keyboard users tab through what matters.
  const interaction = (code: string, count: number | undefined) => {
    if (!count) return { 'aria-hidden': true };
    return {
      role: 'button',
      tabIndex: 0,
      'aria-label': `${countryName(code, language)}: ${count.toLocaleString()}`,
      'aria-pressed': activeCode === code,
      onClick: () => onSelect(code),
      onKeyDown: activateOnKey(() => onSelect(code)),
    };
  };

  const className = (code: string, count: number | undefined) => {
    const classes = count ? [`logs-map-bin-${countryBin(count, lowest, peak)}`] : ['logs-map-land'];
    if (activeCode === code) classes.push('is-active');
    return classes.join(' ');
  };

  const trackPointer = (event: ReactPointerEvent<SVGSVGElement>) => {
    const code = (event.target as Element).getAttribute('data-code');
    const box = wrapper.current?.getBoundingClientRect();
    if (!code || !box) {
      setHover(null);
      return;
    }
    const x = event.clientX - box.left;
    setHover({ code, x, y: event.clientY - box.top, flip: x > box.width * 0.7 });
  };

  return (
    <div ref={wrapper} className="logs-map">
      <svg
        viewBox={`0 0 ${width} ${height}`}
        role="group"
        aria-label={label}
        onPointerMove={trackPointer}
        onPointerLeave={() => setHover(null)}
      >
        {Object.entries(shapes).map(([code, d]) => {
          const count = counts.get(code);
          return (
            <path
              key={code}
              d={d}
              data-code={code}
              className={className(code, count)}
              {...interaction(code, count)}
            />
          );
        })}
        {Object.entries(markers)
          .filter(([code]) => counts.has(code))
          .map(([code, [cx, cy]]) => {
            const count = counts.get(code);
            return (
              <circle
                key={code}
                cx={cx}
                cy={cy}
                r={MARKER_RADIUS}
                data-code={code}
                className={className(code, count)}
                {...interaction(code, count)}
              />
            );
          })}
      </svg>
      {hover && (
        <div
          className={hover.flip ? 'logs-map-tooltip is-flipped' : 'logs-map-tooltip'}
          style={{ left: hover.x, top: hover.y }}
        >
          <bdi>{countryName(hover.code, language)}</bdi>
          <strong>{(counts.get(hover.code) ?? 0).toLocaleString()}</strong>
        </div>
      )}
      <div className="logs-map-legend" aria-hidden>
        <span>{lowest.toLocaleString()}</span>
        {[1, 2, 3, 4, 5].map((bin) => (
          <span key={bin} className="logs-map-swatch">
            <svg viewBox="0 0 10 10" preserveAspectRatio="none">
              <rect width="10" height="10" className={`logs-map-bin-${bin}`} />
            </svg>
          </span>
        ))}
        <span>{peak.toLocaleString()}</span>
      </div>
    </div>
  );
}
