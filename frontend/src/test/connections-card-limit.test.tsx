import { describe, expect, it, vi } from 'vitest';
import { screen } from '@testing-library/react';

import type { ComponentProps } from 'react';
import { Status } from '@/models/status';
import ConnectionsCard from '@/pages/index/ConnectionsCard';
import { renderWithProviders } from './test-utils';

const { sparkline } = vi.hoisted(() => ({
  sparkline: vi.fn<(props: Record<string, unknown>) => null>(() => null),
}));
vi.mock('@/components/viz', () => ({ Sparkline: sparkline }));

const base = {
  tcp: [100, 274],
  udp: [10, 18],
  conntrack: [1000, 6144],
  labels: ['1', '2'],
  isMobile: false,
};

describe('ConnectionsCard conntrack limit', () => {
  it('labels conntrack separately from sockets and includes its limit in the chart range', () => {
    renderWithProviders(
      <ConnectionsCard
        {...base}
        status={
          new Status({ tcpCount: 274, udpCount: 18, conntrackCount: 6144, conntrackMax: 7680 })
        }
      />,
    );
    expect(screen.getByText('6,144 / 7,680')).toBeTruthy();
    const props = sparkline.mock.calls.at(-1)?.[0] as unknown as ComponentProps<
      typeof import('@/components/viz').Sparkline
    >;
    expect(props.data).toEqual(base.tcp);
    expect(props.data2).toEqual(base.udp);
    expect(props.data3).toEqual(base.conntrack);
    expect(props.valueMax).toBeGreaterThan(7680);
    expect(props.referenceLines).toContainEqual(
      expect.objectContaining({ y: 7680, label: 'Conntrack limit 7,680' }),
    );
  });

  it('hides conntrack and its boundary when the host does not expose a limit', () => {
    renderWithProviders(
      <ConnectionsCard {...base} status={new Status({ tcpCount: 274, udpCount: 18 })} />,
    );
    const props = sparkline.mock.calls.at(-1)?.[0] as unknown as ComponentProps<
      typeof import('@/components/viz').Sparkline
    >;
    expect(props.data3).toBeUndefined();
    expect(props.valueMax).toBeNull();
    expect(props.referenceLines).toHaveLength(2);
  });

  it('keeps counts above a lowered limit visible', () => {
    renderWithProviders(
      <ConnectionsCard
        {...base}
        conntrack={[8000]}
        status={new Status({ conntrackCount: 8000, conntrackMax: 7680 })}
      />,
    );
    const props = sparkline.mock.calls.at(-1)?.[0] as unknown as ComponentProps<
      typeof import('@/components/viz').Sparkline
    >;
    expect(props.valueMax).toBeGreaterThan(8000);
  });
});
