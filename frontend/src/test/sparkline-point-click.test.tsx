import { act, render } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';

import { Sparkline } from '@/components/viz';

interface FakePlot {
  cursor: { idx: number | null };
  over: HTMLElement;
}
const plots = vi.hoisted(() => [] as FakePlot[]);

vi.mock('uplot', () => ({
  default: class {
    static paths = { spline: () => undefined };
    static pxRatio = 1;
    cursor = { idx: null };
    over = document.createElement('div');
    constructor(opts: { hooks: { init: ((u: unknown) => void)[] } }) {
      plots.push(this as unknown as FakePlot);
      opts.hooks.init.forEach((hook) => hook(this));
    }
    setData() {}
    setSize() {}
    redraw() {}
    destroy() {}
  },
}));

beforeEach(() => {
  plots.length = 0;
});

// uPlot reports -1 or null off the data, and neither is an index a caller can look up.
it.each([
  [2, [[2]]],
  [0, [[0]]],
  [-1, []],
  [null, []],
])('with the cursor at %s, a click reports %j', async (idx, calls) => {
  const onPointClick = vi.fn();
  render(<Sparkline data={[1, 2, 3]} onPointClick={onPointClick} />);
  await act(async () => {});
  plots[0].cursor.idx = idx;
  plots[0].over.click();
  expect(onPointClick.mock.calls).toEqual(calls);
});
