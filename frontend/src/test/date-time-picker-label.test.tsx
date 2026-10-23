import { act, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import DateTimePicker from '@/components/form/DateTimePicker';
import { setDatepicker } from '@/hooks/useDatepicker';
import { renderWithProviders } from './test-utils';

vi.mock('persian-calendar-suite', () => ({
  PersianDateTimePicker: () => <input type="text" />,
}));

afterEach(() => act(() => setDatepicker('gregorian')));

// Form.Item labels reach the picker only through the id it forwards to its input.
describe.each(['gregorian', 'jalalian'] as const)('DateTimePicker (%s) labelling', (calendar) => {
  it('ties a visible label to the picker input through id', () => {
    act(() => setDatepicker(calendar));
    renderWithProviders(
      <>
        <label htmlFor="expiry">Expiry</label>
        <DateTimePicker id="expiry" value={null} onChange={vi.fn()} />
      </>,
    );

    expect(screen.getByLabelText('Expiry').tagName).toBe('INPUT');
  });

  it('names the picker input from aria-label', () => {
    act(() => setDatepicker(calendar));
    renderWithProviders(<DateTimePicker aria-label="Expiry" value={null} onChange={vi.fn()} />);

    expect(screen.getByRole('textbox', { name: 'Expiry' }).tagName).toBe('INPUT');
  });

  it('describes the picker input from aria-describedby', () => {
    act(() => setDatepicker(calendar));
    renderWithProviders(
      <>
        <span id="expiry-hint">Leave blank to never expire</span>
        <DateTimePicker
          aria-label="Expiry"
          aria-describedby="expiry-hint"
          value={null}
          onChange={vi.fn()}
        />
      </>,
    );

    const input = screen.getByRole('textbox', { name: 'Expiry' });
    expect(input.getAttribute('aria-describedby')).toBe('expiry-hint');
  });
});
