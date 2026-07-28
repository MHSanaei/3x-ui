import { fireEvent } from '@testing-library/react';
import dayjs from 'dayjs';
import type { Dayjs } from 'dayjs';
import { describe, expect, it, vi } from 'vitest';

import DateTimePicker from '@/components/form/DateTimePicker';
import { renderWithProviders } from './test-utils';

function openPicker(): void {
  const input = document.querySelector('.ant-picker input');
  if (!input) throw new Error('picker input not rendered');
  fireEvent.mouseDown(input);
  fireEvent.click(input);
}

function clickDayCell(title: string): void {
  const cell = document.querySelector(`.ant-picker-cell[title="${title}"] .ant-picker-cell-inner`);
  if (!cell) throw new Error(`day cell ${title} not rendered`);
  fireEvent.click(cell);
}

describe('DateTimePicker', () => {
  it('commits a clicked calendar date without an OK press', () => {
    const onChange = vi.fn<(next: Dayjs | null) => void>();
    renderWithProviders(<DateTimePicker value={null} onChange={onChange} />);

    openPicker();
    const tomorrow = dayjs().add(1, 'day').format('YYYY-MM-DD');
    clickDayCell(tomorrow);

    expect(onChange).toHaveBeenCalled();
    const committed = onChange.mock.calls.at(-1)?.[0];
    expect(committed?.format('YYYY-MM-DD HH:mm:ss')).toBe(`${tomorrow} 00:00:00`);
  });

  it('renders no OK confirm button in the picker footer', () => {
    renderWithProviders(<DateTimePicker value={null} onChange={vi.fn()} />);

    openPicker();

    expect(document.querySelector('.ant-picker-ok')).toBeNull();
  });
});
