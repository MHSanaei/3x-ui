import type { CSSProperties } from 'react';

/** Shared class for live speed tags (also defined in styles/utils.css). */
export const SPEED_TAG_CLASS_NAME = 'speed-tag' as const;

/**
 * Fixed layout for up/down rate tags.
 * Live B/KB/MB text changes must not reflow the Speed column (issue #5912).
 */
export const SPEED_TAG_STYLE = {
  width: '148px',
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  textAlign: 'center',
  whiteSpace: 'nowrap',
  fontVariantNumeric: 'tabular-nums',
  marginInlineEnd: 0,
  boxSizing: 'border-box',
} as const satisfies CSSProperties;

/** Ant Design table column width aligned with SPEED_TAG_STYLE.width + cell padding. */
export const SPEED_COLUMN_WIDTH = 160 as const;
