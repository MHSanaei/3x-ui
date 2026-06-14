import { Tag, Tooltip } from 'antd';

import { SizeFormatter } from '@/utils';

import type { InboundSpeedEntry } from './types';

// True when an inbound has live throughput worth showing.
export function isActiveSpeed(speed?: InboundSpeedEntry): speed is InboundSpeedEntry {
  return !!speed && (speed.up > 0 || speed.down > 0);
}

interface InboundSpeedTagProps {
  speed: InboundSpeedEntry;
  withTooltip?: boolean;
}

// Blue "↑ up / ↓ down" rate tag, optionally with a stacked breakdown tooltip.
export function InboundSpeedTag({ speed, withTooltip = false }: InboundSpeedTagProps) {
  const tag = (
    <Tag color="blue">
      ↑ {SizeFormatter.speedFormat(speed.up)}
      {' / '}
      ↓ {SizeFormatter.speedFormat(speed.down)}
    </Tag>
  );
  if (!withTooltip) return tag;
  return (
    <Tooltip
      title={(
        <div>
          <div>↑ {SizeFormatter.speedFormat(speed.up)}</div>
          <div>↓ {SizeFormatter.speedFormat(speed.down)}</div>
        </div>
      )}
    >
      {tag}
    </Tooltip>
  );
}
