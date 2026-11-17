import { describe, expect, it, vi } from 'vitest';
import { screen } from '@testing-library/react';

import { Status } from '@/models/status';
import ConnectionsCard from '@/pages/index/ConnectionsCard';
import { renderWithProviders } from './test-utils';

const { sparkline } = vi.hoisted(() => ({
  sparkline: vi.fn<(props: Record<string, unknown>) => null>(() => null),
}));
vi.mock('@/components/viz', () => ({ Sparkline: sparkline }));

describe('ConnectionsCard', () => {
  it('plots TCP and UDP with a relative scale even when conntrack reaches capacity', () => {
    const tcp = [100, 274];
    const udp = [10, 18];
    renderWithProviders(
      <ConnectionsCard
        tcp={tcp}
        udp={udp}
        labels={['1', '2']}
        isMobile={false}
        status={
          new Status({ tcpCount: 274, udpCount: 18, conntrackCount: 8000, conntrackMax: 7680 })
        }
      />,
    );
    expect(screen.getByText('292')).toBeTruthy();
    expect(screen.queryByText('Conntrack usage')).toBeNull();
    const props = sparkline.mock.calls.at(-1)?.[0];
    expect(props?.data).toEqual(tcp);
    expect(props?.data2).toEqual(udp);
    expect(props?.data3).toBeUndefined();
    expect(props?.valueMax).toBeNull();
    expect(props?.referenceLines).toEqual([
      expect.objectContaining({ y: 18 }),
      expect.objectContaining({ y: 274 }),
    ]);
  });
});
