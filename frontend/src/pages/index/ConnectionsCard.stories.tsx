import type { Meta, StoryObj } from '@storybook/react-vite';

import { Status } from '@/models/status';
import ConnectionsCard from './ConnectionsCard';
import './IndexPage.css';

const samples = Array.from({ length: 72 }, (_, i) => Math.round(1800 + (4344 * i) / 71));

const meta = {
  title: 'Pages/Overview/ConnectionsCard',
  component: ConnectionsCard,
  parameters: { layout: 'padded' },
  decorators: [
    (Story) => (
      <div className="index-page ov-page" style={{ width: 900, maxWidth: '100%' }}>
        <Story />
      </div>
    ),
  ],
  args: {
    status: new Status({ tcpCount: 274, udpCount: 18, conntrackCount: 6144, conntrackMax: 7680 }),
    tcp: samples.map((_, i) => Math.round(70 + (204 * i) / 71)),
    udp: samples.map(() => 18),
    conntrack: samples,
    labels: samples.map((_, i) => `21:${String(i).padStart(2, '0')}`),
    isMobile: false,
  },
} satisfies Meta<typeof ConnectionsCard>;

export default meta;
type Story = StoryObj<typeof meta>;

export const ConntrackLimit: Story = {};

export const AboveLimit: Story = {
  args: {
    status: new Status({ tcpCount: 4000, udpCount: 18, conntrackCount: 8000, conntrackMax: 7680 }),
    conntrack: samples.map((v) => v + 1856),
  },
};

export const Unavailable: Story = {
  args: { status: new Status({ tcpCount: 274, udpCount: 18 }) },
};
