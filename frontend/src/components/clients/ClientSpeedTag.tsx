import { Tag } from 'antd';

import { SizeFormatter } from '@/utils';
import type { ClientSpeedEntry } from '@/hooks/useClients';
import { SPEED_TAG_CLASS_NAME, SPEED_TAG_STYLE } from '@/components/utility/speedTagStyle';

export type { ClientSpeedEntry };

export function isActiveSpeed(speed?: ClientSpeedEntry): speed is ClientSpeedEntry {
  return !!speed && (speed.up > 0 || speed.down > 0);
}

interface ClientSpeedTagProps {
  speed: ClientSpeedEntry;
  tableCell?: boolean;
}

export function ClientSpeedTag({ speed, tableCell = false }: ClientSpeedTagProps) {
  return (
    <Tag
      color="blue"
      className={tableCell ? SPEED_TAG_CLASS_NAME : undefined}
      style={tableCell ? SPEED_TAG_STYLE : undefined}
    >
      ↑ {SizeFormatter.speedFormat(speed.up)}
      {' / '}
      ↓ {SizeFormatter.speedFormat(speed.down)}
    </Tag>
  );
}

export default ClientSpeedTag;
