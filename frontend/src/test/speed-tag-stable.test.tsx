import { describe, expect, it } from 'vitest';
import { render } from '@testing-library/react';

import { ClientSpeedTag } from '@/components/clients/ClientSpeedTag';
import { InboundSpeedTag } from '@/pages/inbounds/list/InboundSpeedTag';
import {
  SPEED_COLUMN_WIDTH,
  SPEED_TAG_CLASS_NAME,
  SPEED_TAG_STYLE,
} from '@/components/utility/speedTagStyle';

const SMALL_RATE = { up: 512, down: 1023 } as const;
const LARGE_RATE = { up: 12_340_000, down: 99_990_000 } as const;

function firstTag(): HTMLElement {
  const tag = document.querySelector('.ant-tag');
  if (!(tag instanceof HTMLElement)) {
    throw new Error('expected an Ant Design Tag in the document');
  }
  return tag;
}

function assertStableSpeedTag(tag: HTMLElement) {
  expect(tag.classList.contains(SPEED_TAG_CLASS_NAME)).toBe(true);
  expect(tag.style.width).toBe(SPEED_TAG_STYLE.width);
  expect(tag.style.display).toBe(SPEED_TAG_STYLE.display);
  expect(tag.style.justifyContent).toBe(SPEED_TAG_STYLE.justifyContent);
  expect(tag.style.alignItems).toBe(SPEED_TAG_STYLE.alignItems);
  expect(tag.style.textAlign).toBe(SPEED_TAG_STYLE.textAlign);
  expect(tag.style.whiteSpace).toBe(SPEED_TAG_STYLE.whiteSpace);
  expect(tag.style.fontVariantNumeric).toBe(SPEED_TAG_STYLE.fontVariantNumeric);
}

describe('stable speed tags (issue #5912)', () => {
  it('ClientSpeedTag keeps the same stable class/style for small and large rates', () => {
    // Given a client speed tag rendered with a small rate
    const { rerender } = render(<ClientSpeedTag speed={SMALL_RATE} />);
    const smallTag = firstTag();
    assertStableSpeedTag(smallTag);
    const smallWidth = smallTag.style.width;

    // When the rate jumps to a much longer formatted string
    rerender(<ClientSpeedTag speed={LARGE_RATE} />);
    const largeTag = firstTag();

    // Then the tag still uses the shared stable layout (no width jitter)
    assertStableSpeedTag(largeTag);
    expect(largeTag.style.width).toBe(smallWidth);
  });

  it('InboundSpeedTag keeps the same stable class/style for small and large rates', () => {
    // Given an inbound speed tag rendered with a small rate
    const { rerender } = render(<InboundSpeedTag speed={SMALL_RATE} />);
    const smallTag = firstTag();
    assertStableSpeedTag(smallTag);
    const smallWidth = smallTag.style.width;

    // When the rate jumps to a much longer formatted string
    rerender(<InboundSpeedTag speed={LARGE_RATE} withTooltip />);
    const largeTag = firstTag();

    // Then the tag still uses the shared stable layout (no width jitter)
    assertStableSpeedTag(largeTag);
    expect(largeTag.style.width).toBe(smallWidth);
  });

  it('both tags share the same stable style contract and column width constant', () => {
    render(
      <>
        <ClientSpeedTag speed={LARGE_RATE} />
        <InboundSpeedTag speed={LARGE_RATE} />
      </>,
    );
    const tags = Array.from(document.querySelectorAll('.ant-tag'));
    expect(tags).toHaveLength(2);
    for (const tag of tags) {
      assertStableSpeedTag(tag as HTMLElement);
    }
    expect((tags[0] as HTMLElement).style.width).toBe((tags[1] as HTMLElement).style.width);
    // Column width must be at least the tag width so the table cell does not clip.
    expect(SPEED_COLUMN_WIDTH).toBeGreaterThanOrEqual(Number.parseInt(SPEED_TAG_STYLE.width, 10));
  });
});
