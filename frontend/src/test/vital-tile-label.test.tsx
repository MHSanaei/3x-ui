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
});
