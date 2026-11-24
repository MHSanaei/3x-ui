import { fireEvent, render, screen } from '@testing-library/react';
import { expect, it, vi } from 'vitest';

import { NotifyTimeField } from '@/pages/settings/NotifyTimeField';

it.each([
  ['@every 45m', '@every 45m'],
  ['@hourly', '0 0 * * * *'],
])('preserves the cadence of %s when switching to Custom', (schedule, expected) => {
  const onChange = vi.fn();
  render(<NotifyTimeField value={schedule} onChange={onChange} ariaLabel="Schedule" />);

  fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Schedule' }));
  fireEvent.click(screen.getByText('Custom (crontab)'));

  expect(screen.getByRole('textbox', { name: 'Custom (crontab)' })).toHaveProperty(
    'value',
    expected,
  );
  expect(onChange).toHaveBeenLastCalledWith(expected);
});
