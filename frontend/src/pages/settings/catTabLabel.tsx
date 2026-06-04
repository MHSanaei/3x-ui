import type { ReactNode } from 'react';
import { Tooltip } from 'antd';

/* Builds a settings category tab label: icon + text on desktop, and on
   mobile just the icon with the text moved into a tooltip — mirroring the
   old top tab bar's icons-only behaviour. */
export function catTabLabel(icon: ReactNode, text: ReactNode, iconsOnly: boolean): ReactNode {
  if (iconsOnly) {
    return <Tooltip title={text}>{icon}</Tooltip>;
  }
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
      {icon}
      <span>{text}</span>
    </span>
  );
}
