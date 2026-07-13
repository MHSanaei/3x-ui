import type { CSSProperties } from 'react';

export const SPEED_TAG_CLASS_NAME = 'table-speed-tag' as const;
export const SPEED_TAG_WIDTH = 200 as const;
export const SPEED_TABLE_CELL_INLINE_PADDING = 8 as const;

export const SPEED_TAG_STYLE = {
  width: SPEED_TAG_WIDTH,
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  textAlign: 'center',
  whiteSpace: 'nowrap',
  fontVariantNumeric: 'tabular-nums',
  marginInlineEnd: 0,
  boxSizing: 'border-box',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
} as const satisfies CSSProperties;

export const SPEED_COLUMN_WIDTH = SPEED_TAG_WIDTH + SPEED_TABLE_CELL_INLINE_PADDING * 2;
