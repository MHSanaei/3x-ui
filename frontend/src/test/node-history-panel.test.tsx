import { render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import NodeHistoryPanel from '@/pages/nodes/NodeHistoryPanel';
import { HttpUtil, Msg } from '@/utils';

const plots = vi.hoisted(() => [] as { scales: { y: { range: () => [number, number] } } }[]);

vi.mock('uplot', () => ({
  default: class {
    static paths = { spline: () => undefined };
    static pxRatio = 1;
    constructor(opts: (typeof plots)[number]) {
      plots.push(opts);
    }
    setData() {}
    setSize() {}
    redraw() {}
    destroy() {}
  },
}));

// The net series fell through to Sparkline's percentage defaults: a 0-100 scale
// and a "%" label, so 512 KB/s rendered as "512%" far above the chart.
describe('NodeHistoryPanel', () => {
  it('charts net throughput in KB/s on its own scale', async () => {
    const samples: Record<string, number> = {
      cpu: 40,
      mem: 60,
      netUp: 512 * 1024,
      netDown: 200 * 1024,
    };
    vi.spyOn(HttpUtil, 'get').mockImplementation(async (url: string) => {
      const metric = url.split('/').at(-2) ?? '';
      return new Msg(true, '', [{ t: 1_700_000_000, v: samples[metric] }]);
    });

    render(<NodeHistoryPanel node={{ id: 7 }} />);

    // Sparkline updates its chart refs in a passive effect after the DOM commits.
    await waitFor(() => {
      expect(screen.getAllByRole('img')).toHaveLength(4);
      expect(screen.getAllByRole('img').map((el) => el.getAttribute('aria-label'))).toEqual([
        '40%',
        '60%',
        '512',
        '200',
      ]);
      expect(plots.map((p) => p.scales.y.range())).toEqual([
        [0, 100],
        [0, 100],
        [0, 512 * 1.1],
        [0, 200 * 1.1],
      ]);
    });
  });
});
