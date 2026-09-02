import { fireEvent, screen } from '@testing-library/react';
import dayjs from 'dayjs';
import type { Dayjs } from 'dayjs';
import { afterEach, describe, expect, it, vi } from 'vitest';

import DateTimePicker from '@/components/form/DateTimePicker';
import { setDatepicker } from '@/hooks/useDatepicker';
import { renderWithProviders } from './test-utils';

vi.mock('persian-calendar-suite', () => ({
  PersianDateTimePicker: ({
    maxDate,
    onChange,
  }: {
    maxDate?: Date;
    onChange?: (value: number) => void;
  }) => (
    <>
      <button
        type="button"
        aria-label="Persian date time picker"
        data-testid="persian-date-time-picker"
        data-max-date={maxDate?.toISOString()}
        onClick={() => onChange?.((maxDate?.getTime() ?? 0) + 1000)}
      />
      <button
        type="button"
        aria-label="Persian date time picker (under max)"
        data-testid="persian-date-time-picker-under-max"
        onClick={() => onChange?.((maxDate?.getTime() ?? 0) - 1000)}
      />
    </>
  ),
}));

afterEach(() => setDatepicker('gregorian'));

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

  it('hides the Gregorian clear control and disables dates after maxDate', () => {
    const maxDate = dayjs().add(1, 'day').startOf('day');
    renderWithProviders(
      <DateTimePicker value={maxDate} onChange={vi.fn()} allowClear={false} maxDate={maxDate} />,
    );

    expect(document.querySelector('.ant-picker-clear')).toBeNull();
    openPicker();
    const blockedCell = document.querySelector(
      `.ant-picker-cell[title="${maxDate.add(1, 'day').format('YYYY-MM-DD')}"]`,
    );
    expect(blockedCell?.classList.contains('ant-picker-cell-disabled')).toBe(true);
  });

  it('applies clear and max-date constraints to the Jalali picker', () => {
    setDatepicker('jalalian');
    const maxDate = dayjs('2030-01-02T03:04:05');
    const onChange = vi.fn();
    renderWithProviders(
      <DateTimePicker value={maxDate} onChange={onChange} allowClear={false} maxDate={maxDate} />,
    );

    expect(document.querySelector('.jdp-clear')).toBeNull();
    expect(screen.getByTestId('persian-date-time-picker').dataset.maxDate).toBe(
      maxDate.toDate().toISOString(),
    );

    // Positive control: a value under maxDate passes through unchanged, so
    // the next assertion (clamped, not rejected outright) is actually meaningful.
    fireEvent.click(screen.getByTestId('persian-date-time-picker-under-max'));
    const underCall = onChange.mock.calls.at(-1)?.[0] as Dayjs;
    expect(underCall.format()).toBe(maxDate.subtract(1, 'second').format());

    fireEvent.click(screen.getByTestId('persian-date-time-picker'));
    const overCall = onChange.mock.calls.at(-1)?.[0] as Dayjs;
    expect(overCall.format()).toBe(maxDate.format());
  });
});
