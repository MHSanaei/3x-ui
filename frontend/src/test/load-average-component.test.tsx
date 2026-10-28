import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { Status } from '@/models/status';
import LoadAverage from '@/pages/index/LoadAverage';

describe('unavailable and scoped load readings', () => {
  it.each([
    { loads: [], expected: ['—', '—', '—'] },
    { loads: [0], expected: ['0.00', '—', '—'] },
    { loads: [1.2, -1, Infinity], expected: ['1.20', '—', '—'] },
    { loads: [NaN, 0, 1], expected: ['—', '0.00', '1.00'] },
  ])('does not invent missing windows or show invalid values: $loads', ({ loads, expected }) => {
    render(<LoadAverage status={new Status({ loads, logicalPro: 4 })} />);
    const values = screen.getAllByRole('definition');
    expect(values.map((value) => value.textContent)).toEqual(expected);
    for (const value of values.filter((value) => value.textContent === '—')) {
      expect(value.getAttribute('aria-label')).toContain('Unavailable');
      expect(value.closest('[data-severity]')?.getAttribute('data-severity')).toBe('unavailable');
    }
  });

  it.each(['container', 'unknown', 'future-scope'])(
    'preserves raw loads but avoids comparing %s CPUs to host load',
    async (resourceScope) => {
      render(
        <LoadAverage status={new Status({ resourceScope, logicalPro: 2, loads: [10, 20, 30] })} />,
      );
      expect(screen.getByLabelText('1m: 10.00 — Unavailable').textContent).toBe('10.00');
      expect(
        screen
          .getByRole('img', { name: 'Load Average (1m): Unavailable' })
          .getAttribute('data-severity'),
      ).toBe('unavailable');
      fireEvent.focus(screen.getByRole('button', { name: 'Load Average' }));
      await waitFor(() =>
        expect(screen.getByRole('tooltip').textContent).toContain('different scopes'),
      );
    },
  );

  it('shows a keyboard-accessible explanation without treating load as CPU utilization', async () => {
    render(<LoadAverage status={new Status({ logicalPro: 4, loads: [0, 4, 5] })} />);
    fireEvent.focus(screen.getByRole('button', { name: 'Load Average' }));
    await waitFor(() =>
      expect(screen.getByRole('tooltip').textContent).toContain('not CPU utilization'),
    );
    expect(screen.getByLabelText('1m: 0.00 — Healthy')).toBeTruthy();
    expect(screen.getByLabelText('5m: 4.00 — Near capacity')).toBeTruthy();
    expect(screen.getByLabelText('15m: 5.00 — Above capacity')).toBeTruthy();
  });

  it.each([0, -1, NaN, Infinity])(
    'keeps valid readings neutral when CPU capacity is %s',
    (logicalPro) => {
      render(<LoadAverage status={new Status({ logicalPro, loads: [1, 2, 3] })} />);
      expect(screen.getByLabelText('1m: 1.00 — Unavailable').textContent).toBe('1.00');
      expect(screen.queryByLabelText(/Healthy|Above capacity/)).toBeNull();
    },
  );

  it('honors unavailable load metadata even if a numeric placeholder was sent', () => {
    render(
      <LoadAverage
        status={new Status({ logicalPro: 4, loads: [0, 0, 0], unavailableMetrics: ['loads'] })}
      />,
    );
    expect(screen.getAllByRole('definition').map((value) => value.textContent)).toEqual([
      '—',
      '—',
      '—',
    ]);
  });

  it('does not use affinity-limited CPU capacity for host load', () => {
    render(
      <LoadAverage
        status={
          new Status({ logicalPro: 2, loads: [10, 20, 30], unavailableMetrics: ['loadCapacity'] })
        }
      />,
    );
    expect(screen.getByLabelText('1m: 10.00 — Unavailable').textContent).toBe('10.00');
  });
});
