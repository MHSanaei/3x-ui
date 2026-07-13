import { render } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { ClientSpeedTag } from '@/components/clients/ClientSpeedTag';
import {
  SPEED_COLUMN_WIDTH,
  SPEED_TABLE_CELL_INLINE_PADDING,
  SPEED_TAG_CLASS_NAME,
  SPEED_TAG_STYLE,
  SPEED_TAG_WIDTH,
} from '@/components/utility/speedTagStyle';
import { InboundSpeedTag } from '@/pages/inbounds/list/InboundSpeedTag';
import { SizeFormatter } from '@/utils';
import '@/styles/utils.css';

const SMALL_RATE = { up: 512, down: 1023 } as const;
const LARGE_RATE = { up: 12_340_000, down: 99_990_000 } as const;
const FORMATTER_BOUNDARY_RATE = {
  up: SizeFormatter.ONE_GB - 1,
  down: SizeFormatter.ONE_GB - 1,
} as const;
const FORMATTER_BOUNDARY_NATURAL_WIDTH = 192.765625;

function firstTag(): HTMLElement {
  const tag = document.querySelector('.ant-tag');
  if (!(tag instanceof HTMLElement)) {
    throw new Error('expected an Ant Design Tag in the document');
  }
  return tag;
}

function expectFluidTag(tag: HTMLElement) {
  expect(tag.classList.contains(SPEED_TAG_CLASS_NAME)).toBe(false);
  expect(tag.style.width).toBe('');
}

function expectStableTableTag(tag: HTMLElement) {
  const style = getComputedStyle(tag);
  expect(tag.classList.contains(SPEED_TAG_CLASS_NAME)).toBe(true);
  expect(style.width).toBe(`${SPEED_TAG_WIDTH}px`);
  expect(style.display).toBe(SPEED_TAG_STYLE.display);
  expect(style.justifyContent).toBe(SPEED_TAG_STYLE.justifyContent);
  expect(style.alignItems).toBe(SPEED_TAG_STYLE.alignItems);
  expect(style.textAlign).toBe(SPEED_TAG_STYLE.textAlign);
  expect(style.whiteSpace).toBe(SPEED_TAG_STYLE.whiteSpace);
  expect(style.fontVariantNumeric).toBe(SPEED_TAG_STYLE.fontVariantNumeric);
  expect(style.overflow).toBe('hidden');
  expect(style.textOverflow).toBe('ellipsis');
}

describe('stable table speed tags (issue #5912)', () => {
  it('scopes ClientSpeedTag stable sizing to table cells', () => {
    const { rerender } = render(<ClientSpeedTag speed={SMALL_RATE} />);
    expectFluidTag(firstTag());

    rerender(<ClientSpeedTag speed={LARGE_RATE} tableCell />);
    expectStableTableTag(firstTag());
  });

  it('scopes InboundSpeedTag stable sizing to table cells', () => {
    const { rerender } = render(<InboundSpeedTag speed={SMALL_RATE} />);
    expectFluidTag(firstTag());

    rerender(<InboundSpeedTag speed={LARGE_RATE} withTooltip tableCell />);
    expectStableTableTag(firstTag());
  });

  it('fits the widest formatter rollover and includes small-cell padding', () => {
    render(
      <>
        <ClientSpeedTag speed={FORMATTER_BOUNDARY_RATE} tableCell />
        <InboundSpeedTag speed={FORMATTER_BOUNDARY_RATE} tableCell />
      </>,
    );
    const tags = Array.from(document.querySelectorAll<HTMLElement>('.ant-tag'));
    expect(tags).toHaveLength(2);
    expect(tags[0]?.textContent).toBe('↑ 1024.00 MB/s / ↓ 1024.00 MB/s');
    expect(tags[1]?.textContent).toBe(tags[0]?.textContent);
    for (const tag of tags) expectStableTableTag(tag);

    expect(SPEED_TAG_WIDTH).toBeGreaterThan(FORMATTER_BOUNDARY_NATURAL_WIDTH);
    expect(SPEED_COLUMN_WIDTH).toBe(SPEED_TAG_WIDTH + SPEED_TABLE_CELL_INLINE_PADDING * 2);
  });
});
