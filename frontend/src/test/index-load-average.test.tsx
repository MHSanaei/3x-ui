import { screen, render, within, waitFor } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';

import { Status } from '@/models/status';
import { ThemeProvider } from '@/hooks/useTheme';
import { HttpUtil, Msg } from '@/utils';
import IndexPage from '@/pages/index/IndexPage';

const feed = vi.hoisted(() => ({ status: null as Status | null }));
vi.mock('uplot', () => ({
  default: class {
    static paths = { spline: () => undefined };
    setData() {}
    setSize() {}
    redraw() {}
    destroy() {}
  },
}));
vi.mock('@/api/queries/useStatusQuery', () => ({
  useStatusQuery: () => ({ status: feed.status, fetched: true, fetchError: '', refresh: vi.fn() }),
}));
vi.mock('@/layouts/AppSidebar', () => ({ default: () => null }));
vi.mock('@/components/sponsor/SponsorSlot', () => ({ default: () => null }));

beforeEach(() => {
  feed.status = new Status({ cpu: 72, cpuCores: 2, logicalPro: 4, loads: [1.2, 3.6, 4.8] });
  vi.mocked(HttpUtil.get).mockImplementation(
    async (url) =>
      new Msg(
        true,
        '',
        url.includes('/history/cpu/')
          ? [
              { t: 1, v: 30 },
              { t: 2, v: 50 },
            ]
          : [],
      ),
  );
});

it('adds independently colored loads only to CPU and refreshes with the existing status and history', async () => {
  const { container, rerender } = render(<IndexPage />, { wrapper: ThemeProvider });
  const title = screen.getByRole('button', { name: 'Load Average' });
  const tile = title.closest('.ov-tile')!;
  const cpu = within(tile as HTMLElement);
  expect(cpu.getByText('72.0')).toBeTruthy();
  expect(cpu.getByText('2 Cores / 4T · 0.00 MHz')).toBeTruthy();
  expect(cpu.getByLabelText('1m: 1.20 — Healthy').textContent).toBe('1.20');
  expect(cpu.getByLabelText('5m: 3.60 — Near capacity').textContent).toBe('3.60');
  expect(cpu.getByLabelText('15m: 4.80 — Above capacity').textContent).toBe('4.80');
  expect(
    Array.from(tile.querySelectorAll<HTMLElement>('.ov-load-reading')).map((reading) =>
      reading.style.getPropertyValue('--load-color'),
    ),
  ).toEqual(['#22c55e', '#f97316', '#ef4444']);
  expect(
    cpu.getByRole('img', { name: 'Load Average (1m): Healthy' }).getAttribute('data-severity'),
  ).toBe('healthy');
  await waitFor(() => expect(cpu.getByText('avg 51%').textContent).toBe('avg 51%'));
  expect(cpu.getByText('peak 72%')).toBeTruthy();
  expect(cpu.getByRole('img', { name: /CPU: 72%/ })).toBeTruthy();
  expect(container.querySelectorAll('.ov-vitals .ov-tile')).toHaveLength(4);
  expect(container.querySelectorAll('.ov-load')).toHaveLength(1);
  const requestCount = vi.mocked(HttpUtil.get).mock.calls.length;
  feed.status = new Status({ cpu: 80, logicalPro: 4, loads: [4.0004, 0, 3.6] });
  rerender(<IndexPage />);
  expect(cpu.getByText('80.0')).toBeTruthy();
  expect(cpu.getByLabelText('1m: 4.00 — Above capacity').textContent).toBe('4.00');
  expect(cpu.getByLabelText('5m: 0.00 — Healthy').textContent).toBe('0.00');
  expect(
    cpu
      .getByRole('img', { name: 'Load Average (1m): Above capacity' })
      .getAttribute('data-severity'),
  ).toBe('aboveCapacity');
  expect(cpu.getByText('avg 58%')).toBeTruthy();
  expect(cpu.getByRole('img', { name: /CPU: 80%/ })).toBeTruthy();
  expect(vi.mocked(HttpUtil.get).mock.calls).toHaveLength(requestCount);
});
