import { cloneElement, isValidElement, type ReactElement, type ReactNode } from 'react';
import { Tooltip } from 'antd';

/* Builds a settings category tab label: icon + text on desktop, and on
   mobile just the icon with the text moved into a tooltip — mirroring the
   old top tab bar's icons-only behaviour. */
export function catTabLabel(icon: ReactNode, text: ReactNode, iconsOnly: boolean): ReactNode {
  if (iconsOnly) {
    // Callers hide the icon for the desktop label; here it is the tab's only name.
    const labelledIcon =
      typeof text === 'string' && isValidElement(icon)
        ? cloneElement(icon as ReactElement<{ 'aria-label'?: string; 'aria-hidden'?: boolean }>, {
            'aria-label': text,
            'aria-hidden': undefined,
          })
        : icon;
    return <Tooltip title={text}>{labelledIcon}</Tooltip>;
  }
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
      {icon}
      <span>{text}</span>
    </span>
  );
}
