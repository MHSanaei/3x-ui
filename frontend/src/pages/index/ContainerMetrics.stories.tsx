import type { Meta, StoryObj } from '@storybook/react-vite';
import { DatabaseOutlined } from '@ant-design/icons';
import { expect, within } from 'storybook/test';

import { Status } from '@/models/status';
import VitalTile from './VitalTile';
import SystemStrip from './SystemStrip';
import './IndexPage.css';

function ContainerMetricsPreview({ unavailable = false }: { unavailable?: boolean }) {
  const status = new Status({
    resourceScope: 'container',
    unavailableMetrics: unavailable ? ['swap', 'uptime'] : [],
    swap: { current: 19357696, total: 536870912 },
    uptime: 52624,
    appStats: { mem: 104857600, threads: 37, uptime: 50000 },
  });
  return (
    <div style={{ maxWidth: 1000, display: 'grid', gap: 16 }}>
      <div style={{ width: 340 }}>
        <VitalTile
          icon={<DatabaseOutlined />}
          label="SWAP"
          available={status.metricAvailable('swap')}
          percent={status.swap.percent}
          statusColor={status.swap.color}
          detail="18.46 MiB / 512 MiB"
          footLeft="Container"
          footRight="2 vCPU"
          data={unavailable ? [] : [3.5, 3.6, 3.6]}
          isMobile={false}
        />
      </div>
      <SystemStrip status={status} showIp={false} onToggleIp={() => undefined} />
    </div>
  );
}

const meta = {
  title: 'Monitoring/Container metrics',
  component: ContainerMetricsPreview,
  tags: ['autodocs'],
  parameters: { layout: 'padded' },
} satisfies Meta<typeof ContainerMetricsPreview>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Measured: Story = {
  args: { unavailable: false },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByText('18.46 MiB / 512 MiB')).toBeVisible();
    await expect(canvas.getByText('CT')).toBeVisible();
    await expect(canvas.queryByText('OS')).not.toBeInTheDocument();
  },
};

export const Unavailable: Story = {
  args: { unavailable: true },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.queryByText('18.46 MiB / 512 MiB')).not.toBeInTheDocument();
    await expect(canvas.queryByText('%')).not.toBeInTheDocument();
    await expect(canvas.getAllByText('—').length).toBeGreaterThan(0);
  },
};
