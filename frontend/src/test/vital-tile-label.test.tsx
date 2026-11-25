import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';

vi.mock('@/components/viz', () => ({ Sparkline: () => null }));

import VitalTile from '@/pages/index/VitalTile';

// A screen reader walking the dashboard heard a bare "47.2 %" with no hint it was CPU;
// the tile is now a group named by its label, so the number is read in context.
describe('VitalTile', () => {
  it('names the tile group after its metric so the value is read with it', () => {
    render(
      <VitalTile
        icon={null}
        label="CPU"
        percent={47.2}
        statusColor="#000"
        detail="4 / 8T"
        footLeft="Avg 40%"
        footRight="Peak 90%"
        data={[40, 47.2]}
        isMobile={false}
      />,
    );
    expect(screen.getByRole('group', { name: 'CPU' }).textContent).toContain('47.2%');
  });

  it('isolates Persian metric labels from their numeric values', () => {
    render(
      <VitalTile
        icon={null}
        label="Swap"
        percent={25}
        statusColor="#000"
        detail="1 / 4 GB"
        footLeft={
          <>
            <span>میانگین</span>
            <bdi>18.0%</bdi>
          </>
        }
        footRight={
          <>
            <span>اوج</span>
            <bdi>23%</bdi>
          </>
        }
        data={[18, 23]}
        isMobile={false}
      />,
    );

    const summaries = screen
      .getByRole('group', { name: 'Swap' })
      .querySelectorAll('.ov-tile-foot > span[dir="auto"]');
    expect(summaries).toHaveLength(2);
    expect(summaries[0].querySelector('bdi')?.textContent).toBe('18.0%');
    expect(summaries[1].querySelector('bdi')?.textContent).toBe('23%');
  });
});
